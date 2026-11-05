package audio

import (
	"errors"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	mmDevicesKey = `SOFTWARE\Microsoft\Windows\CurrentVersion\MMDevices\Audio\`
	// PKEY_DeviceInterface_FriendlyName of the endpoint's device, e.g. "CORSAIR HS80 RGB ...".
	deviceNameProperty = "{b3f8fa53-0004-438e-9003-51a46e139bfc},6"
	deviceStateActive  = 1
	// Equalizer APO's pre-mix class; present in an endpoint's effects once it is installed there.
	equalizerAPOClass = "{EACD2258-FCAC-4FF4-B36D-419E924A6D79}"
	maxMicBoost       = 20.0
)

// endpointGUID returns the GUID of the active HS80 endpoint for "Render" or "Capture".
// Equalizer APO's Device filter matches it exactly, unlike the name, which the headphones and
// the microphone share.
func endpointGUID(flow string) string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, mmDevicesKey+flow, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return ""
	}
	defer key.Close()
	ids, _ := key.ReadSubKeyNames(-1)
	for _, id := range ids {
		endpoint, err := registry.OpenKey(key, id, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		state, _, _ := endpoint.GetIntegerValue("DeviceState")
		endpoint.Close()
		if state != deviceStateActive {
			continue
		}
		props, err := registry.OpenKey(key, id+`\Properties`, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		name, _, _ := props.GetStringValue(deviceNameProperty)
		props.Close()
		if strings.Contains(name, deviceMatch) {
			return id
		}
	}
	return ""
}

// MicBoostAvailable reports whether Equalizer APO runs on the HS80 microphone, which the boost
// needs; it is added there with Equalizer APO's Device Selector.
func MicBoostAvailable() bool {
	id := endpointGUID("Capture")
	if id == "" {
		return false
	}
	fx, err := registry.OpenKey(registry.LOCAL_MACHINE, mmDevicesKey+`Capture\`+id+`\FxProperties`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer fx.Close()
	names, _ := fx.ReadValueNames(-1)
	for _, name := range names {
		if value, _, err := fx.GetStringValue(name); err == nil && strings.EqualFold(value, equalizerAPOClass) {
			return true
		}
	}
	return false
}

// MicBoost returns the microphone gain in dB.
func MicBoost() float64 {
	mutex.Lock()
	defer mutex.Unlock()
	return settings.MicBoost
}

// SetMicBoost stores the microphone gain in dB (0-20) and applies it.
func SetMicBoost(db float64) error {
	if db < 0 || db > maxMicBoost {
		return errors.New("mic boost must be between 0 and 20 dB")
	}
	mutex.Lock()
	settings.MicBoost = db
	mutex.Unlock()
	return saveSettings()
}
