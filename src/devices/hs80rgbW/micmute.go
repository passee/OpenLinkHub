package hs80rgbW

import (
	"OpenLinkHub/src/logger"
	"sync/atomic"
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
	return 1
}
