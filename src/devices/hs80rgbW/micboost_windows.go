package hs80rgbW

import "OpenLinkHub/src/audio"

// MicBoost is read by the device page: the digital microphone gain in dB, or -1 while
// Equalizer APO does not run on the microphone.
func (d *Device) MicBoost() float64 {
	if !audio.MicBoostAvailable() {
		return -1
	}
	return audio.MicBoost()
}
