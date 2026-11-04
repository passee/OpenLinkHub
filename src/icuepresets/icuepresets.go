// Package icuepresets reads the equalizer presets iCUE stores, so a headset keeps the sound
// it had under iCUE.
package icuepresets

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Preset holds the ten band gains in dB, 32 Hz to 16 kHz.
type Preset struct {
	Name  string      `json:"name"`
	Bands [10]float64 `json:"bands"`
}

type presetLibrary struct {
	Presets struct {
		Items []struct {
			Name   string `xml:"name"`
			Stages struct {
				Items []struct {
					XMLName xml.Name
					Value   string `xml:",chardata"`
				} `xml:",any"`
			} `xml:"stages"`
		} `xml:",any"`
	} `xml:"value0>presets"`
}

func presetDir() string {
	return filepath.Join(os.Getenv("APPDATA"), "Corsair", "CUE5", "eq_presets")
}

// Load returns every ten-band preset found in the iCUE preset libraries. iCUE keeps one library
// per headset generation; a name that appears with different values is labelled with its library.
func Load() []Preset {
	files, _ := filepath.Glob(filepath.Join(presetDir(), "*.cuelibrary"))
	sort.Strings(files)
	type found struct {
		preset  Preset
		library string
	}
	var all []found
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var library presetLibrary
		if xml.Unmarshal(data, &library) != nil {
			continue
		}
		libraryName := strings.TrimSuffix(filepath.Base(file), "equalizerpresets.cuelibrary")
		for _, item := range library.Presets.Items {
			name := strings.TrimSpace(item.Name)
			if name == "" || len(item.Stages.Items) != 10 {
				continue
			}
			preset := Preset{Name: name}
			valid := true
			for i, stage := range item.Stages.Items {
				value, err := strconv.ParseFloat(strings.TrimSpace(stage.Value), 64)
				if err != nil || value < -12 || value > 12 {
					valid = false
					break
				}
				preset.Bands[i] = float64(int(value*10+sign(value)*0.5)) / 10
			}
			if valid {
				all = append(all, found{preset: preset, library: libraryName})
			}
		}
	}

	variants := map[string]map[[10]float64]bool{}
	for _, f := range all {
		if variants[f.preset.Name] == nil {
			variants[f.preset.Name] = map[[10]float64]bool{}
		}
		variants[f.preset.Name][f.preset.Bands] = true
	}

	seen := map[string]bool{}
	presets := make([]Preset, 0, len(all))
	for _, f := range all {
		p := f.preset
		if len(variants[p.Name]) > 1 {
			p.Name += " (" + f.library + ")"
		}
		if !seen[p.Name] {
			seen[p.Name] = true
			presets = append(presets, p)
		}
	}
	sort.Slice(presets, func(i, j int) bool { return presets[i].Name < presets[j].Name })
	return presets
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

// Find returns the preset with the given name.
func Find(name string) (Preset, bool) {
	for _, p := range Load() {
		if p.Name == name {
			return p, true
		}
	}
	return Preset{}, false
}
