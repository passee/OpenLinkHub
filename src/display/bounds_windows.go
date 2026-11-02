package display

import (
	"OpenLinkHub/src/common"

	"golang.org/x/sys/windows"
)

const (
	smCxVirtualScreen = 78
	smCyVirtualScreen = 79
)

var getSystemMetrics = windows.NewLazySystemDLL("user32.dll").NewProc("GetSystemMetrics")

// Absolute pointer moves are sent relative to the whole virtual desktop, so it is reported
// as one display spanning every monitor.
func platformScreenBounds() ([]common.Display, bool) {
	w, _, _ := getSystemMetrics.Call(smCxVirtualScreen)
	h, _, _ := getSystemMetrics.Call(smCyVirtualScreen)
	if w == 0 || h == 0 {
		return nil, false
	}
	return []common.Display{{Name: "virtual-desktop", Width: int(w), Height: int(h)}}, true
}
