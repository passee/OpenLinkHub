package appmonitor

import (
	"OpenLinkHub/src/logger"
	"OpenLinkHub/src/sounds"
	"sync/atomic"
)

// locked pauses automatic profile switching: running games no longer switch profiles in or
// out. Unlocking catches up with games that started or stopped meanwhile.
var locked atomic.Bool

// Locked reports whether automatic profile switching is paused.
func Locked() bool {
	return locked.Load()
}

// SetLocked pauses or resumes automatic profile switching. A short beep on the headset
// confirms it: one for locked, two for unlocked.
func SetLocked(value bool) {
	locked.Store(value)
	logger.Log(logger.Fields{"locked": value}).Info("Automatic profile switching lock changed")
	sounds.Play("AutoShutdownBeep", "HS80")
	if !value {
		sounds.Play("AutoShutdownBeep", "HS80")
	}
}

// ToggleLock flips the lock and returns the new state.
func ToggleLock() bool {
	value := !locked.Load()
	SetLocked(value)
	return value
}
