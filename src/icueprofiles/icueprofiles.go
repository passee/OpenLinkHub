// Package icueprofiles turns iCUE profiles into OpenLinkHub device profiles and application rules.
// iCUE's default profile maps to each device's "default" profile; every other profile becomes a
// device profile of the same name (letters and digits only) with its key bindings. Profiles
// linked to games switch on while the game runs; profiles without a game become manual global
// profiles. A profile without any lighting layers turns the lighting off.
package icueprofiles

import (
	"OpenLinkHub/src/applications"
	"OpenLinkHub/src/appmonitor"
	"OpenLinkHub/src/config"
	"OpenLinkHub/src/devices"
	"OpenLinkHub/src/icuebindings"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"unicode"
)

const defaultProfile = "default"

// Profile is one iCUE profile as found on disk.
type Profile struct {
	Name     string   `json:"name"`
	Local    string   `json:"local"`
	Programs []string `json:"programs"`
	Dark     bool     `json:"dark"`
}

// Step reports one action of the import.
type Step struct {
	Profile string `json:"profile"`
	Action  string `json:"action"`
	Result  string `json:"result"`
}

var (
	nameTag     = regexp.MustCompile(`<name>([^<]*)</name>`)
	programList = regexp.MustCompile(`(?s)<linkedProgramsPaths[^>]*>(.*?)</linkedProgramsPaths>`)
	programPath = regexp.MustCompile(`<value\d+>([^<]+)</value\d+>`)
	// Every lighting layer names its effect; "BasicNone" is iCUE's "no lighting".
	layerTag = regexp.MustCompile(`<effect>([^<]+)</effect>`)
)

func localName(name string) string {
	return strings.Map(func(r rune) rune {
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return r
		}
		return -1
	}, name)
}

// Read lists the iCUE profiles of the current Windows user.
func Read() ([]Profile, error) {
	dir := filepath.Join(os.Getenv("APPDATA"), "Corsair", "CUE5", "profiles")
	files, err := filepath.Glob(filepath.Join(dir, "*.cueprofiledata"))
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("no iCUE profiles in %s", dir)
	}
	var profiles []Profile
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		text := string(data)
		name := nameTag.FindStringSubmatch(text)
		if name == nil || localName(name[1]) == "" {
			continue
		}
		p := Profile{Name: name[1], Local: localName(name[1]), Dark: true}
		for _, effect := range layerTag.FindAllStringSubmatch(text, -1) {
			if effect[1] != "BasicNone" {
				p.Dark = false
			}
		}
		if list := programList.FindStringSubmatch(text); list != nil {
			for _, m := range programPath.FindAllStringSubmatch(list[1], -1) {
				p.Programs = append(p.Programs, m[1])
			}
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}

// profileDevices are the devices that keep per-profile lighting and bindings.
func profileDevices() []string {
	var serials []string
	for serial, d := range devices.GetDevices() {
		if d == nil || d.Hidden {
			continue
		}
		instance := reflect.ValueOf(d.Instance)
		if instance.MethodByName("SaveUserProfile").IsValid() &&
			instance.MethodByName("ChangeDeviceProfile").IsValid() &&
			instance.MethodByName("UpdateRgbProfile").IsValid() {
			serials = append(serials, serial)
		}
	}
	return serials
}

func profileExists(serial, name string) bool {
	_, err := os.Stat(filepath.Join(config.GetConfig().ConfigPath, "database", "profiles", serial+"-"+name+".json"))
	return err == nil
}

func ok(results []reflect.Value) bool {
	return len(results) > 0 && results[0].Uint() == 1
}

// switchAll switches every device and returns those that did, which excludes devices that are
// switched off; nothing may be applied to those, or it would land in their current profile.
func switchAll(serials []string, name string) []string {
	var switched []string
	for _, serial := range serials {
		if ok(devices.CallDeviceMethod(serial, "ChangeDeviceProfile", name)) {
			switched = append(switched, serial)
		}
	}
	return switched
}

// keepSharedPrograms drops a game from every profile but the one it is the only game of, so a
// game linked to two profiles switches to one of them.
func keepSharedPrograms(profiles []Profile) {
	owners := map[string][]int{}
	for i, p := range profiles {
		for _, program := range p.Programs {
			owners[strings.ToLower(program)] = append(owners[strings.ToLower(program)], i)
		}
	}
	for program, list := range owners {
		if len(list) < 2 {
			continue
		}
		keep := -1
		for _, i := range list {
			if len(profiles[i].Programs) == 1 {
				keep = i
			}
		}
		if keep < 0 {
			keep = list[0]
		}
		for _, i := range list {
			if i == keep {
				continue
			}
			filtered := profiles[i].Programs[:0]
			for _, other := range profiles[i].Programs {
				if strings.ToLower(other) != program {
					filtered = append(filtered, other)
				}
			}
			profiles[i].Programs = filtered
		}
	}
}

