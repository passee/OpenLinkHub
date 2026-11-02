package server

import (
	"OpenLinkHub/src/common"
	"OpenLinkHub/src/devices"
	"OpenLinkHub/src/macro"
	"OpenLinkHub/src/shortcuts"
	"encoding/json"
	"net/http"
)

// Routes for features added on top of upstream live here, so server.go only needs one hook.
func registerExtensionRoutes(r *http.ServeMux) {
	handleFunc(r, "/api/mouse/globalDpi", http.MethodPost, changeGlobalDpi)
	handleFunc(r, "/api/headset/micVolume", http.MethodPost, changeMicVolume)
	// ServeMux allows one registration per path, so both methods share a handler.
	r.HandleFunc("/api/shortcuts", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			getShortcuts(w, req)
		case http.MethodPost:
			saveShortcuts(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

func getShortcuts(w http.ResponseWriter, _ *http.Request) {
	type macroOption struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
	}
	options := make([]macroOption, 0)
	for id, m := range macro.GetProfiles() {
		options = append(options, macroOption{Id: id, Name: m.Name})
	}
	resp := &Response{Code: http.StatusOK, Status: 1, Data: map[string]interface{}{
		"shortcuts": shortcuts.Get(),
		"macros":    options,
	}}
	resp.Send(w)
}

func saveShortcuts(w http.ResponseWriter, r *http.Request) {
	resp := &Response{Code: http.StatusOK}
	defer resp.Send(w)

	var list []shortcuts.Shortcut
	if err := json.NewDecoder(r.Body).Decode(&list); err != nil {
		resp.Message = "Invalid request"
		return
	}
	if err := shortcuts.Save(list); err != nil {
		resp.Message = err.Error()
		return
	}
	resp.Status = 1
	resp.Message = "Shortcuts saved"
}

func changeGlobalDpi(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceId string `json:"deviceId"`
		Enabled  bool   `json:"enabled"`
	}
	resp := &Response{Code: http.StatusOK}
	defer resp.Send(w)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		resp.Message = "Invalid request"
		return
	}
	if !common.AlphanumericDashRegex.MatchString(req.DeviceId) || devices.GetDevice(req.DeviceId) == nil {
		resp.Message = "Device not found"
		return
	}
	results := devices.CallDeviceMethod(req.DeviceId, "SetGlobalDpi", req.Enabled)
	if len(results) == 0 || results[0].Uint() != 1 {
		resp.Message = "Unable to change global DPI"
		return
	}
	resp.Status = 1
	if req.Enabled {
		resp.Message = "DPI is now shared by all profiles"
	} else {
		resp.Message = "DPI is now stored per profile"
	}
}

func changeMicVolume(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceId string `json:"deviceId"`
		Volume   int    `json:"volume"`
	}
	resp := &Response{Code: http.StatusOK}
	defer resp.Send(w)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Volume < 0 || req.Volume > 100 {
		resp.Message = "Invalid request"
		return
	}
	if !common.AlphanumericDashRegex.MatchString(req.DeviceId) || devices.GetDevice(req.DeviceId) == nil {
		resp.Message = "Device not found"
		return
	}
	results := devices.CallDeviceMethod(req.DeviceId, "SetMicVolume", req.Volume)
	if len(results) == 0 || results[0].Uint() != 1 {
		resp.Message = "Unable to change microphone volume"
		return
	}
	resp.Status = 1
}
