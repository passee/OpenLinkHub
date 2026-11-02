package sensors

import (
	"os"
	"testing"
)

func TestReadHardware(t *testing.T) {
	if os.Getenv("OLH_HW_TEST") == "" {
		t.Skip("set OLH_HW_TEST=1 and run elevated")
	}
	cpu, err := CpuTemperature()
	t.Logf("cpu=%.2f err=%v", cpu, err)
	gpu, err := GpuTemperature(0)
	t.Logf("gpu=%.1f err=%v", gpu, err)
}
