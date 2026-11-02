package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	logFileName = "openlinkhub.log"
	maxLogSize  = 10 << 20

	// Interactive users may signal the event, so "OpenLinkHub.exe stop" works without elevation.
	stopEventName = `Local\OpenLinkHubStop`
	stopEventSDDL = "D:(A;;GA;;;SY)(A;;GA;;;BA)(A;;0x1F0003;;;IU)"

	restartDelay     = 3 * time.Second
	crashWindow      = time.Minute
	crashesPerWindow = 5
	crashBackoff     = time.Minute
)

// supervise runs the worker and starts it again whenever it exits with an error, so a crash or
// a resume-triggered exit costs a few seconds of control instead of leaving devices unattended.
func supervise(exe string) int {
	logFile := openLog(filepath.Join(filepath.Dir(exe), logFileName))
	if logFile != nil {
		defer logFile.Close()
	}

	// Ctrl+C reaches the worker through the shared console; the supervisor must survive it and
	// wait for the worker to return devices to hardware mode. signal.Ignore would not do: it
	// passes the event to the default handler, which terminates the process.
	ignored := make(chan os.Signal, 1)
	signal.Notify(ignored, os.Interrupt, syscall.SIGTERM)
	go func() {
		for range ignored {
		}
	}()

	var crashes []time.Time
	for {
		code := runChild(exe, logFile)
		if code == 0 {
			return 0
		}

		now := time.Now()
		crashes = append(crashes, now)
		for len(crashes) > 0 && now.Sub(crashes[0]) > crashWindow {
			crashes = crashes[1:]
		}
		delay := restartDelay
		if len(crashes) >= crashesPerWindow {
			delay = crashBackoff
		}
		logLine(logFile, fmt.Sprintf("worker exited with code %d, restarting in %s", code, delay))
		time.Sleep(delay)
	}
}

func runChild(exe string, logFile *os.File) int {
	cmd := exec.Command(exe, "run")
	cmd.Dir = filepath.Dir(exe)
	var out io.Writer = os.Stdout
	if logFile != nil {
		out = io.MultiWriter(os.Stdout, logFile)
	}
	cmd.Stdout, cmd.Stderr = out, out

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		logLine(logFile, "unable to start worker: "+err.Error())
		return 1
	}
	return 0
}

func openLog(path string) *os.File {
	if info, err := os.Stat(path); err == nil && info.Size() > maxLogSize {
		_ = os.Rename(path, path+".old")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil
	}
	return f
}

func logLine(f *os.File, msg string) {
	line := fmt.Sprintf(`{"level":"warn","message":%q,"source":"supervisor","timestamp":%q}`+"\n", msg, time.Now().Format(time.RFC3339))
	fmt.Print(line)
	if f != nil {
		_, _ = f.WriteString(line)
	}
}

func stopEvent(create bool) (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(stopEventName)
	if err != nil {
		return 0, err
	}
	if !create {
		return windows.OpenEvent(windows.EVENT_MODIFY_STATE, false, name)
	}
	sd, err := windows.SecurityDescriptorFromString(stopEventSDDL)
	if err != nil {
		return 0, err
	}
	sa := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	return windows.CreateEvent(&sa, 1, 0, name)
}

// stopRequested is closed when "OpenLinkHub.exe stop" signals the stop event.
func stopRequested() <-chan struct{} {
	done := make(chan struct{})
	event, err := stopEvent(true)
	if err != nil {
		fmt.Println("Unable to create stop event:", err)
		return done
	}
	_ = windows.ResetEvent(event)
	go func() {
		if _, err := windows.WaitForSingleObject(event, windows.INFINITE); err == nil {
			close(done)
		}
	}()
	return done
}

func requestStop() error {
	event, err := stopEvent(false)
	if err != nil {
		return errors.New("OpenLinkHub is not running")
	}
	defer windows.CloseHandle(event)
	return windows.SetEvent(event)
}
