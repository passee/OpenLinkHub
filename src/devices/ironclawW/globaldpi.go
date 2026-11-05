package ironclawW

import (
	"OpenLinkHub/src/common"
	"OpenLinkHub/src/logger"
	"OpenLinkHub/src/rgb"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// globalDpi keeps one set of DPI stages for the mouse that every user profile uses while
// enabled, so switching lighting or game profiles leaves the sensitivity untouched.
type globalDpi struct {
	Enabled  bool               `json:"enabled"`
	Profile  int                `json:"profile"`
	Profiles map[int]DPIProfile `json:"profiles"`
	// SniperColor lights the DPI LED while sniper DPI is active; nil means the yellow default.
	SniperColor *rgb.Color `json:"sniperColor,omitempty"`
}

func (d *Device) globalDpiPath() string {
	return filepath.Join(pwd, "database", "dpi", d.Serial+".json")
}

func (d *Device) loadGlobalDpi() *globalDpi {
	data, err := os.ReadFile(d.globalDpiPath())
	if err != nil {
		return &globalDpi{}
	}
	g := &globalDpi{}
	if err = json.Unmarshal(data, g); err != nil {
		logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Error("Unable to decode global DPI settings")
		return &globalDpi{}
	}
	return g
}

func (d *Device) saveGlobalDpi(g *globalDpi) {
	if err := os.MkdirAll(filepath.Dir(d.globalDpiPath()), 0755); err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Unable to create global DPI directory")
		return
	}
	if err := common.SaveJsonData(d.globalDpiPath(), g); err != nil {
		logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Error("Unable to save global DPI settings")
	}
}

// GlobalDpiEnabled reports whether DPI stages are shared across all user profiles.
func (d *Device) GlobalDpiEnabled() bool {
	return d.loadGlobalDpi().Enabled
}

// SetGlobalDpi enables or disables shared DPI stages, seeding them from the active profile.
func (d *Device) SetGlobalDpi(enabled bool) uint8 {
	if d.DeviceProfile == nil {
		return 0
	}
	g := d.loadGlobalDpi()
	g.Enabled = enabled
	if enabled {
		g.Profile = d.DeviceProfile.Profile
		g.Profiles = d.DeviceProfile.Profiles
	}
	d.saveGlobalDpi(g)
	return 1
}

// applyGlobalDpi overwrites the active profile's DPI stages with the shared ones.
func (d *Device) applyGlobalDpi() {
	if d.DeviceProfile == nil {
		return
	}
	g := d.loadGlobalDpi()
	if !g.Enabled || len(g.Profiles) == 0 {
		return
	}
	d.DeviceProfile.Profile = g.Profile
	d.DeviceProfile.Profiles = g.Profiles
}

// storeGlobalDpi records the active profile's DPI stages as the shared ones.
func (d *Device) storeGlobalDpi() {
	if d.DeviceProfile == nil {
		return
	}
	g := d.loadGlobalDpi()
	if !g.Enabled {
		return
	}
	g.Profile = d.DeviceProfile.Profile
	g.Profiles = d.DeviceProfile.Profiles
	d.saveGlobalDpi(g)
}

// CycleDpi moves to the next DPI stage and wraps to the first after the last, like iCUE's
// DPI toggle; ModifyDpi stops at either end.
func (d *Device) CycleDpi() {
	if d.DeviceProfile == nil || len(d.DeviceProfile.Profiles) == 0 {
		return
	}
	// The sniper stage is reached through sniper mode, not by cycling.
	stages := 0
	for _, stage := range d.DeviceProfile.Profiles {
		if !stage.Sniper {
			stages++
		}
	}
	if stages == 0 {
		return
	}
	d.DeviceProfile.Profile = (d.DeviceProfile.Profile + 1) % stages
	d.storeGlobalDpi()
	d.saveDeviceProfile()
	d.toggleDPI()
}

// ToggleSniperMode switches sniper DPI on or off, for keys bound in on/off mode.
func (d *Device) ToggleSniperMode() {
	if d.Connected {
		d.sniperMode(!d.SniperMode)
	}
}

func (d *Device) sniperColor() *rgb.Color {
	if c := d.loadGlobalDpi().SniperColor; c != nil {
		return &rgb.Color{Red: c.Red, Green: c.Green, Blue: c.Blue, Brightness: 1}
	}
	return &rgb.Color{Red: 255, Green: 255, Blue: 0, Brightness: 1}
}

// SniperColorHex is read by the device page.
func (d *Device) SniperColorHex() string {
	c := d.sniperColor()
	return fmt.Sprintf("#%02x%02x%02x", int(c.Red), int(c.Green), int(c.Blue))
}

// SetSniperColor stores the DPI LED colour shown while sniper DPI is active.
func (d *Device) SetSniperColor(color rgb.Color) uint8 {
	if color.Red < 0 || color.Red > 255 || color.Green < 0 || color.Green > 255 || color.Blue < 0 || color.Blue > 255 {
		return 0
	}
	g := d.loadGlobalDpi()
	g.SniperColor = &rgb.Color{Red: color.Red, Green: color.Green, Blue: color.Blue, Brightness: 1}
	d.saveGlobalDpi(g)
	if d.SniperMode {
		if d.activeRgb != nil {
			d.activeRgb.Exit <- true
			d.activeRgb = nil
		}
		d.setDeviceColor()
	}
	return 1
}
