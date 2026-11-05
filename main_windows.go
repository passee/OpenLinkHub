package main

import (
	"OpenLinkHub/src/controller"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// OpenLinkHub.exe            supervises a worker process and restarts it when it fails
// OpenLinkHub.exe run        runs the controller in this process (the worker)
// OpenLinkHub.exe stop       asks a running instance to return devices to hardware mode and exit
// OpenLinkHub.exe install    starts OpenLinkHub at logon with administrator rights
// OpenLinkHub.exe uninstall  removes the logon task
func main() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Println("Unable to locate executable:", err)
		os.Exit(1)
	}

	command := ""
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "":
		// The CPU sensor driver and pausing iCUE's service both need administrator rights.
		if !isElevated() {
			exitOnError(relaunchElevated(exe))
			return
		}
		if !claimInstance(supervisorLockName) {
			fmt.Println("OpenLinkHub is already running")
			return
		}
		resume := pauseConflictingServices()
		code := supervise(exe)
		resume()
		os.Exit(code)
	case "run":
		if !claimInstance(workerLockName) {
			fmt.Println("OpenLinkHub is already running")
			os.Exit(exitAlreadyRunning)
		}
		runWorker(filepath.Dir(exe))
	case "stop":
		exitOnError(requestStop())
	case "install":
		exitOnError(installTask(exe))
	case "uninstall":
		exitOnError(uninstallTask())
	default:
		fmt.Printf("unknown command %q (use run, stop, install or uninstall)\n", command)
		os.Exit(2)
	}
}

func exitOnError(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

const stopTimeout = 15 * time.Second

func runWorker(dir string) {
	// Config, database and web assets are resolved relative to the working directory.
	if err := os.Chdir(dir); err != nil {
		fmt.Println("Unable to change working directory:", err)
		os.Exit(1)
	}

	// Lighting and fan updates are short periodic bursts; above-normal scheduling keeps them
	// on time while a game saturates the CPU, which is what made iCUE's lighting stall.
	_ = windows.SetPriorityClass(windows.CurrentProcess(), windows.HIGH_PRIORITY_CLASS)

	stopped := make(chan struct{})
	go func() {
		terminate := make(chan os.Signal, 1)
		signal.Notify(terminate, os.Interrupt, syscall.SIGTERM)
		select {
		case <-terminate:
		case <-stopRequested():
		}
		done := make(chan struct{})
		go func() {
			controller.Stop() // Back to hardware mode
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(stopTimeout):
			// A device stuck in its stop routine must not keep the old worker alive, or the next
			// start finds the instance lock taken.
			fmt.Println("Devices did not stop within", stopTimeout, "- exiting anyway")
		}
		close(stopped)
		os.Exit(0)
	}()

	controller.Start()
	<-stopped
}
