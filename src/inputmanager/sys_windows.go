package inputmanager

// uinput does not exist on Windows; opening /dev/uinput already fails before these are reached.

import "syscall"

func sysIoctl(fd, req, arg uintptr) (uintptr, uintptr, syscall.Errno) {
	return 0, 0, syscall.EWINDOWS
}

func sysSetNonblock(fd uintptr) (uintptr, uintptr, syscall.Errno) {
	return 0, 0, syscall.EWINDOWS
}
