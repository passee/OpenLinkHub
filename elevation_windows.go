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

// iCUE's background service keeps polling the Commander Pro even while iCUE is closed. Its
// replies arrive on every open handle, so OpenLinkHub would read them as answers to its own
// commands and show wrong fan, voltage and port data.
const conflictingService = "CorsairService"

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

// pauseConflictingService stops iCUE's device service and returns a function that starts it
// again, or nil when it was not running.
func pauseConflictingService() func() {
	m, err := mgr.Connect()
	if err != nil {
		return nil
	}
	defer m.Disconnect()

	s, err := m.OpenService(conflictingService)
	if err != nil {
		return nil
	}
	defer s.Close()

	status, err := s.Query()
	if err != nil || status.State != svc.Running {
		return nil
	}
	if _, err = s.Control(svc.Stop); err != nil {
		fmt.Printf("Unable to pause %s, device readings may be wrong: %v\n", conflictingService, err)
		return nil
	}
	for i := 0; i < 20; i++ {
		if status, err = s.Query(); err == nil && status.State == svc.Stopped {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	fmt.Printf("Paused %s while OpenLinkHub runs\n", conflictingService)

	return func() {
		m, err := mgr.Connect()
		if err != nil {
			return
		}
		defer m.Disconnect()
		if s, err := m.OpenService(conflictingService); err == nil {
			_ = s.Start()
			s.Close()
		}
	}
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
