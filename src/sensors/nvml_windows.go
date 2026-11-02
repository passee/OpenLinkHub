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
	device, err := nvmlDevice(index)
	if err != nil {
		return 0, err
	}
	var temp uint32
	if rc, _, _ := nvmlTemperature.Call(device, nvmlTemperatureGpu, uintptr(unsafe.Pointer(&temp))); rc != 0 {
		return 0, fmt.Errorf("nvmlDeviceGetTemperature failed: %d", rc)
	}
	return float32(temp), nil
}

var (
	nvmlCount       = nvml.NewProc("nvmlDeviceGetCount_v2")
	nvmlName        = nvml.NewProc("nvmlDeviceGetName")
	nvmlUtilization = nvml.NewProc("nvmlDeviceGetUtilizationRates")
)

func nvmlDevice(index int) (uintptr, error) {
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
	return device, nil
}

// GpuCount returns the number of NVIDIA GPUs, 0 when none or no driver.
func GpuCount() int {
	if _, err := nvmlDevice(0); err != nil {
		return 0
	}
	var count uint32
	if rc, _, _ := nvmlCount.Call(uintptr(unsafe.Pointer(&count))); rc != 0 {
		return 0
	}
	return int(count)
}

// GpuName returns the marketing name of an NVIDIA GPU.
func GpuName(index int) string {
	device, err := nvmlDevice(index)
	if err != nil {
		return ""
	}
	buf := make([]byte, 96)
	if rc, _, _ := nvmlName.Call(device, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))); rc != 0 {
		return ""
	}
	return windows.ByteSliceToString(buf)
}

// GpuUtilization returns the GPU core load in percent, -1 when unavailable.
func GpuUtilization(index int) int {
	device, err := nvmlDevice(index)
	if err != nil {
		return -1
	}
	var rates struct{ gpu, memory uint32 }
	if rc, _, _ := nvmlUtilization.Call(device, uintptr(unsafe.Pointer(&rates))); rc != 0 {
		return -1
	}
	return int(rates.gpu)
}
