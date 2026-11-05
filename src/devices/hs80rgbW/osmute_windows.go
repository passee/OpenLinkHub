package hs80rgbW

import (
	"OpenLinkHub/src/logger"
	"OpenLinkHub/src/micmute"
	"sync/atomic"
	"time"
)

const micEndpointMatch = "HS80"

// syncOsMute mirrors the headset mute state onto its Windows microphone endpoint.
func syncOsMute(muted bool) bool {
	if err := micmute.SetMuted(micEndpointMatch, muted); err != nil {
		logger.Log(logger.Fields{"error": err, "muted": muted}).Warn("Unable to change HS80 microphone mute state")
	}
	return true
}

// MicVolume returns the HS80 microphone level, 0-100, or -1 when it cannot be read.
func (d *Device) MicVolume() int {
	volume, err := micmute.Volume(micEndpointMatch)
	if err != nil {
		return -1
	}
	return volume
}

// SetMicVolume sets the HS80 microphone level at its Windows endpoint.
func (d *Device) SetMicVolume(volume int) uint8 {
	if err := micmute.SetVolume(micEndpointMatch, volume); err != nil {
		logger.Log(logger.Fields{"error": err, "volume": volume}).Warn("Unable to change HS80 microphone volume")
		return 0
	}
	return 1
}

const osMutePollInterval = 750 * time.Millisecond

var watchingOsMute atomic.Bool

// osMicSilenced reports a microphone that Windows mutes, sets to 0 %, or has switched off
// ("Don't allow" in Settings removes the endpoint from the active list).
func osMicSilenced() bool {
	muted, err := micmute.Muted(micEndpointMatch)
	if err != nil {
		return true
	}
	if muted {
		return true
	}
	volume, err := micmute.Volume(micEndpointMatch)
	return err == nil && volume == 0
}

// watchOsMute follows mutes made outside the headset, in Windows sound settings or an app, so
// the mic LED turns red for them too, as it did with iCUE.
func (d *Device) watchOsMute() {
	if !watchingOsMute.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer watchingOsMute.Store(false)
		for range time.Tick(osMutePollInterval) {
			if d.Exit {
				return
			}
			if d.MuteStatus == 1 || !d.Connected {
				continue
			}
			muted := osMicSilenced()
			if muted == softMuted.Load() {
				continue
			}
			softMuted.Store(muted)
			d.promptMute(muted)
			if d.activeRgb != nil {
				d.activeRgb.Exit <- true
				d.activeRgb = nil
			}
			d.setDeviceColor()
		}
	}()
}
