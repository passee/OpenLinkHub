package sensors

import (
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const nvmlTemperatureGpu = 0

var (
	nvml            = windows.NewLazySystemDLL("nvml.dll")
	nvmlInit        = nvml.NewProc("nvmlInit_v2")
	nvmlHandleByIdx = nvml.NewProc("nvmlDeviceGetHandleByIndex_v2")
	nvmlTemperature = nvml.NewProc("nvmlDeviceGetTemperature")
	nvmlInitOnce    sync.Once
	nvmlInitErr     error
)

// GpuTemperature returns the core temperature of an NVIDIA GPU through the driver's NVML library.
func GpuTemperature(index int) (float32, error) {
	nvmlInitOnce.Do(func() {
		if err := nvml.Load(); err != nil {
			nvmlInitErr = fmt.Errorf("NVIDIA driver library not found: %w", err)
			return
		}
		if rc, _, _ := nvmlInit.Call(); rc != 0 {
			nvmlInitErr = fmt.Errorf("nvmlInit failed: %d", rc)
		}
	})
	if nvmlInitErr != nil {
		return 0, nvmlInitErr
	}

	var device uintptr
	if rc, _, _ := nvmlHandleByIdx.Call(uintptr(index), uintptr(unsafe.Pointer(&device))); rc != 0 {
		return 0, fmt.Errorf("no NVIDIA GPU at index %d: %d", index, rc)
	}
	var temp uint32
	if rc, _, _ := nvmlTemperature.Call(device, nvmlTemperatureGpu, uintptr(unsafe.Pointer(&temp))); rc != 0 {
		return 0, fmt.Errorf("nvmlDeviceGetTemperature failed: %d", rc)
	}
	return float32(temp), nil
}
