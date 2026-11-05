package templates

import (
	"OpenLinkHub/src/common"
	"OpenLinkHub/src/config"
	"html/template"
	"os"
	"path/filepath"
)

// Reload parses the web templates again and swaps them in only when they all parse, so an
// edit with a syntax error keeps the previous pages instead of stopping the server.
func Reload() error {
	dir := filepath.Join(config.GetConfig().ConfigPath, "web")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var files []string
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if common.IsValidExtension(path, ".html") {
			files = append(files, path)
		}
	}
	tpl, err := template.ParseFiles(files...)
	if err != nil {
		return err
	}
	templates = tpl
	return nil
}