func bindingDevices() map[string]bool {
	keys := map[string]bool{}
	for serial, d := range devices.GetDevices() {
		if d != nil && reflect.ValueOf(d.Instance).MethodByName("UpdateDeviceKeyAssignment").IsValid() {
			keys[serial] = true
		}
	}
	return keys
}

// Import creates the device profiles and application rules. defaultName is the iCUE profile
// that matches the devices' "default" profile; its bindings are not touched.
func Import(defaultName string) ([]Step, error) {
	profiles, err := Read()
	if err != nil {
		return nil, err
	}
	keepSharedPrograms(profiles)
	serials := profileDevices()
	withBindings := bindingDevices()
	if len(serials) == 0 {
		return nil, fmt.Errorf("no devices with profiles found")
	}

	var steps []Step
	report := func(profile, action, result string) {
		steps = append(steps, Step{Profile: profile, Action: action, Result: result})
	}

	// Every new profile starts as a copy of the default one, so save them all from there.
	switchAll(serials, defaultProfile)
	for _, p := range profiles {
		if p.Name == defaultName {
			continue
		}
		created := 0
		for _, serial := range serials {
			if profileExists(serial, p.Local) || ok(devices.CallDeviceMethod(serial, "SaveUserProfile", p.Local)) {
				created++
			}
		}
		report(p.Name, "device profiles", fmt.Sprintf("%d of %d devices", created, len(serials)))
	}

	manualSlot := 1
	for _, p := range profiles {
		if p.Name == defaultName {
			continue
		}
		switched := switchAll(serials, p.Local)
		missing := false
		for serial := range withBindings {
			if !contains(switched, serial) && contains(serials, serial) {
				missing = true
			}
		}
		bindings, err := []icuebindings.Binding(nil), error(nil)
		if missing {
			err = fmt.Errorf("a keyboard or mouse is switched off; run the import again once it is on")
		} else {
			bindings, err = icuebindings.Import(p.Name, true)
		}
		if err != nil {
			report(p.Name, "key bindings", "failed: "+err.Error())
		} else {
			imported := 0
			for _, b := range bindings {
				if b.Result == "imported" {
					imported++
				}
			}
			report(p.Name, "key bindings", fmt.Sprintf("%d imported", imported))
		}
		if p.Dark {
			dark := 0
			for _, serial := range switched {
				if ok(devices.CallDeviceMethod(serial, "UpdateRgbProfile", -1, "off")) {
					dark++
				}
			}
			report(p.Name, "lighting", fmt.Sprintf("off on %d devices", dark))
		}

		active := map[string]string{}
		passive := map[string]string{}
		for _, serial := range serials {
			active[serial] = p.Local
			passive[serial] = defaultProfile
		}
		appId := ""
		if len(p.Programs) > 0 {
			appId = "icue-" + strings.ToLower(p.Local)
			executables := make([]string, 0, len(p.Programs))
			for _, program := range p.Programs {
				executables = append(executables, filepath.Base(filepath.FromSlash(program)))
			}
			if err := applications.Add(applications.Application{Id: appId, Name: p.Name + " (iCUE)", Executables: executables}); err != nil {
				report(p.Name, "game rule", "failed: "+err.Error())
				continue
			}
		} else {
			appId = fmt.Sprintf("openlinkhubgp%d", manualSlot)
			manualSlot++
		}
		rule := appmonitor.ApplicationProfile{ApplicationId: appId, Enabled: true, ActiveProfiles: active, PassiveProfiles: passive}
		result := appmonitor.AddApplicationProfile(rule)
		if result == 2 {
			result = appmonitor.UpdateApplicationProfile(rule)
		}
		target := "switches on with " + strings.Join(p.Programs, ", ")
		if len(p.Programs) == 0 {
			target = "manual, " + applications.GetApplicationById(appId).Name
		}
		if result == 1 {
			report(p.Name, "rule", target)
		} else {
			report(p.Name, "rule", "failed")
		}
	}

	switchAll(serials, defaultProfile)
	return steps, nil
}

func contains(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}
