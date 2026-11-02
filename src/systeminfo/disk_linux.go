package systeminfo

import "syscall"

func diskStats(mountPoint string) (total, free, avail uint64, ok bool) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(mountPoint, &stat); err != nil {
		return 0, 0, 0, false
	}
	if stat.Bsize <= 0 || stat.Blocks == 0 || stat.Bfree > stat.Blocks || stat.Bavail > stat.Bfree {
		return 0, 0, 0, false
	}
	bs := uint64(stat.Bsize)
	return stat.Blocks * bs, stat.Bfree * bs, stat.Bavail * bs, true
}
