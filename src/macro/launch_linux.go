package macro

import (
	"OpenLinkHub/src/logger"
	"os/exec"
)

// Launch opens target with the desktop's default handler.
func Launch(target string) {
	if target == "" {
		return
	}
	if err := exec.Command("xdg-open", target).Start(); err != nil {
		logger.Log(logger.Fields{"error": err, "target": target}).Error("Unable to launch program")
	}
}
