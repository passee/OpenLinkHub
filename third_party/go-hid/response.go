package hid

import "time"

// ResponseTimeout bounds ReadResponse. Replies to a command arrive within milliseconds; a device
// that is switched off or asleep never replies.
const ResponseTimeout = 3 * time.Second

// ReadResponse reads the reply to a command just written. Unlike Read it gives up after
// ResponseTimeout, so an unresponsive device cannot block its caller forever.
func (d *Device) ReadResponse(p []byte) (int, error) {
	return d.ReadWithTimeout(p, ResponseTimeout)
}
