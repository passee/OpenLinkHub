package hs80rgbW

import (
	"OpenLinkHub/src/logger"
	"sync/atomic"
	"time"
)

// softMuted is a mute requested by a shortcut instead of the boom flip. It is one state for
// the headset, independent of user profiles; flipping the boom always overrides it.
var softMuted atomic.Bool

func (d *Device) micMuted() bool {
	return d.MuteStatus == 1 || softMuted.Load()
}

// ToggleMicMute mutes or unmutes the microphone as if the boom had been flipped.
func (d *Device) ToggleMicMute() uint8 {
	if d.MuteStatus == 1 {
		logger.Log(logger.Fields{"serial": d.Serial}).Info("Microphone boom is flipped up; flip it down to unmute")
		return 2
	}
	muted := !softMuted.Load()
	softMuted.Store(muted)
	syncOsMute(muted)
	if d.activeRgb != nil {
		d.activeRgb.Exit <- true
		d.activeRgb = nil
	}
	d.setDeviceColor()
	d.keepMuteLedLit()
	return 1
}

const muteRefreshInterval = 500 * time.Millisecond

// refreshingMuteLed guards a single refresher per process; there is one HS80 per dongle.
var refreshingMuteLed atomic.Bool

// keepMuteLedLit rewrites the static colour while the microphone is muted. The headset firmware
// switches the mic LED off once the boom moves past the mute point, and only reacts to a new
// colour frame; iCUE never shows this because it streams frames continuously. Animated modes
// stream frames themselves, so only static colours need it.
func (d *Device) keepMuteLedLit() {
	if !refreshingMuteLed.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer refreshingMuteLed.Store(false)
		ticker := time.NewTicker(muteRefreshInterval)
		defer ticker.Stop()
		for range ticker.C {
			if d.Exit || !d.Connected || !d.micMuted() {
				return
			}
			if d.activeRgb == nil {
				d.setDeviceColor()
			}
		}
	}()
}
