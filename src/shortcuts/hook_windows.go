package shortcuts

import (
	"OpenLinkHub/src/logger"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	whKeyboardLL  = 13
	wmKeyDown     = 0x0100
	wmKeyUp       = 0x0101
	wmSysKeyDown  = 0x0104
	wmSysKeyUp    = 0x0105
	llkhfInjected = 0x10
)

type kbdllHookStruct struct {
	vkCode    uint32
	scanCode  uint32
	flags     uint32
	time      uint32
	extraInfo uintptr
}

type message struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      [2]int32
	private uint32
}

var (
	user32              = windows.NewLazySystemDLL("user32.dll")
	procSetWindowsHook  = user32.NewProc("SetWindowsHookExW")
	procCallNextHook    = user32.NewProc("CallNextHookEx")
	procGetMessage      = user32.NewProc("GetMessageW")
	procGetModuleHandle = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetModuleHandleW")

	listenerOnce sync.Once
	// Only touched on the hook thread.
	pressed = map[string]bool{}
	blocked = map[string]bool{}
	fired   bool
)

func startListener() {
	listenerOnce.Do(func() {
		go hookLoop()
	})
}

// hookLoop installs a low-level keyboard hook. The hook callback runs on this thread, which
// therefore has to be locked and pump messages for as long as the hook is wanted.
func hookLoop() {
	runtime.LockOSThread()

	module, _, _ := procGetModuleHandle.Call(0)
	hook, _, err := procSetWindowsHook.Call(whKeyboardLL, windows.NewCallback(hookProc), module, 0)
	if hook == 0 {
		logger.Log(logger.Fields{"error": err}).Error("Unable to install keyboard hook; shortcuts are disabled")
		return
	}
	logger.Log(logger.Fields{}).Info("Keyboard shortcuts active")

	var msg message
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
	}
}

func hookProc(code int, wParam uintptr, lParam uintptr) uintptr {
	if code >= 0 {
		event := *(**kbdllHookStruct)(unsafe.Pointer(&lParam))
		// Keys injected by our own macros must not trigger or be swallowed by shortcuts.
		if event.flags&llkhfInjected == 0 && handleKey(event.vkCode, wParam) {
			return 1
		}
	}
	r, _, _ := procCallNextHook.Call(0, uintptr(code), wParam, lParam)
	return r
}

// handleKey updates the pressed set and reports whether the event should be swallowed.
func handleKey(vk uint32, wParam uintptr) bool {
	key, ok := vkToKey(vk)
	if !ok {
		return false
	}

	switch wParam {
	case wmKeyDown, wmSysKeyDown:
		if pressed[key] {
			// Autorepeat of a key that completed a shortcut stays swallowed.
			return blocked[key]
		}
		pressed[key] = true
		if fired {
			return false
		}
		if s, ok := match(pressed); ok {
			fired = true
			go run(s)
			if s.Block {
				blocked[key] = true
				return true
			}
		}
	case wmKeyUp, wmSysKeyUp:
		delete(pressed, key)
		if len(pressed) == 0 {
			fired = false
		}
		if blocked[key] {
			delete(blocked, key)
			return true
		}
	}
	return false
}
