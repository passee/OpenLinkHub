package hs80rgbW

import "OpenLinkHub/src/audio"

// EqClippingProtection is read by the device page to render the "Avoid distortion" switch.
func (d *Device) EqClippingProtection() bool {
	return audio.ClippingProtection()
}

// EqAvailable reports whether an equalizer backend is active for the headset.
func (d *Device) EqAvailable() bool {
	return audio.GetAudio().Enabled
}
