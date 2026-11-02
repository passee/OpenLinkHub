package inputmanager

import (
	"OpenLinkHub/src/logger"
	"errors"
	"os"
	"syscall"
	"unsafe"
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
		return os.OpenFile("/dev/uinput", os.O_RDWR|syscall.O_NONBLOCK, 0660)
	}
	return os.OpenFile("/dev/uinput", os.O_WRONLY, 0660)
}

func sysIoctl(fd, req, arg uintptr) (uintptr, uintptr, syscall.Errno) {
	return syscall.Syscall(syscall.SYS_IOCTL, fd, req, arg)
}

func sysSetNonblock(fd uintptr) (uintptr, uintptr, syscall.Errno) {
	return syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_SETFL, syscall.O_NONBLOCK)
}

// writeVirtualEvent will send event to virtual keyboard device
func writeVirtualEvent(f *os.File, event *inputEvent) error {
	if f == nil {
		logger.Log(logger.Fields{}).Error("No valid virtual inputs")
		return errors.New("no valid virtual inputs")
	}

	buf := (*(*[unsafe.Sizeof(*event)]byte)(unsafe.Pointer(event)))[:]
	_, err := f.Write(buf)
	if err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Failed to send virtual inputs event")
		return err
	}
	return nil
}
