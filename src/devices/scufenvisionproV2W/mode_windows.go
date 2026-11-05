package scufenvisionproV2W

import "OpenLinkHub/src/inputmanager"

// hardwareModeOnly keeps the controller in hardware mode when its buttons cannot be passed on.
// In software mode every button goes through OpenLinkHub, which on Windows needs the SCUF driver's
// control device; without it, or while iCUE holds it, games would see the sticks but no buttons.
func hardwareModeOnly() bool {
	return !inputmanager.GamepadInjectionAvailable()
}
