// Package pawnio runs signed PawnIO modules, which give user mode access to CPU registers,
// PCI configuration space and the SMBus without a driver of our own.
package pawnio

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

var (
	//go:embed modules/AMDFamily17.bin
	AMDFamily17 []byte

	//go:embed modules/SmbusPIIX4.bin
	SmbusPIIX4 []byte
)

type Executor struct {
	mutex   sync.Mutex
	handle  uintptr
	execute *windows.LazyProc
}

// Open loads a module into a new PawnIO executor. It needs administrator rights.
func Open(module []byte) (*Executor, error) {
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
	return &Executor{handle: handle, execute: dll.NewProc("pawnio_execute")}, nil
}

// Call runs a module function and returns outCount output cells.
func (e *Executor) Call(function string, in []uint64, outCount int) ([]uint64, error) {
	name, err := windows.BytePtrFromString(function)
	if err != nil {
		return nil, err
	}
	out := make([]uint64, outCount+1) // never empty, so &out[0] is valid
	var returned uintptr
	var inPtr uintptr
	if len(in) > 0 {
		inPtr = uintptr(unsafe.Pointer(&in[0]))
	}

	e.mutex.Lock()
	defer e.mutex.Unlock()
	hr, _, _ := e.execute.Call(
		e.handle,
		uintptr(unsafe.Pointer(name)),
		inPtr, uintptr(len(in)),
		uintptr(unsafe.Pointer(&out[0])), uintptr(outCount),
		uintptr(unsafe.Pointer(&returned)),
	)
	if hr != 0 {
		return nil, fmt.Errorf("%s failed: 0x%08x", function, uint32(hr))
	}
	return out[:outCount], nil
}

var (
	systemMutexes   = map[string]windows.Handle{}
	systemMutexLock sync.Mutex
)

// WithSystemMutex runs fn while holding a named mutex that other hardware monitoring tools
// use to serialise access to the same bus.
func WithSystemMutex(name string, timeoutMs uint32, fn func() error) error {
	systemMutexLock.Lock()
	handle, ok := systemMutexes[name]
	if !ok {
		ptr, _ := windows.UTF16PtrFromString(name)
		h, err := windows.CreateMutex(nil, false, ptr)
		if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			systemMutexLock.Unlock()
			return fmt.Errorf("unable to create %s mutex: %w", name, err)
		}
		handle = h
		systemMutexes[name] = h
	}
	systemMutexLock.Unlock()

	// Mutex ownership is per OS thread; without pinning, the release can run on another
	// thread, fail, and leave the mutex held for every other user of the bus.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	event, err := windows.WaitForSingleObject(handle, timeoutMs)
	if err != nil || (event != windows.WAIT_OBJECT_0 && event != windows.WAIT_ABANDONED) {
		return fmt.Errorf("timed out waiting for %s", name)
	}
	defer func() { _ = windows.ReleaseMutex(handle) }()
	return fn()
}
