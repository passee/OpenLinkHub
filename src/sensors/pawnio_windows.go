package sensors

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

//go:embed modules/AMDFamily17.bin
var amdFamily17Module []byte

// SMN register holding the current Tctl reading on Zen (family 17h-1Ah) CPUs.
const smnThmTconCurTmp = 0x00059800

type pawnioExecutor struct {
	handle  uintptr
	execute *windows.LazyProc
	pciLock windows.Handle
}

var (
	cpuExecutor     *pawnioExecutor
	cpuExecutorErr  error
	cpuExecutorOnce sync.Once
	cpuMutex        sync.Mutex
)

func openPawnio(module []byte) (*pawnioExecutor, error) {
	dll := windows.NewLazyDLL(filepath.Join(os.Getenv("ProgramFiles"), "PawnIO", "PawnIOLib.dll"))
	if err := dll.Load(); err != nil {
		return nil, fmt.Errorf("PawnIO is not installed: %w", err)
	}

	var handle uintptr
	if hr, _, _ := dll.NewProc("pawnio_open").Call(uintptr(unsafe.Pointer(&handle))); hr != 0 {
		return nil, fmt.Errorf("pawnio_open failed (needs administrator rights): 0x%08x", uint32(hr))
	}
	hr, _, _ := dll.NewProc("pawnio_load").Call(handle, uintptr(unsafe.Pointer(&module[0])), uintptr(len(module)))
	if hr != 0 {
		_, _, _ = dll.NewProc("pawnio_close").Call(handle)
		return nil, fmt.Errorf("pawnio_load failed: 0x%08x", uint32(hr))
	}

	// Other monitoring tools serialise PCI config access through this mutant; share it.
	name, _ := windows.UTF16PtrFromString(`Global\Access_PCI`)
	lock, err := windows.CreateMutex(nil, false, name)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return nil, fmt.Errorf("unable to create Access_PCI mutex: %w", err)
	}

	return &pawnioExecutor{handle: handle, execute: dll.NewProc("pawnio_execute"), pciLock: lock}, nil
}

func (p *pawnioExecutor) call(function string, in []uint64, outCount int) ([]uint64, error) {
	name, err := windows.BytePtrFromString(function)
	if err != nil {
		return nil, err
	}
	out := make([]uint64, outCount)
	var returned uintptr
	var inPtr uintptr
	if len(in) > 0 {
		inPtr = uintptr(unsafe.Pointer(&in[0]))
	}
	hr, _, _ := p.execute.Call(
		p.handle,
		uintptr(unsafe.Pointer(name)),
		inPtr, uintptr(len(in)),
		uintptr(unsafe.Pointer(&out[0])), uintptr(outCount),
		uintptr(unsafe.Pointer(&returned)),
	)
	if hr != 0 {
		return nil, fmt.Errorf("%s failed: 0x%08x", function, uint32(hr))
	}
	return out, nil
}

func (p *pawnioExecutor) readSmn(offset uint32) (uint32, error) {
	// Mutex ownership is per OS thread; without pinning, the release can run on another
	// thread, fail, and leave the mutex held for every other PCI user on the system.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	event, err := windows.WaitForSingleObject(p.pciLock, 500)
	if err != nil || (event != windows.WAIT_OBJECT_0 && event != windows.WAIT_ABANDONED) {
		return 0, errors.New("timed out waiting for Access_PCI mutex")
	}
	defer func() { _ = windows.ReleaseMutex(p.pciLock) }()

	out, err := p.call("ioctl_read_smn", []uint64{uint64(offset)}, 1)
	if err != nil {
		return 0, err
	}
	return uint32(out[0]), nil
}

// CpuTemperature returns the AMD Zen Tctl temperature in degrees Celsius.
func CpuTemperature() (float32, error) {
	cpuExecutorOnce.Do(func() {
		cpuExecutor, cpuExecutorErr = openPawnio(amdFamily17Module)
	})
	if cpuExecutorErr != nil {
		return 0, cpuExecutorErr
	}

	cpuMutex.Lock()
	defer cpuMutex.Unlock()

	raw, err := cpuExecutor.readSmn(smnThmTconCurTmp)
	if err != nil {
		return 0, err
	}
	temp := float32((raw>>21)&0x7FF) * 0.125
	if raw&(1<<19) != 0 {
		temp -= 49
	}
	return temp, nil
}
