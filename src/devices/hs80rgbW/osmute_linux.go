package hs80rgbW

// syncOsMute reports false so the PulseAudio/ALSA handling in hs80rgbW.go runs.
func syncOsMute(muted bool) bool { return false }
