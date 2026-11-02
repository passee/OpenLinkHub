package hs80rgbW

import (
	"OpenLinkHub/src/logger"
	"OpenLinkHub/src/micmute"
)

const micEndpointMatch = "HS80"

// syncOsMute mirrors the headset mute state onto its Windows microphone endpoint.
func syncOsMute(muted bool) bool {
	if err := micmute.SetMuted(micEndpointMatch, muted); err != nil {
		logger.Log(logger.Fields{"error": err, "muted": muted}).Warn("Unable to change HS80 microphone mute state")
	}
	return true
}
