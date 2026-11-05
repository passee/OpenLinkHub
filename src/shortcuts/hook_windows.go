package shortcuts

import (
	"OpenLinkHub/src/logger"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	whKeyboardLL  = 13
	whMouseLL     = 14
	llmhfInjected = 0x01
	// wmSyncMouseHook asks the hook thread to install or remove the mouse hook.
	wmSyncMouseHook = 0x8000 + 1
	wmKeyDown       = 0x0100
	wmKeyUp         = 0x0101
	wmSysKeyDown    = 0x0104
	wmSysKeyUp      = 0x0105
	llkhfInjected   = 0x10
)

type kbdllHookStruct struct {
	vkCode    uint32
	scanCode  uint32
	flags     uint32
	time      uint32
	extraInfo uintptr
}

type msllHookStruct struct {
	pt        [2]int32
	mouseData uint32
	flags     uint32
	time      uint32
	extraInfo uintptr
}

// mouseButtons maps button messages to the virtual-key code of the button and whether it is a
// press. X buttons carry which one in the high word of mouseData.
var mouseButtons = map[uintptr]struct {
	vk   uint32
	down bool
}{
	0x0201: {0x01, true}, 0x0202: {0x01, false}, // left
	0x0204: {0x02, true}, 0x0205: {0x02, false}, // right
	0x0207: {0x04, true}, 0x0208: {0x04, false}, // middle
	0x020B: {0x05, true}, 0x020C: {0x05, false}, // X button, refined below
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
	procUnhookHook      = user32.NewProc("UnhookWindowsHookEx")
	procPostThreadMsg   = user32.NewProc("PostThreadMessageW")

	hookThread atomic.Uint32
	mouseHook  uintptr // only touched on the hook thread

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
	hookThread.Store(windows.GetCurrentThreadId())
	syncMouseHook(module)

	var msg message
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		if msg.message == wmSyncMouseHook {
			syncMouseHook(module)
		}
	}
}

// syncMouseHook installs the mouse hook only while a shortcut uses a mouse button: every mouse
// movement passes through a low-level hook, which is wasted work otherwise.
func syncMouseHook(module uintptr) {
	want := usesMouse()
	switch {
	case want && mouseHook == 0:
		hook, _, err := procSetWindowsHook.Call(whMouseLL, mouseCallback, module, 0)
		if hook == 0 {
			logger.Log(logger.Fields{"error": err}).Warn("Unable to install mouse hook; mouse buttons in shortcuts are disabled")
			return
		}
		mouseHook = hook
	case !want && mouseHook != 0:
		_, _, _ = procUnhookHook.Call(mouseHook)
		mouseHook = 0
	}
}

func listenerChanged() {
	if thread := hookThread.Load(); thread != 0 {
		_, _, _ = procPostThreadMsg.Call(uintptr(thread), wmSyncMouseHook, 0, 0)
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

// Callbacks are a limited resource, so the mouse one is created once and reused.
var mouseCallback = windows.NewCallback(mouseHookProc)

func mouseHookProc(code int, wParam uintptr, lParam uintptr) uintptr {
	if code >= 0 {
		if button, ok := mouseButtons[wParam]; ok {
			event := *(**msllHookStruct)(unsafe.Pointer(&lParam))
			vk := button.vk
			if vk == 0x05 && event.mouseData>>16 == 2 {
				vk = 0x06
			}
			message := uintptr(wmKeyUp)
			if button.down {
				message = wmKeyDown
			}
			if event.flags&llmhfInjected == 0 && handleKey(vk, message) {
				return 1
			}
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
		// Not waiting for every key to be up: a mouse button held in a game would otherwise
		// keep shortcuts from firing a second time.
		fired = false
		if blocked[key] {
			delete(blocked, key)
			return true
		}
	}
	return false
}
