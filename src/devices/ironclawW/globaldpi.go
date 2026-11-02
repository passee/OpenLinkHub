package ironclawW

import (
	"OpenLinkHub/src/common"
	"OpenLinkHub/src/logger"
	"encoding/json"
	"os"
	"path/filepath"
)

// globalDpi keeps one set of DPI stages for the mouse that every user profile uses while
// enabled, so switching lighting or game profiles leaves the sensitivity untouched.
type globalDpi struct {
	Enabled  bool               `json:"enabled"`
	Profile  int                `json:"profile"`
	Profiles map[int]DPIProfile `json:"profiles"`
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
