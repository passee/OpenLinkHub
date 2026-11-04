package hs80rgbW

// syncOsMute reports false so the PulseAudio/ALSA handling in hs80rgbW.go runs.
func syncOsMute(muted bool) bool { return false }

// MicVolume is not available on Linux; -1 hides the control.
func (d *Device) MicVolume() int { return -1 }

func (d *Device) SetMicVolume(volume int) uint8 { return 0 }

// watchOsMute is not needed on Linux, where the PulseAudio/ALSA sync handles it.
func (d *Device) watchOsMute() {}
