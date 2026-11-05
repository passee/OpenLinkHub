// Package hotreload reloads the web UI while it is being edited: changed templates are parsed
// again without a restart, and open pages reload themselves when web/ or static/ changes.
// It is active only while a file named "hotreload" exists in the config directory.
package hotreload

import (
	"OpenLinkHub/src/config"
	"OpenLinkHub/src/logger"
	"OpenLinkHub/src/templates"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const (
	pollInterval = 400 * time.Millisecond
	longPoll     = 25 * time.Second
)

var (
	mutex   sync.Mutex
	version int
	changed = make(chan struct{})
)

func enabled() bool {
	_, err := os.Stat(filepath.Join(config.GetConfig().ConfigPath, "hotreload"))
	return err == nil
}

// Init starts watching the UI files when hot reload is enabled.
func Init() {
	if !enabled() {
		return
	}
	logger.Log(logger.Fields{}).Info("Hot reload enabled for web/ and static/")
	go watch()
}

func fingerprint(dir string) int64 {
	var sum int64
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		if info, err := entry.Info(); err == nil {
			sum += info.ModTime().UnixNano() ^ info.Size()
		}
		return nil
	})
	return sum
}

func watch() {
	root := config.GetConfig().ConfigPath
	web, static := filepath.Join(root, "web"), filepath.Join(root, "static")
	lastWeb, lastStatic := fingerprint(web), fingerprint(static)
	for range time.Tick(pollInterval) {
		currentWeb, currentStatic := fingerprint(web), fingerprint(static)
		if currentWeb == lastWeb && currentStatic == lastStatic {
			continue
		}
		if currentWeb != lastWeb {
			if err := templates.Reload(); err != nil {
				logger.Log(logger.Fields{"error": err}).Warn("Template has an error; keeping the previous version")
				lastWeb = currentWeb
				continue
			}
		}
		lastWeb, lastStatic = currentWeb, currentStatic
		mutex.Lock()
		version++
		close(changed)
		changed = make(chan struct{})
		mutex.Unlock()
	}
}

// Handler answers the page script: it returns the current version, or waits until the version
// passed in "since" is outdated. It returns 404 while hot reload is disabled.
func Handler(w http.ResponseWriter, r *http.Request) {
	if !enabled() {
		http.NotFound(w, r)
		return
	}
	since, err := strconv.Atoi(r.URL.Query().Get("since"))
	mutex.Lock()
	current, wait := version, changed
	mutex.Unlock()
	if err == nil && since == current {
		select {
		case <-wait:
		case <-time.After(longPoll):
		case <-r.Context().Done():
			return
		}
		mutex.Lock()
		current = version
		mutex.Unlock()
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]int{"version": current})
}
