package hs80rgbW

import "OpenLinkHub/src/logger"

// Brightness property, 0-1000. iCUE can leave it at 0, which hides every colour written in
// software mode; the brightness slider scales the colour data instead.
var cmdSetHardwareBrightness = []byte{0x01, 0x02, 0x00, 0xe8, 0x03}

func (d *Device) setHardwareBrightness() {
	if _, err := d.transfer(cmdSetHardwareBrightness, nil); err != nil {
		logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Error("Unable to set hardware brightness")
	}
}
