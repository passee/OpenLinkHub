package devices

import (
	"github.com/sstallion/go-hid"
	"strings"
)

// Windows splits one USB interface into a device path per top-level collection ("&col0N").
// Linux sees a single hidraw node, so without this filter each collection would be
// initialised as a separate device and all but one would reject every write.
func isCommandCollection(info *hid.DeviceInfo) bool {
	if !strings.Contains(strings.ToLower(info.Path), "&col") {
		return true
	}
	return info.UsagePage == 0xFF42 && info.Usage == 0x01
}
