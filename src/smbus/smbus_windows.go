package smbus

// The SMBus is driven through PawnIO's signed SmbusPIIX4 module, which covers the AMD chipset
// SMBus controllers that RAM lighting sits on. The *os.File parameters only keep the Linux API;
// every transfer goes through the shared executor.

import (
	"OpenLinkHub/src/pawnio"
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"sync"
)

const (
	smbusRead      = 1
	smbusWrite     = 0
	smbusByteData  = 2
	smbusWordData  = 3
	smbusBlockData = 5
	smbusBlockMax  = 32

	// Shared with other tools that drive the same controller, such as RGB and sensor software.
	smbusMutex     = `Global\Access_SMBUS.HTP.Method`
	smbusTimeoutMs = 1000
	// The RAM module derives a bus number from the "i2c-N" form.
	busPath = "i2c-0"
)

type SmBus struct {
	Path string
}

type Connection struct {
	File *os.File
}

type cacheKey struct {
	addr uint8
	reg  uint8
}

var (
	executor     *pawnio.Executor
	executorErr  error
	executorOnce sync.Once
	cacheMutex   sync.Mutex
	cacheData    = map[cacheKey][]byte{}
)

func open() error {
	executorOnce.Do(func() {
		executor, executorErr = pawnio.Open(pawnio.SmbusPIIX4)
	})
	return executorErr
}

func transfer(address, readWrite, command, protocol uint8, data []uint64, outCount int) ([]uint64, error) {
	if err := open(); err != nil {
		return nil, err
	}
	in := append([]uint64{uint64(address), uint64(readWrite), uint64(command), uint64(protocol)}, data...)
	var out []uint64
	var callErr error
	// One retry: a missed probe hides that RAM module until the next restart.
	for attempt := 0; attempt < 2; attempt++ {
		lockErr := pawnio.WithSystemMutex(smbusMutex, smbusTimeoutMs, func() error {
			out, callErr = executor.Call("ioctl_smbus_xfer", in, outCount)
			return nil
		})
		if lockErr == nil {
			return out, callErr
		}
		callErr = lockErr
	}
	return nil, callErr
}

// GetSmBus returns the chipset SMBus when the PawnIO module can drive it.
func GetSmBus() (*SmBus, error) {
	if err := open(); err != nil {
		return nil, err
	}
	if _, err := executor.Call("ioctl_identity", nil, 3); err != nil {
		return nil, errors.New("no supported SMBus controller found")
	}
	return &SmBus{Path: busPath}, nil
}

func Open(device string) (*Connection, error) {
	if err := open(); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	return &Connection{File: f}, nil
}

// WriteBlockData will write a byte array to register
func WriteBlockData(_ *os.File, address, register uint8, buf []byte) error {
	if len(buf) > smbusBlockMax {
		return errors.New("buffer is too long for this type")
	}

	k := cacheKey{addr: address, reg: register}
	cacheMutex.Lock()
	prev, ok := cacheData[k]
	cacheMutex.Unlock()
	if ok && bytes.Equal(prev, buf) {
		return nil
	}

	// The module takes the length byte followed by the data, packed little endian into
	// five 64-bit cells.
	packed := make([]byte, 40)
	packed[0] = byte(len(buf))
	copy(packed[1:], buf)
	cells := make([]uint64, 5)
	for i := range cells {
		cells[i] = binary.LittleEndian.Uint64(packed[i*8:])
	}

	if _, err := transfer(address, smbusWrite, register, smbusBlockData, cells, 0); err != nil {
		return err
	}
	cp := make([]byte, len(buf))
	copy(cp, buf)
	cacheMutex.Lock()
	cacheData[k] = cp
	cacheMutex.Unlock()
	return nil
}

// ReadRegister will read byte from a register
func ReadRegister(_ *os.File, address, register uint8) (uint8, error) {
	out, err := transfer(address, smbusRead, register, smbusByteData, nil, 1)
	if err != nil {
		return 0, err
	}
	return uint8(out[0]), nil
}

// ReadWord reads a 2-bytes word from a designated register.
func ReadWord(_ *os.File, addr, reg uint8) (uint16, error) {
	out, err := transfer(addr, smbusRead, reg, smbusWordData, nil, 1)
	if err != nil {
		return 0, err
	}
	return uint16(out[0]), nil
}
