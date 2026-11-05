// Package keyvariants keeps several saved assignments per key, of which one is active. A variant
// is a single assignment or a bulk list of steps that all run on one press.
package keyvariants

import (
	"OpenLinkHub/src/common"
	"OpenLinkHub/src/config"
	"OpenLinkHub/src/devices"
	"OpenLinkHub/src/inputmanager"
	"OpenLinkHub/src/macro"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	KindSingle = "single"
	KindBulk   = "bulk"

	actionNone     = 0
	actionKeyboard = 3
	actionMacro    = 10
)

// Step is one action of a bulk variant.
type Step struct {
	Type    string   `json:"type"` // key, combo, mouse, media, text, delay, launch
	Command uint16   `json:"command,omitempty"`
	Keys    []uint16 `json:"keys,omitempty"` // combo: modifiers first, the last key is tapped
	Text    string   `json:"text,omitempty"` // text or program path
	Delay   uint16   `json:"delay,omitempty"`
}

type Variant struct {
	Name       string                     `json:"name"`
	Kind       string                     `json:"kind"`
	Assignment inputmanager.KeyAssignment `json:"assignment"`
	Steps      []Step                     `json:"steps,omitempty"`
}

// KeyVariants holds the variants of one key; Active is an index into Variants or -1 for the
// key's default behaviour.
type KeyVariants struct {
	Active   int       `json:"active"`
	Variants []Variant `json:"variants"`
}

type store map[string]map[int]*KeyVariants // device serial -> key index -> variants

var (
	mutex sync.Mutex
	data  store
)

func location() string {
	return filepath.Join(config.GetConfig().ConfigPath, "database", "key-variants.json")
}

func load() {
	if data != nil {
		return
	}
	data = store{}
	if raw, err := os.ReadFile(location()); err == nil {
		_ = json.Unmarshal(raw, &data)
	}
}

func save() error {
	return common.SaveJsonData(location(), data)
}

// Get returns the variants of every key of a device.
func Get(serial string) map[int]*KeyVariants {
	mutex.Lock()
	defer mutex.Unlock()
	load()
	return data[serial]
}

// Set replaces the variants of one key and applies the active one.
func Set(serial string, key int, kv KeyVariants) error {
	if devices.GetDevice(serial) == nil {
		return errors.New("device not found")
	}
	if kv.Active >= len(kv.Variants) {
		return errors.New("active variant does not exist")
	}
	for i, v := range kv.Variants {
		if v.Name == "" {
			return fmt.Errorf("variant %d needs a name", i+1)
		}
		if v.Kind != KindSingle && v.Kind != KindBulk {
			return fmt.Errorf("unknown variant kind %q", v.Kind)
		}
		if v.Kind == KindBulk && len(v.Steps) == 0 {
			return fmt.Errorf("bulk variant %q has no steps", v.Name)
		}
	}

	mutex.Lock()
	load()
	if data[serial] == nil {
		data[serial] = map[int]*KeyVariants{}
	}
	data[serial][key] = &kv
	err := save()
	mutex.Unlock()
	if err != nil {
		return err
	}
	return apply(serial, key, kv)
}

// apply writes the active variant as the key's assignment.
func apply(serial string, key int, kv KeyVariants) error {
	assignment := inputmanager.KeyAssignment{Default: true, ActionType: actionNone}
	if kv.Active >= 0 {
		v := kv.Variants[kv.Active]
		switch v.Kind {
		case KindSingle:
			assignment = v.Assignment
			assignment.Default = false
		case KindBulk:
			id, err := bulkMacro(serial, key, v)
			if err != nil {
				return err
			}
			assignment = inputmanager.KeyAssignment{ActionType: actionMacro, ActionCommand: uint16(id), IsMacro: true}
		}
	}
	res := devices.CallDeviceMethod(serial, "UpdateDeviceKeyAssignment", key, assignment)
	if len(res) == 0 || res[0].Uint() != 1 {
		return errors.New("the device rejected the assignment")
	}
	return nil
}

// bulkMacro rebuilds the macro behind a bulk variant, one macro per key.
func bulkMacro(serial string, key int, v Variant) (int, error) {
	name := fmt.Sprintf("Bulk %s %d", serial[:min(6, len(serial))], key)
	for id, m := range macro.GetProfiles() {
		if m.Name == name {
			macro.DeleteMacroProfile(id)
		}
	}
	if macro.NewMacroProfile(name) != 1 {
		return 0, errors.New("unable to create the bulk macro")
	}
	id := -1
	for macroId, m := range macro.GetProfiles() {
		if m.Name == name {
			id = macroId
		}
	}
	if id < 0 {
		return 0, errors.New("bulk macro not found after creation")
	}

	index := 0
	add := func(step macro.Actions, hold bool) {
		macro.NewMacroProfileValue(id, &step)
		if hold {
			// NewMacroProfileValue drops the hold flag; only UpdateMacroValue sets it.
			macro.UpdateMacroValue(id, index, &macro.Actions{ActionHold: true})
		}
		index++
	}
	for _, s := range v.Steps {
		switch s.Type {
		case "key", "media":
			add(macro.Actions{ActionType: actionKeyboard, ActionCommand: s.Command}, false)
		case "combo":
			for i, k := range s.Keys {
				add(macro.Actions{ActionType: actionKeyboard, ActionCommand: k}, i < len(s.Keys)-1)
			}
		case "mouse":
			add(macro.Actions{ActionType: 9, ActionCommand: s.Command}, false)
		case "text":
			add(macro.Actions{ActionType: 6, ActionText: s.Text}, false)
		case "delay":
			add(macro.Actions{ActionType: 5, ActionDelay: s.Delay}, false)
		case "launch":
			add(macro.Actions{ActionType: macro.ActionLaunch, ActionText: s.Text}, false)
		default:
			return 0, fmt.Errorf("unknown step type %q", s.Type)
		}
	}
	return id, nil
}
