package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// iCUE's services keep talking to the hardware while iCUE is closed. CorsairService polls the
// Commander Pro, and its replies reach every open handle, so OpenLinkHub would read them as
// answers to its own commands. CorsairDeviceControlService drives the RAM over the SMBus.
var conflictingServices = []string{"CorsairService", "CorsairDeviceControlService"}

func isElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// relaunchElevated restarts this executable with the same arguments through a UAC prompt.
func relaunchElevated(exe string) error {
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exe)
	args, _ := windows.UTF16PtrFromString(strings.Join(os.Args[1:], " "))
	cwd, _ := windows.UTF16PtrFromString(filepath.Dir(exe))
	return windows.ShellExecute(0, verb, file, args, cwd, windows.SW_SHOWMINNOACTIVE)
}

// pauseConflictingServices stops iCUE's device services and returns a function that starts
// the ones it stopped again.
func pauseConflictingServices() func() {
	m, err := mgr.Connect()
	if err != nil {
		return func() {}
	}
	defer m.Disconnect()

	var paused []string
	for _, name := range conflictingServices {
		if stopService(m, name) {
			paused = append(paused, name)
			fmt.Printf("Paused %s while OpenLinkHub runs\n", name)
		}
	}

	return func() {
		m, err := mgr.Connect()
		if err != nil {
			return
		}
		defer m.Disconnect()
		for _, name := range paused {
			if s, err := m.OpenService(name); err == nil {
				_ = s.Start()
				s.Close()
			}
		}
	}
}

func stopService(m *mgr.Mgr, name string) bool {
	s, err := m.OpenService(name)
	if err != nil {
		return false
	}
	defer s.Close()

	status, err := s.Query()
	if err != nil || status.State != svc.Running {
		return false
	}
	if _, err = s.Control(svc.Stop); err != nil {
		fmt.Printf("Unable to pause %s, device readings may be wrong: %v\n", name, err)
		return false
	}
	for i := 0; i < 20; i++ {
		if status, err = s.Query(); err == nil && status.State == svc.Stopped {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	return true
}

const (
	supervisorLockName = `Local\OpenLinkHubSupervisor`
	workerLockName     = `Local\OpenLinkHubWorker`
	// exitAlreadyRunning tells the supervisor not to restart a worker that found another one.
	exitAlreadyRunning = 3
)

// claimInstance reports whether this is the only process holding the named lock. Two instances
// would both talk to every device, and the Commander Pro answers each command on all open
// handles, so their readings scramble each other. The handle stays open until exit.
func claimInstance(name string) bool {
	ptr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return false
	}
	_, err = windows.CreateMutex(nil, false, ptr)
	return err == nil
}
