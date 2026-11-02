package inputmanager

// Windows has no uinput. The virtual devices are opened on NUL so the shared code sees valid
// handles, ioctls become no-ops, and every evdev event written to them is translated into
// SendInput calls. That keeps all upstream key assignment and macro logic unchanged.

import (
	"OpenLinkHub/src/logger"
	"errors"
	"os"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type uinputKind int

const (
	uinputKeyboard uinputKind = iota
	uinputMouse
	uinputMouseAbs
	uinputGamepad
)

func openUinput(kind uinputKind) (*os.File, error) {
	if kind == uinputGamepad {
		return nil, errors.New("virtual gamepads are not supported on Windows")
	}
	return os.OpenFile(os.DevNull, os.O_WRONLY, 0)
}

func sysIoctl(fd, req, arg uintptr) (uintptr, uintptr, syscall.Errno) { return 0, 0, 0 }

func sysSetNonblock(fd uintptr) (uintptr, uintptr, syscall.Errno) { return 0, 0, 0 }

const (
	inputMouse    = 0
	inputKeyboard = 1

	keyEventExtendedKey = 0x0001
	keyEventKeyUp       = 0x0002
	keyEventScanCode    = 0x0008

	mouseEventMove        = 0x0001
	mouseEventLeftDown    = 0x0002
	mouseEventLeftUp      = 0x0004
	mouseEventRightDown   = 0x0008
	mouseEventRightUp     = 0x0010
	mouseEventMiddleDown  = 0x0020
	mouseEventMiddleUp    = 0x0040
	mouseEventXDown       = 0x0080
	mouseEventXUp         = 0x0100
	mouseEventWheel       = 0x0800
	mouseEventHWheel      = 0x1000
	mouseEventVirtualDesk = 0x4000
	mouseEventAbsolute    = 0x8000

	wheelDelta = 120
)

// Layouts match the 64-bit INPUT union: 4-byte type, 4 bytes padding, then the payload.
type keyboardInput struct {
	inputType uint32
	_         uint32
	vk        uint16
	scan      uint16
	flags     uint32
	time      uint32
	extraInfo uintptr
	_         [8]byte
}

type mouseInput struct {
	inputType uint32
	_         uint32
	dx        int32
	dy        int32
	mouseData uint32
	flags     uint32
	time      uint32
	extraInfo uintptr
}

var (
	user32        = windows.NewLazySystemDLL("user32.dll")
	procSendInput = user32.NewProc("SendInput")

	pendingMutex sync.Mutex
	pendingRelX  int32
	pendingRelY  int32
	pendingAbsX  int32 = -1
	pendingAbsY  int32 = -1
)

func sendInput(ptr unsafe.Pointer, size uintptr) error {
	if n, _, err := procSendInput.Call(1, uintptr(ptr), size); n != 1 {
		return err
	}
	return nil
}

func sendKey(code uint16, down bool) error {
	key, ok := evdevToWindowsKey(code)
	if !ok {
		return nil
	}
	in := keyboardInput{inputType: inputKeyboard}
	if key.vk != 0 {
		in.vk = key.vk
	} else {
		in.scan = key.scan
		in.flags = keyEventScanCode
	}
	if key.extended {
		in.flags |= keyEventExtendedKey
	}
	if !down {
		in.flags |= keyEventKeyUp
	}
	return sendInput(unsafe.Pointer(&in), unsafe.Sizeof(in))
}

func sendMouse(dx, dy int32, data, flags uint32) error {
	in := mouseInput{inputType: inputMouse, dx: dx, dy: dy, mouseData: data, flags: flags}
	return sendInput(unsafe.Pointer(&in), unsafe.Sizeof(in))
}

func sendMouseButton(code uint16, down bool) error {
	var flags, data uint32
	switch code {
	case btnLeft:
		flags = pick(down, mouseEventLeftDown, mouseEventLeftUp)
	case btnRight:
		flags = pick(down, mouseEventRightDown, mouseEventRightUp)
	case btnMiddle:
		flags = pick(down, mouseEventMiddleDown, mouseEventMiddleUp)
	case btnBack:
		flags, data = pick(down, mouseEventXDown, mouseEventXUp), 1
	case btnForward:
		flags, data = pick(down, mouseEventXDown, mouseEventXUp), 2
	default:
		return nil
	}
	return sendMouse(0, 0, data, flags)
}

func pick(cond bool, a, b uint32) uint32 {
	if cond {
		return a
	}
	return b
}

// flush emits the motion collected since the previous EV_SYN, as evdev frames do.
func flush() error {
	if pendingRelX != 0 || pendingRelY != 0 {
		dx, dy := pendingRelX, pendingRelY
		pendingRelX, pendingRelY = 0, 0
		if err := sendMouse(dx, dy, 0, mouseEventMove); err != nil {
			return err
		}
	}
	if pendingAbsX >= 0 && pendingAbsY >= 0 && screenWidth > 1 && screenHeight > 1 {
		x := pendingAbsX * 65535 / (screenWidth - 1)
		y := pendingAbsY * 65535 / (screenHeight - 1)
		pendingAbsX, pendingAbsY = -1, -1
		return sendMouse(x, y, 0, mouseEventMove|mouseEventAbsolute|mouseEventVirtualDesk)
	}
	return nil
}

func writeVirtualEvent(f *os.File, event *inputEvent) error {
	if f == nil {
		logger.Log(logger.Fields{}).Error("No valid virtual inputs")
		return errors.New("no valid virtual inputs")
	}

	pendingMutex.Lock()
	defer pendingMutex.Unlock()

	var err error
	switch event.Type {
	case EvSyn:
		err = flush()
	case EvKey:
		if event.Value == 2 {
			return nil // autorepeat; Windows repeats held keys itself
		}
		if event.Code >= btnLeft && event.Code <= btnForward {
			err = sendMouseButton(event.Code, event.Value != 0)
		} else {
			err = sendKey(event.Code, event.Value != 0)
		}
	case EvRel:
		switch event.Code {
		case RelX:
			pendingRelX += event.Value
		case RelY:
			pendingRelY += event.Value
		case RelWheel:
			err = sendMouse(0, 0, uint32(event.Value*wheelDelta), mouseEventWheel)
		case RelHWheel:
			err = sendMouse(0, 0, uint32(event.Value*wheelDelta), mouseEventHWheel)
		}
	case EvAbs:
		if f != virtualMouseAbsFile {
			return nil
		}
		switch event.Code {
		case AbsX:
			pendingAbsX = event.Value
		case AbsY:
			pendingAbsY = event.Value
		}
	}
	if err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Failed to send virtual inputs event")
	}
	return err
}
