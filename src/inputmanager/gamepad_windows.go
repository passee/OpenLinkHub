package inputmanager

// Windows has no uinput gamepad. SCUF's own driver (Scufdriver5, installed with iCUE) creates the
// Xbox 360 controller games see and exposes \\.\ScufOneCtlN, the interface iCUE uses: a written
// 20-byte Xbox 360 input report is OR-ed into the controller's buttons, and a 12-byte packet of six
// flags (LX, LY, RX, RY, LT, RT) makes the driver take that axis from the report instead of the
// controller. The driver ignores writes of any other length.

import (
	"errors"
	"fmt"
	"sync"

	"golang.org/x/sys/windows"
)

const (
	scufControlDevices = 8
	xusbReportSize     = 20
)

var (
	gamepadMutex  sync.Mutex
	scufControl   windows.Handle
	gamepadReport = [xusbReportSize]byte{0x00, xusbReportSize}
)

// Bit positions in bytes 2 and 3 of the Xbox 360 report.
var xusbButtons = map[uint16]struct {
	index int
	bit   byte
}{
	btnControllerDpadUp:    {2, 0x01},
	btnControllerDpadDown:  {2, 0x02},
	btnControllerDpadLeft:  {2, 0x04},
	btnControllerDpadRight: {2, 0x08},
	btnControllerStart:     {2, 0x10},
	btnControllerSelect:    {2, 0x20},
	btnControllerThumbL:    {2, 0x40},
	btnControllerThumbR:    {2, 0x80},
	btnControllerTL:        {3, 0x01},
	btnControllerTR:        {3, 0x02},
	btnControllerMode:      {3, 0x04},
	btnControllerSouth:     {3, 0x10},
	btnControllerEast:      {3, 0x20},
	btnControllerWest:      {3, 0x40},
	btnControllerNorth:     {3, 0x80},
}

// GamepadInjectionAvailable reports whether controller buttons can be sent to Windows.
func GamepadInjectionAvailable() bool {
	gamepadMutex.Lock()
	defer gamepadMutex.Unlock()
	return openScufControl() == nil
}

// openScufControl opens the first SCUF control device. The driver allows one handle at a time,
// so it fails while iCUE runs. Callers hold gamepadMutex.
func openScufControl() error {
	if scufControl != 0 {
		return nil
	}
	for i := 0; i < scufControlDevices; i++ {
		path, _ := windows.UTF16PtrFromString(fmt.Sprintf("\\\\.\\ScufOneCtl%d", i))
		h, err := windows.CreateFile(path, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
		if err == nil {
			scufControl = h
			return nil
		}
	}
	return errors.New("SCUF driver control device not available")
}

func writeScuf(data []byte) error {
	if err := openScufControl(); err != nil {
		return err
	}
	var written uint32
	if err := windows.WriteFile(scufControl, data, &written, nil); err != nil {
		_ = windows.CloseHandle(scufControl)
		scufControl = 0
		return err
	}
	return nil
}

// writeGamepadEvent applies one evdev event to the injected report. Sticks and triggers are not
// injected, so the driver passes the controller's own analog values through; locking a trigger
// for a remapped button would turn the real trigger into an on/off switch.
func writeGamepadEvent(event *inputEvent) error {
	if event.Type != EvKey || event.Value == 2 {
		return nil
	}
	gamepadMutex.Lock()
	defer gamepadMutex.Unlock()

	pressed := event.Value != 0
	button, ok := xusbButtons[event.Code]
	if !ok {
		return nil
	}
	if pressed {
		gamepadReport[button.index] |= button.bit
	} else {
		gamepadReport[button.index] &^= button.bit
	}
	return writeScuf(gamepadReport[:])
}
