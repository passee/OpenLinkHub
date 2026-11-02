//go:build !windows

package hid

func openMatching(vid, pid uint16, serial string) (*Device, error) {
	return openFirstMatch(vid, pid, serial)
}
