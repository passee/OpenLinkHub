package main

import (
	"OpenLinkHub/src/controller"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	serviceName        = "OpenLinkHub"
	serviceDisplayName = "OpenLinkHub"
	serviceDescription = "Controls Corsair fans, pumps, lighting and peripherals."
	logFileName        = "openlinkhub.log"
	maxLogSize         = 10 << 20

	pbtApmSuspend = 0x4
)

type serviceHandler struct{}

func (serviceHandler) Execute(_ []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	go controller.Start()
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPowerEvent}

	for request := range requests {
		switch request.Cmd {
		case svc.Interrogate:
			status <- request.CurrentStatus
		case svc.Stop, svc.Shutdown:
			status <- svc.Status{State: svc.StopPending}
			controller.Stop() // Back to hardware mode
			return false, 0
		case svc.PowerEvent:
			// Resume is handled by the monitor, which exits so the service restarts cleanly.
			if request.EventType == pbtApmSuspend {
				controller.Stop()
			}
		}
	}
	return false, 0
}

func runService(dir string) {
	redirectOutput(filepath.Join(dir, logFileName))
	if err := svc.Run(serviceName, serviceHandler{}); err != nil {
		fmt.Println("Service failed:", err)
		os.Exit(1)
	}
}

// A service has no console. Point both the Go writers and the process stderr handle at the
// log file, so runtime panics are captured as well.
func redirectOutput(path string) {
	if info, err := os.Stat(path); err == nil && info.Size() > maxLogSize {
		_ = os.Rename(path, path+".old")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	os.Stdout, os.Stderr = f, f
	_ = windows.SetStdHandle(windows.STD_OUTPUT_HANDLE, windows.Handle(f.Fd()))
	_ = windows.SetStdHandle(windows.STD_ERROR_HANDLE, windows.Handle(f.Fd()))
}

func installService(exe string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("unable to connect to the service manager (run as administrator): %w", err)
	}
	defer m.Disconnect()

	if s, err := m.OpenService(serviceName); err == nil {
		s.Close()
		return errors.New("service is already installed")
	}

	s, err := m.CreateService(serviceName, exe, mgr.Config{
		DisplayName: serviceDisplayName,
		Description: serviceDescription,
		StartType:   mgr.StartAutomatic,
	})
	if err != nil {
		return fmt.Errorf("unable to create service: %w", err)
	}
	defer s.Close()

	restart := mgr.RecoveryAction{Type: mgr.ServiceRestart, Delay: 5 * time.Second}
	if err = s.SetRecoveryActions([]mgr.RecoveryAction{restart, restart, restart}, uint32((24 * time.Hour).Seconds())); err != nil {
		return fmt.Errorf("service created, but restart-on-failure could not be set: %w", err)
	}
	// Recovery normally only triggers on a crash; also restart when the process exits with an error.
	if err = s.SetRecoveryActionsOnNonCrashFailures(true); err != nil {
		return fmt.Errorf("service created, but restart-on-error could not be set: %w", err)
	}
	fmt.Println("Service installed. Start it with: OpenLinkHub.exe start")
	return nil
}

func uninstallService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("unable to connect to the service manager (run as administrator): %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return errors.New("service is not installed")
	}
	defer s.Close()

	_, _ = s.Control(svc.Stop)
	if err = s.Delete(); err != nil {
		return fmt.Errorf("unable to delete service: %w", err)
	}
	fmt.Println("Service removed.")
	return nil
}

func startService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("unable to connect to the service manager (run as administrator): %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return errors.New("service is not installed")
	}
	defer s.Close()
	return s.Start()
}

func stopService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("unable to connect to the service manager (run as administrator): %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return errors.New("service is not installed")
	}
	defer s.Close()

	if _, err = s.Control(svc.Stop); err != nil {
		return err
	}
	for i := 0; i < 30; i++ {
		st, err := s.Query()
		if err == nil && st.State == svc.Stopped {
			return nil
		}
		time.Sleep(time.Second)
	}
	return errors.New("service did not stop within 30 seconds")
}
