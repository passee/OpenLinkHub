package systeminfo

func platformFill(si *SystemInfo) bool { return false }

func platformCpuUtilization() (float64, bool) { return 0, false }

func platformGpuUtilization() (int, bool) { return 0, false }

func platformMemoryUsage() (*MemoryUsage, bool) { return nil, false }

func platformStorageUsage() ([]StorageUsage, bool) { return nil, false }

const hwmonSupported = true
