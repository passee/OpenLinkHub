// Package sounds plays iCUE's headset voice prompts ("Mic on", "Battery low", ...) on the
// headset. iCUE plays them from the PC, the headset firmware has none of its own.
package sounds

import (
	"OpenLinkHub/src/config"
	"OpenLinkHub/src/logger"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The prompt files are headerless PCM: 44.1 kHz, 16-bit, stereo.
const (
	sampleRate    = 44100
	channels      = 2
	bitsPerSample = 16
	waveMapper    = 0xFFFFFFFF
	whdrDone      = 0x00000001
	mmNoError     = 0
	nameLength    = 32
)

type waveFormatEx struct {
	formatTag      uint16
	channels       uint16
	samplesPerSec  uint32
	avgBytesPerSec uint32
	blockAlign     uint16
	bitsPerSample  uint16
	size           uint16
}

type waveHdr struct {
	data          uintptr
	bufferLength  uint32
	bytesRecorded uint32
	user          uintptr
	flags         uint32
	loops         uint32
	next          uintptr
	reserved      uintptr
}

type waveOutCaps struct {
	mid           uint16
	pid           uint16
	driverVersion uint32
	name          [nameLength]uint16
	formats       uint32
	channels      uint16
	reserved      uint16
	support       uint32
}

var (
	winmm                = windows.NewLazySystemDLL("winmm.dll")
	procWaveOutGetNum    = winmm.NewProc("waveOutGetNumDevs")
	procWaveOutGetCaps   = winmm.NewProc("waveOutGetDevCapsW")
	procWaveOutOpen      = winmm.NewProc("waveOutOpen")
	procWaveOutPrepare   = winmm.NewProc("waveOutPrepareHeader")
	procWaveOutWrite     = winmm.NewProc("waveOutWrite")
	procWaveOutUnprepare = winmm.NewProc("waveOutUnprepareHeader")
	procWaveOutClose     = winmm.NewProc("waveOutClose")

	playMutex sync.Mutex
)

// soundPath prefers the copy next to OpenLinkHub, then an installed iCUE.
func soundPath(name string) (string, error) {
	candidates := []string{
		filepath.Join(config.GetConfig().ConfigPath, "sounds", name+".wav"),
		filepath.Join(os.Getenv("ProgramFiles"), "Corsair", "Corsair iCUE5 Software", "sounds", "default", name+".wav"),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", errors.New("voice prompt not found: " + name)
}

// outputDevice returns the waveOut device whose name contains match, or the default device.
func outputDevice(match string) uintptr {
	count, _, _ := procWaveOutGetNum.Call()
	for i := uintptr(0); i < count; i++ {
		var caps waveOutCaps
		if r, _, _ := procWaveOutGetCaps.Call(i, uintptr(unsafe.Pointer(&caps)), unsafe.Sizeof(caps)); r != mmNoError {
			continue
		}
		if strings.Contains(windows.UTF16ToString(caps.name[:]), match) {
			return i
		}
	}
	return waveMapper
}

// Play plays a prompt on the output whose name contains device, in the background. Prompts
// never overlap; a new one waits for the previous to finish.
func Play(name, device string) {
	go func() {
		if err := play(name, device); err != nil {
			logger.Log(logger.Fields{"error": err, "prompt": name}).Warn("Unable to play voice prompt")
		}
	}()
}

func play(name, device string) error {
	path, err := soundPath(name)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return errors.New("empty voice prompt: " + name)
	}

	playMutex.Lock()
	defer playMutex.Unlock()

	format := waveFormatEx{
		formatTag:      1, // WAVE_FORMAT_PCM
		channels:       channels,
		samplesPerSec:  sampleRate,
		avgBytesPerSec: sampleRate * channels * bitsPerSample / 8,
		blockAlign:     channels * bitsPerSample / 8,
		bitsPerSample:  bitsPerSample,
	}
	var handle uintptr
	if r, _, _ := procWaveOutOpen.Call(uintptr(unsafe.Pointer(&handle)), outputDevice(device), uintptr(unsafe.Pointer(&format)), 0, 0, 0); r != mmNoError {
		return errors.New("waveOutOpen failed")
	}
	defer procWaveOutClose.Call(handle)

	header := waveHdr{data: uintptr(unsafe.Pointer(&data[0])), bufferLength: uint32(len(data))}
	size := unsafe.Sizeof(header)
	if r, _, _ := procWaveOutPrepare.Call(handle, uintptr(unsafe.Pointer(&header)), size); r != mmNoError {
		return errors.New("waveOutPrepareHeader failed")
	}
	defer procWaveOutUnprepare.Call(handle, uintptr(unsafe.Pointer(&header)), size)
	if r, _, _ := procWaveOutWrite.Call(handle, uintptr(unsafe.Pointer(&header)), size); r != mmNoError {
		return errors.New("waveOutWrite failed")
	}

	duration := time.Duration(len(data)) * time.Second / time.Duration(format.avgBytesPerSec)
	deadline := time.Now().Add(duration + 2*time.Second)
	for atomic.LoadUint32(&header.flags)&whdrDone == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	// Keeps data alive until the device has finished reading it.
	_ = data[len(data)-1]
	return nil
}
