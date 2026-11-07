// Package livecolors keeps the last colour frame each device was sent, so a client can show
// what the LEDs display right now, effects included.
package livecolors

import (
	"sync"
	"time"
)

type Frame struct {
	Data    []int     `json:"data"`
	Updated time.Time `json:"updated"`
}

var (
	mutex  sync.RWMutex
	frames = map[string]Frame{}
)

// Store records the raw colour buffer exactly as the driver laid it out for the device.
func Store(serial string, data []byte) {
	frame := Frame{Data: make([]int, len(data)), Updated: time.Now()}
	for i, b := range data {
		frame.Data[i] = int(b)
	}
	mutex.Lock()
	frames[serial] = frame
	mutex.Unlock()
}

// Get returns the last frame sent to a device.
func Get(serial string) (Frame, bool) {
	mutex.RLock()
	defer mutex.RUnlock()
	frame, ok := frames[serial]
	return frame, ok
}
