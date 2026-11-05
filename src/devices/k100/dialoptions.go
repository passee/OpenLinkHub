package k100

import (
	"OpenLinkHub/src/common"
	"OpenLinkHub/src/logger"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// actionMicMute toggles a headset microphone; the assignment's DeviceId names the headset.
const actionMicMute = 40

// SniperToggleSupported tells the key assignment request that sniper keys without press and hold
// toggle sniper mode here instead of doing nothing.
func (d *Device) SniperToggleSupported() bool {
	return true
}

// Offsets of the control dial LEDs inside a colour frame.
const (
	dialLedStart = 546
	dialLedEnd   = 567
)

// dialOptions are stored apart from the keyboard profiles, so they apply to every profile.
type dialOptions struct {
	SyncColor bool `json:"syncColor"`
}

var dialOptionsMutex sync.Mutex

func (d *Device) dialOptionsPath() string {
	return filepath.Join(pwd, "database", "dial", d.Serial+".json")
}

func (d *Device) loadDialOptions() dialOptions {
	options := dialOptions{}
	if data, err := os.ReadFile(d.dialOptionsPath()); err == nil {
		_ = json.Unmarshal(data, &options)
	}
	return options
}

func (d *Device) saveDialOptions(options dialOptions) uint8 {
	if err := os.MkdirAll(filepath.Dir(d.dialOptionsPath()), 0755); err != nil {
		return 0
	}
	if err := common.SaveJsonData(d.dialOptionsPath(), options); err != nil {
		logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Error("Unable to save control dial options")
		return 0
	}
	dialOptionsMutex.Lock()
	d.cachedDialOptions = &options
	dialOptionsMutex.Unlock()
	if d.activeRgb != nil {
		d.activeRgb.Exit <- true
		d.activeRgb = nil
	}
	d.setDeviceColor()
	return 1
}

func (d *Device) dialSettings() dialOptions {
	dialOptionsMutex.Lock()
	defer dialOptionsMutex.Unlock()
	if d.cachedDialOptions == nil {
		options := d.loadDialOptions()
		d.cachedDialOptions = &options
	}
	return *d.cachedDialOptions
}

// DialSyncColor is read by the device page.
func (d *Device) DialSyncColor() bool { return d.dialSettings().SyncColor }

// SetDialSyncColor makes the dial follow the keyboard lighting instead of the dial mode colour.
func (d *Device) SetDialSyncColor(enabled bool) uint8 {
	options := d.dialSettings()
	options.SyncColor = enabled
	return d.saveDialOptions(options)
}

// dialColor returns the dial LED bytes for a colour frame: the dial mode colour, or the
// average colour of the keys when synced.
func (d *Device) dialColor(frame []byte) []byte {
	options := d.dialSettings()
	dial := d.getControlDialColor()

	if options.SyncColor {
		var sum [3]int
		lit := 0
		for i := 0; i+2 < dialLedStart && i+2 < len(frame); i += 3 {
			if frame[i] == 0 && frame[i+1] == 0 && frame[i+2] == 0 {
				continue
			}
			sum[0] += int(frame[i])
			sum[1] += int(frame[i+1])
			sum[2] += int(frame[i+2])
			lit++
		}
		if lit > 0 {
			for i := 0; i+2 < len(dial); i += 3 {
				dial[i], dial[i+1], dial[i+2] = byte(sum[0]/lit), byte(sum[1]/lit), byte(sum[2]/lit)
			}
		}
	}

	return dial
}

// KeyboardBrightness is read by the device page, in percent.
func (d *Device) KeyboardBrightness() int {
	if d.DeviceProfile == nil {
		return 100
	}
	return int(d.DeviceProfile.BrightnessLevel) / 10
}

// SetKeyboardBrightness sets the keyboard hardware brightness in percent, the same value the
// control dial changes in brightness mode.
func (d *Device) SetKeyboardBrightness(percent int) uint8 {
	if d.DeviceProfile == nil || percent < 0 || percent > 100 {
		return 0
	}
	d.DeviceProfile.BrightnessLevel = uint16(percent * 10)
	d.saveDeviceProfile()
	d.setBrightnessLevel()
	return 1
}

// KeyIndexByName returns the key index of a key in the active layout, or -1.
func (d *Device) KeyIndexByName(name string) int {
	if d.DeviceProfile == nil {
		return -1
	}
	keyboard, ok := d.DeviceProfile.Keyboards[d.DeviceProfile.Profile]
	if !ok {
		return -1
	}
	for _, row := range keyboard.Row {
		for index, key := range row.Keys {
			if key.KeyName == name {
				return index
			}
		}
	}
	return -1
}

// KeyName is one assignable key of the active layout.
type KeyName struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
}

// AssignableKeys lists the keys of the active layout that can carry an assignment.
func (d *Device) AssignableKeys() []KeyName {
	if d.DeviceProfile == nil {
		return nil
	}
	keyboard, ok := d.DeviceProfile.Keyboards[d.DeviceProfile.Profile]
	if !ok {
		return nil
	}
	var keys []KeyName
	for _, row := range keyboard.Row {
		for index, key := range row.Keys {
			if key.OnlyColor || key.KeyName == "" {
				continue
			}
			keys = append(keys, KeyName{Index: index, Name: key.KeyName})
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Index < keys[j].Index })
	return keys
}
