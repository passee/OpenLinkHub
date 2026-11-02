package systeminfo

import (
	"OpenLinkHub/src/sensors"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procGetSystemTimes       = kernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")

	cpuTimesMutex sync.Mutex
	lastIdle      uint64
	lastTotal     uint64
)

func readRegistryString(path, name string) string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()
	value, _, err := key.GetStringValue(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

// platformFill collects system information from Windows sources instead of /proc and /sys.
func platformFill(si *SystemInfo) bool {
	si.CPU = &CpuData{
		Model:   readRegistryString(`HARDWARE\DESCRIPTION\System\CentralProcessor\0`, "ProcessorNameString"),
		Threads: runtime.NumCPU(),
	}
	si.Kernel = &KernelData{OsType: "Windows", Architecture: runtime.GOARCH}

	bios := `HARDWARE\DESCRIPTION\System\BIOS`
	si.Motherboard = &MotherboardData{
		Model:    strings.TrimSpace(readRegistryString(bios, "BaseBoardManufacturer") + " " + readRegistryString(bios, "BaseBoardProduct")),
		BIOS:     readRegistryString(bios, "BIOSVersion"),
		BIOSDate: readRegistryString(bios, "BIOSReleaseDate"),
	}

	si.GPU = make(map[int]GpuData)
	for i := 0; i < sensors.GpuCount(); i++ {
		temp, _ := sensors.GpuTemperature(i)
		si.GPU[i] = GpuData{Index: i, Model: sensors.GpuName(i), Temperature: temp}
	}
	return true
}

func filetimeToUint64(ft windows.Filetime) uint64 {
	return uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
}

// platformCpuUtilization returns the CPU load since the previous call, in percent.
func platformCpuUtilization() (float64, bool) {
	var idle, kernel, user windows.Filetime
	if r, _, _ := procGetSystemTimes.Call(uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user))); r == 0 {
		return 0, true
	}
	cpuTimesMutex.Lock()
	defer cpuTimesMutex.Unlock()

	// Kernel time includes idle time.
	idleNow := filetimeToUint64(idle)
	totalNow := filetimeToUint64(kernel) + filetimeToUint64(user)
	first := lastTotal == 0
	idleDelta, totalDelta := idleNow-lastIdle, totalNow-lastTotal
	lastIdle, lastTotal = idleNow, totalNow
	if first || totalDelta == 0 {
		return 0, true
	}
	return float64(totalDelta-idleDelta) / float64(totalDelta) * 100, true
}

func platformGpuUtilization() (int, bool) {
	if load := sensors.GpuUtilization(0); load >= 0 {
		return load, true
	}
	return 0, false
}

func platformMemoryUsage() (*MemoryUsage, bool) {
	var status struct {
		length               uint32
		memoryLoad           uint32
		totalPhys            uint64
		availPhys            uint64
		totalPageFile        uint64
		availPageFile        uint64
		totalVirtual         uint64
		availVirtual         uint64
		availExtendedVirtual uint64
	}
	status.length = uint32(unsafe.Sizeof(status))
	if r, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status))); r == 0 {
		return nil, true
	}
	used := status.totalPhys - status.availPhys
	usage := &MemoryUsage{
		Total:     status.totalPhys,
		Available: status.availPhys,
		Used:      used,
	}
	if status.totalPhys > 0 {
		usage.UsedPercent = float64(used) / float64(status.totalPhys) * 100
	}
	return usage, true
}

func platformStorageUsage() ([]StorageUsage, bool) {
	drives, err := windows.GetLogicalDrives()
	if err != nil {
		return nil, true
	}
	data := make([]StorageUsage, 0)
	for i := 0; i < 26; i++ {
		if drives&(1<<uint(i)) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		rootPtr, _ := windows.UTF16PtrFromString(root)
		if windows.GetDriveType(rootPtr) != windows.DRIVE_FIXED {
			continue
		}
		usage := StorageUsage{MountPoint: root, Device: root}
		if total, free, avail, ok := diskStats(root); ok {
			usage.Total, usage.Used, usage.Available = total, total-free, avail
			usage.UsedPercent = float64(usage.Used) / float64(total) * 100
			usage.AvailableData = true
		}
		data = append(data, usage)
	}
	return data, true
}
