package hid

import (
	"sync"
	"time"
)

// ResponseTimeout bounds ReadResponse. Replies to a command arrive within milliseconds; a device
// that is switched off or asleep never replies.
const ResponseTimeout = 3 * time.Second

// ReadResponse reads the reply to a command just written. Unlike Read it gives up after
// ResponseTimeout, so an unresponsive device cannot block its caller forever.
func (d *Device) ReadResponse(p []byte) (int, error) {
	return d.ReadWithTimeout(p, ResponseTimeout)
}

var exchangeLocks sync.Map // *Device -> *sync.Mutex

// ExchangeLock serializes command/reply pairs on one device handle. A receiver and the devices
// paired to it share the handle; without a common lock one of them can read the other's reply.
func (d *Device) ExchangeLock() *sync.Mutex {
	lock, _ := exchangeLocks.LoadOrStore(d, &sync.Mutex{})
	return lock.(*sync.Mutex)
}
