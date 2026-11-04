package audio

import "errors"

// ClippingProtection is a setting of the Windows Equalizer APO backend.
func ClippingProtection() bool { return false }

func SetClippingProtection(enabled bool) error {
	return errors.New("not available on this platform")
}
