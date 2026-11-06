package hs80rgbW

import (
	"OpenLinkHub/src/common"
	"OpenLinkHub/src/logger"
	"OpenLinkHub/src/sounds"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

const (
	// Prompts play on the output whose name contains this, the HS80 headphones.
	promptDevice         = "HS80"
	batteryLowLevel      = 15
	batteryCriticalLevel = 5
)

type voicePrompts struct {
	Enabled bool `json:"enabled"`
	// Off lists prompts switched off individually, so prompts added later start switched on.
	Off []string `json:"off"`
}

// VoicePromptOption is one prompt in the device page's selection dialog.
type VoicePromptOption struct {
	Id     string
	Label  string
	Active bool
}

// promptCatalog lists the prompts OpenLinkHub triggers, by iCUE sound file name.
var promptCatalog = []struct{ id, label string }{
	{"MicOff", "Mic muted"},
	{"MicOn", "Mic unmuted"},
	{"BatteryLow", "Battery low (15 %)"},
	{"BatteryCritical", "Battery critical (5 %)"},
	{"LinkEstablished", "Headset connected"},
}

// startupQuiet keeps the connected prompt from playing for headsets found at startup.
const startupQuiet = 20 * time.Second

var (
	promptsMutex  sync.Mutex
	promptsLoaded *voicePrompts
	lastBattery   uint16
	started       = time.Now()
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

// VoicePromptOptions is read by the device page.
func (d *Device) VoicePromptOptions() []VoicePromptOption {
	settings := d.loadVoicePrompts()
	options := make([]VoicePromptOption, 0, len(promptCatalog))
	for _, p := range promptCatalog {
		options = append(options, VoicePromptOption{Id: p.id, Label: p.label, Active: !slices.Contains(settings.Off, p.id)})
	}
	return options
}

// SetVoicePrompts turns all prompts on or off.
func (d *Device) SetVoicePrompts(enabled bool) uint8 {
	d.loadVoicePrompts()
	promptsMutex.Lock()
	promptsLoaded.Enabled = enabled
	promptsMutex.Unlock()
	return d.saveVoicePrompts()
}

// SetVoicePromptSelection chooses the individual prompts; keys are prompt ids.
func (d *Device) SetVoicePromptSelection(active map[string]bool) uint8 {
	off := make([]string, 0)
	for _, p := range promptCatalog {
		if on, ok := active[p.id]; ok && !on {
			off = append(off, p.id)
		}
	}
	d.loadVoicePrompts()
	promptsMutex.Lock()
	promptsLoaded.Off = off
	promptsMutex.Unlock()
	return d.saveVoicePrompts()
}

func (d *Device) saveVoicePrompts() uint8 {
	promptsMutex.Lock()
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
	settings := d.loadVoicePrompts()
	if settings.Enabled && !slices.Contains(settings.Off, name) {
		sounds.Play(name, promptDevice)
	}
}

func (d *Device) promptConnected() {
	if time.Since(started) > startupQuiet {
		d.prompt("LinkEstablished")
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
