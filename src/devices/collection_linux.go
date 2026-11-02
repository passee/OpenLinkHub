package devices

import "github.com/sstallion/go-hid"

func isCommandCollection(info *hid.DeviceInfo) bool { return true }
