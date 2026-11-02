package smbus

// RAM lighting needs an SMBus driver (PawnIO) on Windows; until that backend exists every call fails.

import (
	"errors"
	"os"
)

var errUnsupported = errors.New("SMBus access is not supported on Windows yet")

type SmBus struct {
	Path string
}

type Connection struct {
	File *os.File
}

func GetSmBus() (*SmBus, error) { return nil, errUnsupported }

func Open(device string) (*Connection, error) { return nil, errUnsupported }

func WriteBlockData(f *os.File, address, register uint8, buf []byte) error { return errUnsupported }

func ReadRegister(f *os.File, address, register uint8) (uint8, error) { return 0, errUnsupported }

func ReadWord(f *os.File, addr, reg uint8) (uint16, error) { return 0, errUnsupported }
