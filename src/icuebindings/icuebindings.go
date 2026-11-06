// Package icuebindings imports keyboard and mouse key assignments from an iCUE profile.
package icuebindings

import (
	"OpenLinkHub/src/devices"
	"OpenLinkHub/src/inputmanager"
	"OpenLinkHub/src/macro"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type node struct {
	name     string
	text     string
	parent   *node
	children []*node
}

func (n *node) child(name string) *node {
	if n == nil {
		return nil
	}
	for _, c := range n.children {
		if c.name == name {
			return c
		}
	}
	return nil
}

func (n *node) path(names ...string) *node {
	for _, name := range names {
		n = n.child(name)
	}
	return n
}

func (n *node) value(names ...string) string {
	if c := n.path(names...); c != nil {
		return strings.TrimSpace(c.text)
	}
	return ""
}

func parse(r io.Reader) (*node, error) {
	decoder := xml.NewDecoder(r)
	root := &node{}
	current := root
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return root, nil
		}
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			n := &node{name: t.Name.Local, parent: current}
			current.children = append(current.children, n)
			current = n
		case xml.CharData:
			current.text += string(t)
		case xml.EndElement:
			current = current.parent
		}
	}
}

// Binding is one key assignment read from the iCUE profile.
type Binding struct {
	Device string `json:"device"`
	Key    string `json:"key"`
	Action string `json:"action"`
	Result string `json:"result"`
}

type action struct {
	kind      string // remap, keystroke, media, disable, launch
	keyName   string
	keyStroke []string
	media     string
	name      string
	// sniperHold is true for iCUE's "while pressed" sniper mode, false for on/off.
	sniperHold bool
}

func collect(n *node, polymorphic map[string]string, out *[]struct {
	device string
	key    string
	act    action
}) {
	if n.child("first") != nil && n.path("second", "layer") != nil {
		key := n.value("second", "key")
		if key != "" {
			*out = append(*out, struct {
				device string
				key    string
				act    action
			}{device: owningDevice(n), key: key, act: readAction(n.child("first"), polymorphic)})
		}
		return
	}
	for _, c := range n.children {
		collect(c, polymorphic, out)
	}
}

// owningDevice finds the nearest preceding <key> naming a device, which is how iCUE keys its
// per-device sections.
func owningDevice(n *node) string {
	for a := n.parent; a != nil && a.parent != nil; a = a.parent {
		siblings := a.parent.children
		for i := len(siblings) - 1; i >= 0; i-- {
			if siblings[i] == a {
				for j := i - 1; j >= 0; j-- {
					if siblings[j].name == "key" && strings.TrimSpace(siblings[j].text) != "" {
						return strings.TrimSpace(siblings[j].text)
					}
				}
				break
			}
		}
	}
	return ""
}

func readAction(first *node, polymorphic map[string]string) action {
	data := first.path("ptr_wrapper", "data")
	a := action{name: data.value("base", "name")}
	kind := polymorphic[first.value("polymorphic_id")]
	switch {
	case data.child("actionType") != nil && data.value("actionType") == "DisableKey":
		a.kind = "disable"
	case data.child("action") != nil:
		a.kind, a.media = "media", data.value("action")
	case data.value("applicationPath") != "":
		a.kind = "launch"
	case data.child("keyStroke") != nil && len(data.child("keyStroke").children) > 0:
		a.kind = "keystroke"
		for _, v := range data.child("keyStroke").children {
			a.keyStroke = append(a.keyStroke, strings.TrimSpace(v.text))
		}
	case data.value("keyName") != "":
		a.kind, a.keyName = "remap", data.value("keyName")
		a.sniperHold = data.value("sniperSwitchMode") == "WhilePressed"
	default:
		a.kind = "unsupported:" + kind
	}
	return a
}

func (a action) String() string {
	switch a.kind {
	case "keystroke":
		return strings.Join(a.keyStroke, "+")
	case "remap":
		return a.keyName
	case "media":
		return "media " + a.media
	}
	return a.kind
}

