package monitor

import (
	"OpenLinkHub/src/config"
	"OpenLinkHub/src/devices"
	"OpenLinkHub/src/logger"
	"OpenLinkHub/src/openrgb"
	"github.com/sstallion/go-hid"
	"os"
	"slices"
	"strconv"
	"time"
)

const (
	vendorId     = uint16(6940)  // Corsair
	scufVendorId = uint16(11925) // Scuf
	pollInterval = 2 * time.Second
	// A tick arriving this late means the machine was suspended in between.
	resumeGap = 15 * time.Second
)

var exclude = []uint16{10752, 2666, 2710, 2659}

var longSleep = []uint16{7165, 7033}

type usbKey struct {
	productId uint16
	serial    string
}

func Init() {
	go watchResume()
	go watchHotplug()
}

// Upstream exits on resume and lets the service manager restart the process, because devices
// do not reliably survive re-initialisation after suspend. The Windows service does the same.
func watchResume() {
	last := time.Now()
	for range time.Tick(time.Second) {
		now := time.Now()
		if now.Sub(last) > resumeGap {
			time.Sleep(time.Duration(config.GetConfig().ResumeDelay) * time.Millisecond)
			logger.Log(logger.Fields{}).Info("Resume detected. Process is shutting down...")
			os.Exit(1)
		}
		last = now
	}
}

// Windows has no netlink uevents, so hotplug is detected by diffing HID enumerations.
func watchHotplug() {
	known := enumerate()
	logger.Log(logger.Fields{}).Info("Starting USB monitor...")

	for range time.Tick(pollInterval) {
		current := enumerate()

		for key := range current {
			if _, ok := known[key]; ok {
				continue
			}
			time.Sleep(7000 * time.Millisecond)
			if slices.Contains(longSleep, key.productId) {
				time.Sleep(2000 * time.Millisecond)
			}
			logger.Log(logger.Fields{"productId": key.productId, "serial": key.serial}).Info("Init USB device...")
			devices.InitManual(key.productId, key.serial)
		}

		for key := range known {
			if _, ok := current[key]; ok {
				continue
			}
			serial := key.serial
			if len(serial) < 1 {
				serial = strconv.Itoa(int(key.productId))
			}
			logger.Log(logger.Fields{"productId": key.productId, "serial": serial}).Info("Dirty USB removal...")
			devices.StopDirty(serial, key.productId)
			openrgb.NotifyControllerChange(serial)
		}

		known = current
	}
}

func enumerate() map[usbKey]struct{} {
	found := make(map[usbKey]struct{})
	collect := hid.EnumFunc(func(info *hid.DeviceInfo) error {
		if !slices.Contains(exclude, info.ProductID) {
			found[usbKey{productId: info.ProductID, serial: info.SerialNbr}] = struct{}{}
		}
		return nil
	})
	for _, vid := range []uint16{vendorId, scufVendorId} {
		if err := hid.Enumerate(vid, hid.ProductIDAny, collect); err != nil {
			logger.Log(logger.Fields{"error": err, "vendorId": vid}).Warn("HID enumeration failed")
		}
	}
	return found
}
