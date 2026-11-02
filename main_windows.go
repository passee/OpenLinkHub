package main

import (
	"OpenLinkHub/src/controller"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

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
		os.Exit(supervise(exe))
	case "run":
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
		controller.Stop() // Back to hardware mode
		close(stopped)
		os.Exit(0)
	}()

	controller.Start()
	<-stopped
}
