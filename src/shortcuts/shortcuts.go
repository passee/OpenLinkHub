// Package shortcuts runs actions on global key combinations, including chords that hold
// several ordinary keys at once (for example Ctrl+A+B).
package shortcuts

import (
	"OpenLinkHub/src/appmonitor"
	"OpenLinkHub/src/common"
	"OpenLinkHub/src/config"
	"OpenLinkHub/src/devices"
	"OpenLinkHub/src/inputmanager"
	"OpenLinkHub/src/logger"
	"OpenLinkHub/src/macro"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"time"
)

const (
	ActionMicMute = "micMute"
	ActionLock    = "lockProfile"
	ActionMacro   = "macro"
)

// Shortcut binds a key combination to an action. Keys are KeyboardEvent.code names
// ("ControlLeft", "KeyA"); left and right modifiers are treated as the same key.
type Shortcut struct {
	Name    string   `json:"name"`
	Keys    []string `json:"keys"`
	Action  string   `json:"action"`
	MacroId int      `json:"macroId,omitempty"`
	// Block keeps the final key of the combination from reaching the focused application.
	Block   bool `json:"block"`
	Enabled bool `json:"enabled"`
}

type file struct {
	Shortcuts []Shortcut `json:"shortcuts"`
}

var (
	mutex     sync.RWMutex
	shortcuts []Shortcut
)

func location() string {
	return filepath.Join(config.GetConfig().ConfigPath, "database", "shortcuts.json")
}

// Init loads the saved shortcuts and starts listening for them.
func Init() {
	data, err := os.ReadFile(location())
	if err == nil {
		var f file
		if err = json.Unmarshal(data, &f); err != nil {
			logger.Log(logger.Fields{"error": err}).Error("Unable to decode shortcuts")
		}
		mutex.Lock()
		shortcuts = f.Shortcuts
		mutex.Unlock()
	}
	startListener()
}

// Get returns the configured shortcuts.
func Get() []Shortcut {
	mutex.RLock()
	defer mutex.RUnlock()
	return append([]Shortcut(nil), shortcuts...)
}

// Save validates and stores the shortcuts, replacing the previous set.
func Save(list []Shortcut) error {
	for i := range list {
		if len(list[i].Keys) == 0 {
			return errors.New("every shortcut needs at least one key")
		}
		onlyClicks := true
		for _, key := range list[i].Keys {
			if !isKnownKey(key) {
				return errors.New("unknown key: " + key)
			}
			onlyClicks = onlyClicks && isPrimaryClick(normalizeKey(key))
		}
		if onlyClicks {
			return errors.New("a left or right click needs another key in the combination")
		}
		switch list[i].Action {
		case ActionMicMute, ActionLock:
		case ActionMacro:
			if macro.GetProfile(list[i].MacroId) == nil {
				return errors.New("macro does not exist")
			}
		default:
			return errors.New("unknown action: " + list[i].Action)
		}
	}
	if err := os.MkdirAll(filepath.Dir(location()), 0755); err != nil {
		return err
	}
	if err := common.SaveJsonData(location(), file{Shortcuts: list}); err != nil {
		return err
	}
	mutex.Lock()
	shortcuts = list
	mutex.Unlock()
	listenerChanged()
	return nil
}

func hasMouseKey(s Shortcut) bool {
	for _, key := range s.Keys {
		if isMouseKey(normalizeKey(key)) {
			return true
		}
	}
	return false
}

// usesMouse reports whether an enabled shortcut contains a mouse button.
func usesMouse() bool {
	mutex.RLock()
	defer mutex.RUnlock()
	for _, s := range shortcuts {
		if s.Enabled && hasMouseKey(s) {
			return true
		}
	}
	return false
}

// match returns the enabled shortcut whose keys are exactly the pressed set.
func match(pressed map[string]bool) (Shortcut, bool) {
	mutex.RLock()
	defer mutex.RUnlock()
	keysDown, buttonsDown := 0, 0
	for key := range pressed {
		if isMouseKey(key) {
			buttonsDown++
		} else {
			keysDown++
		}
	}
	for _, s := range shortcuts {
		if !s.Enabled {
			continue
		}
		// A held mouse button, such as firing in a game, must not break keyboard-only shortcuts.
		want := keysDown
		if hasMouseKey(s) {
			want += buttonsDown
		}
		if len(s.Keys) != want {
			continue
		}
		all := true
		for _, key := range s.Keys {
			if !pressed[normalizeKey(key)] {
				all = false
				break
			}
		}
		if all {
			return s, true
		}
	}
	return Shortcut{}, false
}

func run(s Shortcut) {
	logger.Log(logger.Fields{"shortcut": s.Name, "action": s.Action}).Info("Shortcut triggered")
	switch s.Action {
	case ActionMicMute:
		toggleMicMute()
	case ActionLock:
		appmonitor.ToggleLock()
	case ActionMacro:
		playMacro(s.MacroId)
	}
}

func toggleMicMute() {
	found := false
	for serial, device := range devices.GetDevices() {
		if device == nil || device.Instance == nil {
			continue
		}
		method := reflect.ValueOf(device.Instance).MethodByName("ToggleMicMute")
		if !method.IsValid() {
			continue
		}
		found = true
		method.Call(nil)
		logger.Log(logger.Fields{"serial": serial}).Info("Microphone mute toggled")
	}
	if !found {
		logger.Log(logger.Fields{}).Warn("No headset supports microphone mute toggling")
	}
}

func playMacro(id int) {
	profile := macro.GetProfile(id)
	if profile == nil {
		logger.Log(logger.Fields{"macroId": id}).Error("Invalid macro profile")
		return
	}
	order := make([]int, 0, len(profile.Actions))
	for i := range profile.Actions {
		order = append(order, i)
	}
	sort.Ints(order)

	var held []uint16
	for _, i := range order {
		action := profile.Actions[i]
		switch action.ActionType {
		case 1, 3:
			inputmanager.InputControlKeyboard(action.ActionCommand, action.ActionHold)
			if action.ActionHold {
				held = append(held, action.ActionCommand)
			}
		case 9:
			inputmanager.InputControlMouse(action.ActionCommand)
		case 5:
			time.Sleep(time.Duration(action.ActionDelay) * time.Millisecond)
		case 6:
			inputmanager.InputControlKeyboardText(action.ActionText)
		}
	}
	for i := len(held) - 1; i >= 0; i-- {
		inputmanager.InputControlKeyboardHold(held[i], false)
	}
}
