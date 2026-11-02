package appmonitor

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// runningExecutables returns every name a running process can be matched by.
func runningExecutables() ([]string, error) {
	processes, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(processes)*3)
	for _, process := range processes {
		if !process.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(process.Name()); err != nil {
			continue
		}

		path := filepath.Join("/proc", process.Name())

		if executable, err := os.Readlink(filepath.Join(path, "exe")); err == nil {
			names = append(names, strings.TrimSuffix(executable, " (deleted)"))
		}

		// Wine/Proton can expose the Windows executable as argv[0].
		if cmdline, err := os.ReadFile(filepath.Join(path, "cmdline")); err == nil {
			executable, _, _ := strings.Cut(string(cmdline), "\x00")
			if executable != "" {
				names = append(names, executable)
			}
		}

		if comm, err := os.ReadFile(filepath.Join(path, "comm")); err == nil {
			names = append(names, strings.TrimSpace(string(comm)))
		}
	}
	return names, nil
}
