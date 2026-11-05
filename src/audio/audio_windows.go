package audio

// On Windows the headset EQ is applied by Equalizer APO, which runs inside the Windows audio
// engine and reloads its configuration files whenever they change. This package writes the
// ten bands into an include file of Equalizer APO's configuration; there is no PipeWire sink.

import (
	"OpenLinkHub/src/common"
	"OpenLinkHub/src/config"
	"OpenLinkHub/src/logger"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Sink struct {
	Index  int    `json:"index"`
	Serial uint32 `json:"serial"`
	Name   string `json:"name"`
	Desc   string `json:"desc"`
}

type Audio struct {
	Enabled     bool   `json:"enabled"`
	SinkSerial  int    `json:"sinkSerial"`
	SinkName    string `json:"sinkName"`
	SinkDesc    string `json:"sinkDesc"`
	Latency     string `json:"latency"`
	MaxLatency  string `json:"maxLatency"`
	Rate        uint32 `json:"rate"`
	Channels    uint32 `json:"channels"`
	PollingRate uint32 `json:"pollingRate"`
	Debug       uint32 `json:"debug"`
	Frames      uint32 `json:"frames"`
}

const (
	includeName = "openlinkhub.txt"
	// Equalizer APO applies the following lines only to devices whose name contains this.
	deviceMatch = "HS80"
	// One octave per band, matching a ten-band graphic equalizer.
	bandQ       = 1.41
	writeDelay  = 150 * time.Millisecond
	bandCount   = 10
	includeLine = "Include: " + includeName
)

var bandFrequencies = [bandCount]int{32, 64, 125, 250, 500, 1000, 2000, 4000, 8000, 16000}

var (
	audio      Audio
	mutex      sync.Mutex
	gains      [bandCount]float64
	writeTimer *time.Timer
	settings   eqSettings
)

type eqSettings struct {
	// Off by default, which matches iCUE's loudness; on lowers the level by the largest boost.
	ClippingProtection bool `json:"clippingProtection"`
	// Digital gain on the HS80 microphone in dB; the headset's own level tops out at 100 %.
	MicBoost float64 `json:"micBoost"`
}

func settingsPath() string {
	return filepath.Join(config.GetConfig().ConfigPath, "database", "equalizer.json")
}

// ClippingProtection reports whether the EQ lowers the level to keep boosted bands from clipping.
func ClippingProtection() bool {
	mutex.Lock()
	defer mutex.Unlock()
	return settings.ClippingProtection
}

// SetClippingProtection changes and stores the clipping protection and applies it immediately.
func SetClippingProtection(enabled bool) error {
	mutex.Lock()
	settings.ClippingProtection = enabled
	mutex.Unlock()
	return saveSettings()
}

func saveSettings() error {
	mutex.Lock()
	current := settings
	mutex.Unlock()
	if err := common.SaveJsonData(settingsPath(), current); err != nil {
		return err
	}
	writeConfig()
	return nil
}

func configDir() string {
	return filepath.Join(os.Getenv("ProgramFiles"), "EqualizerAPO", "config")
}

// Init enables the EQ when Equalizer APO is installed and hooks our include file into its
// main configuration.
func Init() {
	config := filepath.Join(configDir(), "config.txt")
	data, err := os.ReadFile(config)
	if err != nil {
		logger.Log(logger.Fields{}).Info("Equalizer APO not installed; headset EQ is unavailable")
		return
	}
	text := ""
	switch {
	case isStockConfig(string(data)):
		// The installer's default adds a demo bass boost and -6 dB to every device.
		text = includeLine + "\r\n"
	case !strings.Contains(string(data), includeLine):
		text = strings.TrimRight(string(data), "\r\n") + "\r\n" + includeLine + "\r\n"
	}
	if text != "" {
		if err = os.WriteFile(config, []byte(text), 0644); err != nil {
			logger.Log(logger.Fields{"error": err, "file": config}).Error("Unable to add OpenLinkHub to Equalizer APO configuration")
			return
		}
	}
	if data, err := os.ReadFile(settingsPath()); err == nil {
		_ = json.Unmarshal(data, &settings)
	}
	audio.Enabled = true
	writeConfig()
}

var stockConfigLines = map[string]bool{
	"Preamp: -6 dB":        true,
	"Include: example.txt": true,
	"GraphicEQ: 25 0; 40 0; 63 0; 100 0; 160 0; 250 0; 400 0; 630 0; 1000 0; 1600 0; 2500 0; 4000 0; 6300 0; 10000 0; 16000 0": true,
	includeLine: true,
}

// isStockConfig reports whether config.txt still holds only the installer's default lines.
func isStockConfig(text string) bool {
	found := false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !stockConfigLines[line] {
			return false
		}
		found = true
	}
	return found
}

// SetBand sets one of the ten bands (1-10) in dB and rewrites the configuration shortly after,
// so dragging a slider does not rewrite the file for every step.
func SetBand(band int, value float64) {
	if band < 1 || band > bandCount {
		return
	}
	mutex.Lock()
	defer mutex.Unlock()
	gains[band-1] = value
	if writeTimer != nil {
		writeTimer.Stop()
	}
	writeTimer = time.AfterFunc(writeDelay, writeConfig)
}

func writeConfig() {
	mutex.Lock()
	current := gains
	protect := settings.ClippingProtection
	micBoost := settings.MicBoost
	mutex.Unlock()

	// Negative preamp equal to the largest boost keeps boosted bands from clipping.
	maxGain := 0.0
	if protect {
		for _, g := range current {
			maxGain = math.Max(maxGain, g)
		}
	}

	var b strings.Builder
	b.WriteString("# Written by OpenLinkHub. Changes here are overwritten.\r\n")
	headphones := endpointGUID("Render")
	if headphones == "" {
		headphones = deviceMatch
	}
	b.WriteString("Device: " + headphones + "\r\n")
	b.WriteString(fmt.Sprintf("Preamp: %.1f dB\r\n", 0-maxGain))
	for i, g := range current {
		b.WriteString(fmt.Sprintf("Filter %d: ON PK Fc %d Hz Gain %.1f dB Q %.2f\r\n", i+1, bandFrequencies[i], g, bandQ))
	}
	if microphone := endpointGUID("Capture"); microphone != "" {
		b.WriteString("Device: " + microphone + "\r\n")
		b.WriteString(fmt.Sprintf("Preamp: %.1f dB\r\n", micBoost))
	}
	b.WriteString("Device: all\r\n")

	path := filepath.Join(configDir(), includeName)
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		logger.Log(logger.Fields{"error": err, "file": path}).Error("Unable to write Equalizer APO configuration")
	}
}

func StopAudio() {}

func StartAudio() {}

func GetSinks() []Sink { return make([]Sink, 0) }

func UpdateAudio(settings *Audio) uint8 { return 0 }

func UpdateTargetDevice(settings *Audio) uint8 { return 0 }

func GetAudio() *Audio { return &audio }
