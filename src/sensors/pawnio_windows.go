package sensors

import (
	"OpenLinkHub/src/pawnio"
	"sync"
)

// SMN register holding the current Tctl reading on Zen (family 17h-1Ah) CPUs.
const smnThmTconCurTmp = 0x00059800

var (
	cpuExecutor     *pawnio.Executor
	cpuExecutorErr  error
	cpuExecutorOnce sync.Once
)

func readSmn(offset uint32) (uint32, error) {
	var value uint32
	err := pawnio.WithSystemMutex(`Global\Access_PCI`, 500, func() error {
		out, err := cpuExecutor.Call("ioctl_read_smn", []uint64{uint64(offset)}, 1)
		if err != nil {
			return err
		}
		value = uint32(out[0])
		return nil
	})
	return value, err
}

// CpuTemperature returns the AMD Zen Tctl temperature in degrees Celsius.
func CpuTemperature() (float32, error) {
	cpuExecutorOnce.Do(func() {
		cpuExecutor, cpuExecutorErr = pawnio.Open(pawnio.AMDFamily17)
	})
	if cpuExecutorErr != nil {
		return 0, cpuExecutorErr
	}

	raw, err := readSmn(smnThmTconCurTmp)
	if err != nil {
		return 0, err
	}
	temp := float32((raw>>21)&0x7FF) * 0.125
	if raw&(1<<19) != 0 {
		temp -= 49
	}
	return temp, nil
}
