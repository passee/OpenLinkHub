package hs80rgbW

import (
	"OpenLinkHub/src/common"
	"OpenLinkHub/src/logger"
	"OpenLinkHub/src/rgb"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	MicLedLighting = "lighting" // the mic LED follows the lighting effect
	MicLedColor    = "color"    // a fixed colour while the mic is live
	MicLedOff      = "off"
)

// Positions of the mic LED's red, green and blue bytes in a colour frame.
const (
	micLedRed   = 2
	micLedGreen = 5
	micLedBlue  = 8
)

// MicLed sets what the mic LED shows. It is stored apart from the user profiles, like the
// global mute state, so switching profiles keeps it.
type MicLed struct {
	Active      string    `json:"active"`
	ActiveColor rgb.Color `json:"activeColor"`
	MutedColor  rgb.Color `json:"mutedColor"`
}

var (
	micLedMutex sync.Mutex
	micLeds     = map[string]*MicLed{} // serial -> options, read on every colour frame
)

func (d *Device) micLedPath() string {
	return filepath.Join(pwd, "database", "micled", d.Serial+".json")
}

func (d *Device) micLed() MicLed {
	micLedMutex.Lock()
	defer micLedMutex.Unlock()
	if m, ok := micLeds[d.Serial]; ok {
		return *m
	}
	m := &MicLed{
		Active:      MicLedLighting,
		ActiveColor: rgb.Color{Red: 255, Green: 255, Blue: 255, Brightness: 1},
		MutedColor:  rgb.Color{Red: 255, Brightness: 1},
	}
	if d.DeviceProfile != nil && d.DeviceProfile.DisableMicIndicator == 1 {
		m.Active = MicLedOff
	}
	if data, err := os.ReadFile(d.micLedPath()); err == nil {
		if err = json.Unmarshal(data, m); err != nil {
			logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Error("Unable to decode mic LED settings")
		}
	}
	micLeds[d.Serial] = m
	return *m
}

// MicLedSettings is read by the device page.
func (d *Device) MicLedSettings() map[string]string {
	m := d.micLed()
	return map[string]string{
		"active":      m.Active,
		"activeColor": colorHex(m.ActiveColor),
		"mutedColor":  colorHex(m.MutedColor),
	}
}

func colorHex(c rgb.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", int(c.Red), int(c.Green), int(c.Blue))
}

// SetMicLed stores the mic LED settings and shows them right away.
func (d *Device) SetMicLed(m MicLed) uint8 {
	if m.Active != MicLedLighting && m.Active != MicLedColor && m.Active != MicLedOff {
		return 0
	}
	for _, c := range []rgb.Color{m.ActiveColor, m.MutedColor} {
		if c.Red < 0 || c.Red > 255 || c.Green < 0 || c.Green > 255 || c.Blue < 0 || c.Blue > 255 {
			return 0
		}
	}
	if err := os.MkdirAll(filepath.Dir(d.micLedPath()), 0755); err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Unable to create mic LED directory")
		return 0
	}
	if err := common.SaveJsonData(d.micLedPath(), m); err != nil {
		logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Error("Unable to save mic LED settings")
		return 0
	}
	micLedMutex.Lock()
	micLeds[d.Serial] = &m
	micLedMutex.Unlock()

	if d.activeRgb != nil {
		d.activeRgb.Exit <- true
		d.activeRgb = nil
	}
	d.setDeviceColor()
	return 1
}

// applyMicLed overrides the mic LED bytes of a colour frame. While the boom is up the firmware
// darkens the LED past the mute point no matter what is written; a mute from a key or from
// Windows keeps the boom down, so the muted colour stays visible.
func (d *Device) applyMicLed(data []byte) {
	if len(data) <= micLedBlue {
		return
	}
	m := d.micLed()
	set := func(c rgb.Color) {
		data[micLedRed], data[micLedGreen], data[micLedBlue] = byte(c.Red), byte(c.Green), byte(c.Blue)
	}
	switch {
	case d.micMuted():
		set(m.MutedColor)
	case m.Active == MicLedColor:
		set(m.ActiveColor)
	case m.Active == MicLedOff:
		set(rgb.Color{})
	}
}
