package temperatures

func platformCpuTemperature() (float32, bool) { return 0, false }

func platformNvidiaTemperature(gpuIndex int) (float32, bool) { return 0, false }

const hwmonSupported = true
