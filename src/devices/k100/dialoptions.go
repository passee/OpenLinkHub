package k100

import (
	"OpenLinkHub/src/common"
	"OpenLinkHub/src/logger"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Offsets of the control dial LEDs inside a colour frame.
const (
	dialLedStart = 546
	dialLedEnd   = 567
)

// dialOptions are stored apart from the keyboard profiles, so they apply to every profile.
type dialOptions struct {
	Brightness int  `json:"brightness"` // percent
	SyncColor  bool `json:"syncColor"`
}

var dialOptionsMutex sync.Mutex

func (d *Device) dialOptionsPath() string {
	return filepath.Join(pwd, "database", "dial", d.Serial+".json")
}

func (d *Device) loadDialOptions() dialOptions {
	options := dialOptions{Brightness: 100}
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

// DialBrightness is read by the device page.
func (d *Device) DialBrightness() int { return d.dialSettings().Brightness }

// DialSyncColor is read by the device page.
func (d *Device) DialSyncColor() bool { return d.dialSettings().SyncColor }

// SetDialBrightness sets the control dial LED brightness in percent.
func (d *Device) SetDialBrightness(percent int) uint8 {
	if percent < 0 || percent > 100 {
		return 0
	}
	options := d.dialSettings()
	options.Brightness = percent
	return d.saveDialOptions(options)
}

// SetDialSyncColor makes the dial follow the keyboard lighting instead of the dial mode colour.
func (d *Device) SetDialSyncColor(enabled bool) uint8 {
	options := d.dialSettings()
	options.SyncColor = enabled
	return d.saveDialOptions(options)
}

// dialColor returns the dial LED bytes for a colour frame: the dial mode colour, or the
// average colour of the keys when synced, scaled by the dial brightness.
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

	if options.Brightness < 100 {
		for i := range dial {
			dial[i] = byte(int(dial[i]) * options.Brightness / 100)
		}
	}
	return dial
}
