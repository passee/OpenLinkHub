package audio

// On Windows the headset EQ is applied by Equalizer APO, which runs inside the Windows audio
// engine and reloads its configuration files whenever they change. This package writes the
// ten bands into an include file of Equalizer APO's configuration; there is no PipeWire sink.

import (
	"OpenLinkHub/src/logger"
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
)

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
	if !strings.Contains(string(data), includeLine) {
		text := strings.TrimRight(string(data), "\r\n") + "\r\n" + includeLine + "\r\n"
		if err = os.WriteFile(config, []byte(text), 0644); err != nil {
			logger.Log(logger.Fields{"error": err, "file": config}).Error("Unable to add OpenLinkHub to Equalizer APO configuration")
			return
		}
	}
	audio.Enabled = true
	writeConfig()
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
	mutex.Unlock()

	// Negative preamp equal to the largest boost keeps boosted bands from clipping.
	maxGain := 0.0
	for _, g := range current {
		maxGain = math.Max(maxGain, g)
	}

	var b strings.Builder
	b.WriteString("# Written by OpenLinkHub. Changes here are overwritten.\r\n")
	b.WriteString("Device: " + deviceMatch + "\r\n")
	b.WriteString(fmt.Sprintf("Preamp: %.1f dB\r\n", -maxGain))
	for i, g := range current {
		b.WriteString(fmt.Sprintf("Filter %d: ON PK Fc %d Hz Gain %.1f dB Q %.2f\r\n", i+1, bandFrequencies[i], g, bandQ))
	}

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
