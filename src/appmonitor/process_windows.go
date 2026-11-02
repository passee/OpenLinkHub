package appmonitor

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// runningExecutables returns the executable name of every running process.
func runningExecutables() ([]string, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)

	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	names := make([]string, 0, 256)
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		names = append(names, windows.UTF16ToString(entry.ExeFile[:]))
	}
	return names, nil
}