// Import reads the named iCUE profile and applies its keyboard and mouse bindings to the
// matching OpenLinkHub devices.
func Import(profileName string, apply bool) ([]Binding, error) {
	dir := filepath.Join(os.Getenv("APPDATA"), "Corsair", "CUE5", "profiles")
	files, _ := filepath.Glob(filepath.Join(dir, "*.cueprofiledata"))
	var root *node
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil || profileNameOf(string(data)) != profileName {
			continue
		}
		if root, err = parse(strings.NewReader(string(data))); err != nil {
			return nil, err
		}
		break
	}
	if root == nil {
		return nil, fmt.Errorf("iCUE profile %q not found", profileName)
	}

	polymorphic := map[string]string{}
	var walk func(*node)
	walk = func(n *node) {
		if id, name := n.value("polymorphic_id"), n.value("polymorphic_name"); id != "" && name != "" {
			polymorphic[id] = name
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	walk(root)

	var found []struct {
		device string
		key    string
		act    action
	}
	collect(root, polymorphic, &found)

	var results []Binding
	seen := map[string]bool{}
	for _, f := range found {
		target, index, ok := resolveKey(f.device, f.key)
		b := Binding{Device: f.device, Key: f.key, Action: f.act.String()}
		if !ok {
			continue // devices and keys OpenLinkHub does not manage
		}
		if seen[target+f.key] {
			continue
		}
		seen[target+f.key] = true
		assignment, err := convert(f.act)
		if err == nil && assignment.ActionType == typeSniper {
			assignment.DeviceId = mouseSerial()
			if assignment.DeviceId == "" {
				err = fmt.Errorf("no mouse for sniper DPI")
			}
		}
		if err != nil {
			b.Result = "skipped: " + err.Error()
		} else if !apply {
			b.Result = "importable"
		} else if res := devices.CallDeviceMethod(target, "UpdateDeviceKeyAssignment", index, assignment); len(res) == 0 || res[0].Uint() != 1 {
			b.Result = "skipped: device rejected the assignment"
		} else {
			b.Result = "imported"
		}
		results = append(results, b)
	}
	return results, nil
}

// profileNameOf returns the profile's own name, the first <name> element in the file.
func profileNameOf(text string) string {
	start := strings.Index(text, "<name>")
	if start < 0 {
		return ""
	}
	start += len("<name>")
	end := strings.Index(text[start:], "</name>")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(text[start : start+end])
}

// ironclawButtons maps iCUE button names to the Ironclaw RGB Wireless button bits.
var ironclawButtons = map[string]int{
	"Mouse1": 1, "Mouse2": 2, "Mouse3": 4, "Mouse4": 16, "Mouse5": 8,
	"Mouse6": 32, "Mouse7": 64, "MouseProfileUp": 128, "MouseProfileDown": 256, "MouseOption": 512,
}

var k100KeyAliases = map[string]string{"CapsLock": "CAPS"}

func resolveKey(device, key string) (string, int, bool) {
	for serial, d := range devices.GetDevices() {
		if d == nil {
			continue
		}
		switch {
		case strings.Contains(strings.ToUpper(device), "IRONCLAW") && strings.Contains(strings.ToUpper(d.Product), "IRONCLAW"):
			if bit, ok := ironclawButtons[key]; ok {
				return serial, bit, true
			}
		case device == "Keyboard" && strings.Contains(strings.ToUpper(d.Product), "K100"):
			name := key
			if alias, ok := k100KeyAliases[key]; ok {
				name = alias
			}
			res := devices.CallDeviceMethod(serial, "KeyIndexByName", name)
			if len(res) > 0 && res[0].Int() >= 0 {
				return serial, int(res[0].Int()), true
			}
		}
	}
	return "", 0, false
}

// iCueKeys maps iCUE key names to OpenLinkHub keyboard action ids.
var iCueKeys = func() map[string]uint16 {
	keys := map[string]uint16{
		"Minus": 18, "Equal": 19, "Backspace": 58, "Tab": 59, "Escape": 60, "GraveAccentAndTilde": 61,
		"BracketLeft": 62, "BracketRight": 63, "Backslash": 64, "CapsLock": 65, "Semicolon": 66,
		"ApostropheAndDoubleQuote": 67, "Enter": 68, "LeftShift": 69, "Comma": 70, "Period": 71, "Slash": 72,
		"RightShift": 73, "UpArrow": 74, "LeftCtrl": 75, "LeftGui": 76, "LeftAlt": 77, "Space": 78,
		"RightAlt": 79, "Application": 80, "RightCtrl": 81, "LeftArrow": 82, "DownArrow": 83, "RightArrow": 84,
		"Insert": 85, "Home": 86, "PageUp": 87, "Delete": 88, "End": 89, "PageDown": 90,
		"Mouse1": 91, "Mouse2": 92, "Mouse3": 93, "Mouse4": 94, "Mouse5": 95,
		"KeypadPeriodAndDelete": 106, "KeypadPlus": 107, "KeypadMinus": 108, "KeypadAsterisk": 109,
		"KeypadSlash": 110, "KeypadEnter": 111, "PrintScreen": 126,
		// Alternative spellings seen in older iCUE versions.
		"Esc": 60, "Up": 74, "Down": 83, "Left": 82, "Right": 84, "Menu": 80, "RightGui": 76,
	}
	for i, letter := range "QWERTYUIOP" {
		keys[string(letter)] = uint16(20 + i)
	}
	for i, letter := range "ASDFGHJKL" {
		keys[string(letter)] = uint16(30 + i)
	}
	for i, letter := range "ZXCVBNM" {
		keys[string(letter)] = uint16(39 + i)
	}
	for i, digit := range "1234567890" {
		keys[string(digit)] = uint16(8 + i)
		keys["Keypad"+string(digit)] = uint16(96 + i)
	}
	for f := 1; f <= 12; f++ {
		keys[fmt.Sprintf("F%d", f)] = uint16(45 + f)
	}
	for f := 13; f <= 24; f++ {
		keys[fmt.Sprintf("F%d", f)] = uint16(99 + f)
	}
	return keys
}()

const (
	typeNone     = 0
	typeMedia    = 1
	typeKeyboard = 3
	typeSniper   = 8
	typeMouse    = 9
	typeMacro    = 10
	typeDpiCycle = 24
)

// iCUE names the sniper button of a remap "Mouse8".
const iCueSniper = "Mouse8"

func mouseSerial() string {
	for serial, d := range devices.GetDevices() {
		if d != nil && strings.Contains(strings.ToUpper(d.Product), "IRONCLAW") {
			return serial
		}
	}
	return ""
}

func convert(a action) (inputmanager.KeyAssignment, error) {
	switch a.kind {
	case "disable":
		return inputmanager.KeyAssignment{ActionType: typeNone}, nil
	case "media":
		if a.media == "PlayPause" {
			return inputmanager.KeyAssignment{ActionType: typeMedia, ActionCommand: inputmanager.MediaPlayPause}, nil
		}
		return inputmanager.KeyAssignment{}, fmt.Errorf("media action %s", a.media)
	case "remap":
		switch a.keyName {
		case "MouseDpiToggle":
			return inputmanager.KeyAssignment{ActionType: typeDpiCycle}, nil
		case iCueSniper:
			return inputmanager.KeyAssignment{ActionType: typeSniper, ActionHold: a.sniperHold}, nil
		case "Mouse1", "Mouse2", "Mouse3":
			return inputmanager.KeyAssignment{ActionType: typeMouse, ActionCommand: iCueKeys[a.keyName]}, nil
		case "Mouse4":
			return inputmanager.KeyAssignment{ActionType: typeMouse, ActionCommand: inputmanager.BtnBack}, nil
		case "Mouse5":
			return inputmanager.KeyAssignment{ActionType: typeMouse, ActionCommand: inputmanager.BtnForward}, nil
		}
		if code, ok := iCueKeys[a.keyName]; ok {
			return inputmanager.KeyAssignment{ActionType: typeKeyboard, ActionCommand: code}, nil
		}
		return inputmanager.KeyAssignment{}, fmt.Errorf("no equivalent for %s", a.keyName)
	case "keystroke":
		if len(a.keyStroke) == 1 {
			if code, ok := iCueKeys[a.keyStroke[0]]; ok {
				return inputmanager.KeyAssignment{ActionType: typeKeyboard, ActionCommand: code}, nil
			}
			return inputmanager.KeyAssignment{}, fmt.Errorf("no equivalent for %s", a.keyStroke[0])
		}
		id, err := comboMacro(a)
		if err != nil {
			return inputmanager.KeyAssignment{}, err
		}
		return inputmanager.KeyAssignment{ActionType: typeMacro, ActionCommand: uint16(id), IsMacro: true}, nil
	}
	return inputmanager.KeyAssignment{}, fmt.Errorf("%s actions are not supported", a.kind)
}

// comboMacro creates (or reuses) a macro that holds the modifiers and taps the last key.
func comboMacro(a action) (int, error) {
	name := "iCUE " + strings.Join(a.keyStroke, "+")
	for id, m := range macro.GetProfiles() {
		if m.Name == name {
			holdModifiers(id, len(a.keyStroke))
			return id, nil
		}
	}
	codes := make([]uint16, 0, len(a.keyStroke))
	for _, key := range a.keyStroke {
		code, ok := iCueKeys[key]
		if !ok {
			return 0, fmt.Errorf("no equivalent for %s", key)
		}
		codes = append(codes, code)
	}
	if macro.NewMacroProfile(name) != 1 {
		return 0, fmt.Errorf("unable to create macro %s", name)
	}
	for id, m := range macro.GetProfiles() {
		if m.Name != name {
			continue
		}
		for _, code := range codes {
			macro.NewMacroProfileValue(id, &macro.Actions{ActionType: typeKeyboard, ActionCommand: code})
		}
		holdModifiers(id, len(codes))
		return id, nil
	}
	return 0, fmt.Errorf("macro %s not found after creation", name)
}

// holdModifiers marks every step but the last as held, so the combination is pressed together.
// NewMacroProfileValue drops the hold flag; only UpdateMacroValue sets it.
func holdModifiers(id, steps int) {
	for i := 0; i < steps-1; i++ {
		macro.UpdateMacroValue(id, i, &macro.Actions{ActionHold: true})
	}
}
