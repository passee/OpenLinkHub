package usb

// Raw usbfs access (Hydro / Platinum legacy coolers) has no Windows backend yet.

import (
	"errors"
	"strings"
)

var errUnsupported = errors.New("raw USB access is not supported on Windows")

type DeviceStruct struct {
	Name         string
	VendorId     uint16
	ProductID    uint16
	SerialNbr    string
	Path         string
	InterfaceNbr int
	Manufacturer string
}

type Device struct {
	dev          *DeviceStruct
	UsbdevfsBulk uintptr
}

func Init(deviceList []uint16) int { return 0 }

func Open(devicePath string) (*Device, error) { return nil, errUnsupported }

func GetDevices() []DeviceStruct { return nil }

func GetDevicesByPath(path string) *DeviceStruct { return nil }

func (h *Device) Close() error { return nil }

func (h *Device) GetSerialNbr() string { return h.dev.SerialNbr }

func (h *Device) GetMfrStr() string { return h.dev.Manufacturer }

func (h *Device) GetProductStr() string {
	product := strings.Replace(h.dev.Name, "CORSAIR ", "", -1)
	return strings.Replace(product, "Corsair ", "", -1)
}

func (h *Device) SetEndpoints(endpointOut, endpointIn int) {}

func (h *Device) SetDeviceControl(request uint8, value, index, length uint16, data uintptr) error {
	return errUnsupported
}

func (h *Device) Write(buffer []byte) error { return errUnsupported }

func (h *Device) Read(buffer []byte) error { return errUnsupported }

func (h *Device) ReadNonBlock(buffer []byte, timeoutMs uint32) error { return errUnsupported }
