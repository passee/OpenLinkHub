package serial

import (
	"errors"
	"time"
)

var errUnsupported = errors.New("serial devices are not supported on Windows yet")

type Device struct{}

type Config struct {
	Name string
	Baud int
}

func Open(c *Config) (*Device, error) { return nil, errUnsupported }

func (d *Device) Close() error { return nil }

func (d *Device) ReadTimeout(buf []byte, timeout time.Duration) (int, error) {
	return 0, errUnsupported
}

func (d *Device) Read(buf []byte) (int, error) { return 0, errUnsupported }

func (d *Device) Write(buf []byte) (int, error) { return 0, errUnsupported }
