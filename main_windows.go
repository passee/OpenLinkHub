package main

import (
	"OpenLinkHub/src/controller"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

func waitForExit() {
	terminateSignals := make(chan os.Signal, 1)
	signal.Notify(terminateSignals, os.Interrupt, syscall.SIGTERM)
	<-terminateSignals
	controller.Stop() // Back to hardware mode
	os.Exit(0)
}

func main() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Println("Unable to locate executable:", err)
		os.Exit(1)
	}
	dir := filepath.Dir(exe)

	if len(os.Args) > 1 {
		if err = runCommand(os.Args[1], exe); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		return
	}

	// Config, database and web assets are resolved relative to the working directory,
	// which is System32 for a service.
	if err = os.Chdir(dir); err != nil {
		fmt.Println("Unable to change working directory:", err)
		os.Exit(1)
	}

	// Lighting and fan updates are short periodic bursts; above-normal scheduling keeps them
	// on time while a game saturates the CPU, which is what made iCUE's lighting stall.
	_ = windows.SetPriorityClass(windows.CurrentProcess(), windows.HIGH_PRIORITY_CLASS)

	isService, err := svc.IsWindowsService()
	if err != nil {
		fmt.Println("Unable to determine session type:", err)
		os.Exit(1)
	}
	if isService {
		runService(dir)
		return
	}

	go waitForExit()
	controller.Start()
}

func runCommand(command, exe string) error {
	switch command {
	case "install":
		return installService(exe)
	case "uninstall":
		return uninstallService()
	case "start":
		return startService()
	case "stop":
		return stopService()
	}
	return fmt.Errorf("unknown command %q (use install, uninstall, start or stop)", command)
}
