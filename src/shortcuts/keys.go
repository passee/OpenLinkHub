package shortcuts

import "strings"

// keyVKs maps KeyboardEvent.code names to Windows virtual-key codes. Left and right
// modifiers share one entry so a shortcut works with either side.
var keyVKs = map[string]uint32{
	"Control": 0x11, "Shift": 0x10, "Alt": 0x12, "Meta": 0x5B,
	"Escape": 0x1B, "Tab": 0x09, "CapsLock": 0x14, "Space": 0x20, "Enter": 0x0D, "Backspace": 0x08,
	"Insert": 0x2D, "Delete": 0x2E, "Home": 0x24, "End": 0x23, "PageUp": 0x21, "PageDown": 0x22,
	"ArrowUp": 0x26, "ArrowDown": 0x28, "ArrowLeft": 0x25, "ArrowRight": 0x27,
	"PrintScreen": 0x2C, "ScrollLock": 0x91, "Pause": 0x13, "ContextMenu": 0x5D, "NumLock": 0x90,
	"Backquote": 0xC0, "Minus": 0xBD, "Equal": 0xBB, "BracketLeft": 0xDB, "BracketRight": 0xDD,
	"Backslash": 0xDC, "Semicolon": 0xBA, "Quote": 0xDE, "Comma": 0xBC, "Period": 0xBE, "Slash": 0xBF,
	"IntlBackslash":  0xE2,
	"NumpadMultiply": 0x6A, "NumpadAdd": 0x6B, "NumpadSubtract": 0x6D, "NumpadDecimal": 0x6E, "NumpadDivide": 0x6F,
	"MediaPlayPause": 0xB3, "MediaTrackNext": 0xB0, "MediaTrackPrevious": 0xB1, "MediaStop": 0xB2,
	"AudioVolumeMute": 0xAD, "AudioVolumeDown": 0xAE, "AudioVolumeUp": 0xAF,
	"MouseLeft": 0x01, "MouseRight": 0x02, "MouseMiddle": 0x04, "Mouse4": 0x05, "Mouse5": 0x06,
}

func isMouseKey(key string) bool {
	return strings.HasPrefix(key, "Mouse")
}

// isPrimaryClick marks buttons a shortcut must not consist of alone, or ordinary clicking
// would trigger it.
func isPrimaryClick(key string) bool {
	return key == "MouseLeft" || key == "MouseRight"
}

var vkKeys = map[uint32]string{}

func init() {
	for c := 'A'; c <= 'Z'; c++ {
		keyVKs["Key"+string(c)] = uint32(c)
	}
	for d := '0'; d <= '9'; d++ {
		keyVKs["Digit"+string(d)] = uint32(d)
		keyVKs["Numpad"+string(d)] = 0x60 + uint32(d-'0')
	}
	for f := 1; f <= 24; f++ {
		keyVKs["F"+itoa(f)] = 0x6F + uint32(f)
	}
	for name, vk := range keyVKs {
		vkKeys[vk] = name
	}
	// Low-level hooks report the sided modifier codes.
	vkKeys[0xA0], vkKeys[0xA1] = "Shift", "Shift"
	vkKeys[0xA2], vkKeys[0xA3] = "Control", "Control"
	vkKeys[0xA4], vkKeys[0xA5] = "Alt", "Alt"
	vkKeys[0x5C] = "Meta"
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

func normalizeKey(code string) string {
	for _, side := range []string{"Left", "Right"} {
		for _, mod := range []string{"Control", "Shift", "Alt", "Meta"} {
			if code == mod+side {
				return mod
			}
		}
	}
	if code == "OSLeft" || code == "OSRight" {
		return "Meta"
	}
	return code
}

func isKnownKey(code string) bool {
	_, ok := keyVKs[normalizeKey(strings.TrimSpace(code))]
	return ok
}

func vkToKey(vk uint32) (string, bool) {
	key, ok := vkKeys[vk]
	return key, ok
}
