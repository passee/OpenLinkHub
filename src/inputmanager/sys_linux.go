package inputmanager

import "syscall"

func sysIoctl(fd, req, arg uintptr) (uintptr, uintptr, syscall.Errno) {
	return syscall.Syscall(syscall.SYS_IOCTL, fd, req, arg)
}

func sysSetNonblock(fd uintptr) (uintptr, uintptr, syscall.Errno) {
	return syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_SETFL, syscall.O_NONBLOCK)
}
