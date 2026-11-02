package temperatures

import (
	"OpenLinkHub/src/logger"
	"OpenLinkHub/src/sensors"
	"sync"
	"time"
)

const (
	// Reported when a sensor has been unreadable for longer than sensorGracePeriod, so every
	// curve driven by it ends at its top point instead of idling while the hardware heats up.
	failSafeTemperature = 95
	sensorGracePeriod   = 10 * time.Second
)

type guardedSensor struct {
	mutex    sync.Mutex
	name     string
	lastGood float32
	lastRead time.Time
	failing  bool
}

func (g *guardedSensor) read(source func() (float32, error)) float32 {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	temp, err := source()
	if err == nil && temp > 0 && temp < 150 {
		if g.failing {
			logger.Log(logger.Fields{"sensor": g.name, "temperature": temp}).Info("Sensor recovered")
			g.failing = false
		}
		g.lastGood, g.lastRead = temp, time.Now()
		return temp
	}

	if !g.lastRead.IsZero() && time.Since(g.lastRead) < sensorGracePeriod {
		return g.lastGood
	}
	if !g.failing {
		logger.Log(logger.Fields{"sensor": g.name, "error": err, "failSafe": failSafeTemperature}).Error("Sensor unreadable, reporting fail-safe temperature")
		g.failing = true
	}
	return failSafeTemperature
}

var (
	cpuSensor  = &guardedSensor{name: "cpu"}
	gpuMutex   sync.Mutex
	gpuSensors = map[int]*guardedSensor{}
)

func platformCpuTemperature() (float32, bool) {
	temp := cpuSensor.read(sensors.CpuTemperature)
	if temperatureOffset != 0 {
		temp += float32(temperatureOffset)
	}
	return temp, true
}

func platformNvidiaTemperature(gpuIndex int) (float32, bool) {
	gpuMutex.Lock()
	sensor, ok := gpuSensors[gpuIndex]
	if !ok {
		sensor = &guardedSensor{name: "nvidia-gpu"}
		gpuSensors[gpuIndex] = sensor
	}
	gpuMutex.Unlock()

	return sensor.read(func() (float32, error) { return sensors.GpuTemperature(gpuIndex) }), true
}
