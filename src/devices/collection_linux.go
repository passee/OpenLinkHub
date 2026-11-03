package devices

import (
	"OpenLinkHub/src/logger"
	"github.com/sstallion/go-hid"
)

func isCommandCollection(info *hid.DeviceInfo) bool { return true }

func exitHid() {
	if err := hid.Exit(); err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Unable to exit HID interface")
	}
}
