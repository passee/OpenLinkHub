package shortcuts

import "OpenLinkHub/src/logger"

func startListener() {
	logger.Log(logger.Fields{}).Info("Global shortcuts are only available on Windows")
}

func listenerChanged() {}
