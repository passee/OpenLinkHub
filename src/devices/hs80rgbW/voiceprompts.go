package hs80rgbW

import (
	"OpenLinkHub/src/common"
	"OpenLinkHub/src/logger"
	"OpenLinkHub/src/sounds"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const (
	// Prompts play on the output whose name contains this, the HS80 headphones.
	promptDevice         = "HS80"
	batteryLowLevel      = 15
	batteryCriticalLevel = 5
)

type voicePrompts struct {
	Enabled bool `json:"enabled"`
}

var (
	promptsMutex  sync.Mutex
	promptsLoaded *voicePrompts
	lastBattery   uint16
)

func (d *Device) voicePromptsPath() string {
	return filepath.Join(pwd, "database", "voiceprompts", d.Serial+".json")
}

func (d *Device) loadVoicePrompts() voicePrompts {
	promptsMutex.Lock()
	defer promptsMutex.Unlock()
	if promptsLoaded == nil {
		promptsLoaded = &voicePrompts{Enabled: true}
		if data, err := os.ReadFile(d.voicePromptsPath()); err == nil {
			_ = json.Unmarshal(data, promptsLoaded)
		}
	}
	return *promptsLoaded
}

// VoicePromptsEnabled is read by the device page.
func (d *Device) VoicePromptsEnabled() bool {
	return d.loadVoicePrompts().Enabled
}

// SetVoicePrompts turns the spoken prompts on or off.
func (d *Device) SetVoicePrompts(enabled bool) uint8 {
	d.loadVoicePrompts()
	promptsMutex.Lock()
	promptsLoaded.Enabled = enabled
	current := *promptsLoaded
	promptsMutex.Unlock()
	if err := os.MkdirAll(filepath.Dir(d.voicePromptsPath()), 0755); err != nil {
		return 0
	}
	if err := common.SaveJsonData(d.voicePromptsPath(), current); err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Unable to save voice prompt settings")
		return 0
	}
	return 1
}

func (d *Device) prompt(name string) {
	if d.loadVoicePrompts().Enabled {
		sounds.Play(name, promptDevice)
	}
}

func (d *Device) promptMute(muted bool) {
	if muted {
		d.prompt("MicOff")
	} else {
		d.prompt("MicOn")
	}
}

// promptBattery announces a low battery once when the level drops below a threshold.
func (d *Device) promptBattery(level uint16) {
	promptsMutex.Lock()
	previous := lastBattery
	lastBattery = level
	promptsMutex.Unlock()
	switch {
	case previous == 0 || level == 0:
	case level <= batteryCriticalLevel && previous > batteryCriticalLevel:
		d.prompt("BatteryCritical")
	case level <= batteryLowLevel && previous > batteryLowLevel:
		d.prompt("BatteryLow")
	}
}
