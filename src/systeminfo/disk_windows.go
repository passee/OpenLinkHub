package systeminfo

import "golang.org/x/sys/windows"

func diskStats(mountPoint string) (total, free, avail uint64, ok bool) {
	path, err := windows.UTF16PtrFromString(mountPoint)
	if err != nil {
		return 0, 0, 0, false
	}
	if err = windows.GetDiskFreeSpaceEx(path, &avail, &total, &free); err != nil || total == 0 {
		return 0, 0, 0, false
	}
	return total, free, avail, true
}
