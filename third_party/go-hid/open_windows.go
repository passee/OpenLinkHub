package hid

// Windows exposes every top-level collection of an interface as its own device path, and
// hid_open returns whichever comes first. Linux hidraw has one node per interface, so callers
// that rely on hid_open would get an input-only collection here. Prefer the vendor-defined
// command collection (usage 0x01 on a vendor usage page) when one matches.
func openMatching(vid, pid uint16, serial string) (*Device, error) {
	var first, preferred string
	_ = Enumerate(vid, pid, func(info *DeviceInfo) error {
		if serial != "" && info.SerialNbr != serial {
			return nil
		}
		if first == "" {
			first = info.Path
		}
		if preferred == "" && info.UsagePage >= 0xFF00 && info.Usage == 0x01 {
			preferred = info.Path
		}
		return nil
	})
	switch {
	case preferred != "":
		return OpenPath(preferred)
	case first != "":
		return OpenPath(first)
	}
	return openFirstMatch(vid, pid, serial)
}
