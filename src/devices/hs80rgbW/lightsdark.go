package hs80rgbW

// lightsDark reports whether the lighting shows nothing. A dark device gets one blank frame
// instead of an animation loop, which would keep the radio busy and drain the battery.
func (d *Device) lightsDark() bool {
	return d.DeviceProfile.RGBProfile == "off" ||
		(d.DeviceProfile.BrightnessSlider != nil && *d.DeviceProfile.BrightnessSlider == 0)
}
