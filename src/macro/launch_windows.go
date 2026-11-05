package macro

import (
	"OpenLinkHub/src/logger"
	"os/exec"
)

// Launch starts target through Explorer, so it runs with the user's normal rights rather than
// OpenLinkHub's elevated ones, and .bat files, documents and URLs open like a double click.
func Launch(target string) {
	if target == "" {
		return
	}
	if err := exec.Command("explorer.exe", target).Start(); err != nil {
		logger.Log(logger.Fields{"error": err, "target": target}).Error("Unable to launch program")
	}
}
