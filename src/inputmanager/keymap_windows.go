package inputmanager

type windowsKey struct {
	scan     uint16
	vk       uint16
	extended bool
}

// evdev codes 1-88 are the PC/AT set 1 scancodes themselves.
const lastDirectScancode = 88

// Keys sent as scancodes reach games that read raw input; media and browser keys only exist
// as virtual keys.
var evdevKeys = map[uint16]windowsKey{
	96:  {scan: 0x1C, extended: true}, // KP Enter
	97:  {scan: 0x1D, extended: true}, // Right Ctrl
	98:  {scan: 0x35, extended: true}, // KP /
	99:  {scan: 0x37, extended: true}, // Print Screen
	100: {scan: 0x38, extended: true}, // Right Alt
	102: {scan: 0x47, extended: true}, // Home
	103: {scan: 0x48, extended: true}, // Up
	104: {scan: 0x49, extended: true}, // Page Up
	105: {scan: 0x4B, extended: true}, // Left
	106: {scan: 0x4D, extended: true}, // Right
	107: {scan: 0x4F, extended: true}, // End
	108: {scan: 0x50, extended: true}, // Down
	109: {scan: 0x51, extended: true}, // Page Down
	110: {scan: 0x52, extended: true}, // Insert
	111: {scan: 0x53, extended: true}, // Delete
	113: {vk: 0xAD},                   // Mute
	114: {vk: 0xAE},                   // Volume Down
	115: {vk: 0xAF},                   // Volume Up
	117: {scan: 0x59},                 // KP =
	119: {vk: 0x13},                   // Pause
	121: {scan: 0x53},                 // KP , (mapped to KP .)
	125: {scan: 0x5B, extended: true}, // Left Win
	126: {scan: 0x5C, extended: true}, // Right Win
	127: {scan: 0x5D, extended: true}, // Menu
	140: {vk: 0xB7},                   // Calculator (launch app 2)
	150: {vk: 0xAC},                   // WWW (browser home)
	155: {vk: 0xB4},                   // Mail
	158: {vk: 0xA6},                   // Back
	159: {vk: 0xA7},                   // Forward
	163: {vk: 0xB0},                   // Next track
	164: {vk: 0xB3},                   // Play/Pause
	165: {vk: 0xB1},                   // Previous track
	166: {vk: 0xB2},                   // Stop
	172: {vk: 0xAC},                   // Home page
	173: {vk: 0xA8},                   // Refresh
	183: {scan: 0x64},                 // F13
	184: {scan: 0x65},                 // F14
	185: {scan: 0x66},                 // F15
	186: {scan: 0x67},                 // F16
	187: {scan: 0x68},                 // F17
	188: {scan: 0x69},                 // F18
	189: {scan: 0x6A},                 // F19
	190: {scan: 0x6B},                 // F20
	191: {scan: 0x6C},                 // F21
	192: {scan: 0x6D},                 // F22
	193: {scan: 0x6E},                 // F23
	194: {scan: 0x76},                 // F24
	200: {vk: 0xFA},                   // Play
	207: {vk: 0xFA},                   // Play
	217: {vk: 0xAA},                   // Search
}

func evdevToWindowsKey(code uint16) (windowsKey, bool) {
	if code >= 1 && code <= lastDirectScancode {
		return windowsKey{scan: code}, true
	}
	key, ok := evdevKeys[code]
	return key, ok
}
