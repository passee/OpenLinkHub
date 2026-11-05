package server

import (
	"OpenLinkHub/src/audio"
	"OpenLinkHub/src/common"
	"OpenLinkHub/src/devices"
	"OpenLinkHub/src/hotreload"
	"OpenLinkHub/src/icuebindings"
	"OpenLinkHub/src/icuepresets"
	"OpenLinkHub/src/macro"
	"OpenLinkHub/src/rgb"
	"OpenLinkHub/src/shortcuts"
	"encoding/json"
	"net/http"
	"reflect"
)

// Routes for features added on top of upstream live here, so server.go only needs one hook.
func registerExtensionRoutes(r *http.ServeMux) {
	handleFunc(r, "/api/mouse/globalDpi", http.MethodPost, changeGlobalDpi)
	handleFunc(r, "/api/headset/micVolume", http.MethodPost, changeMicVolume)
	handleFunc(r, "/api/headset/eqPresets", http.MethodGet, getEqPresets)
	handleFunc(r, "/api/headset/eqPreset", http.MethodPost, applyEqPreset)
	handleFunc(r, "/api/headset/eqClippingProtection", http.MethodPost, changeEqClippingProtection)
	handleFunc(r, "/api/keyboard/dial/options", http.MethodPost, changeDialOptions)
	handleFunc(r, "/api/import/icueBindings", http.MethodPost, importIcueBindings)
	handleFunc(r, "/api/mouse/sniperColor", http.MethodPost, changeSniperColor)
	handleFunc(r, "/api/dev/reload", http.MethodGet, hotreload.Handler)
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

func getEqPresets(w http.ResponseWriter, _ *http.Request) {
	resp := &Response{Code: http.StatusOK, Status: 1, Data: icuepresets.Load()}
	resp.Send(w)
}

func applyEqPreset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceId string `json:"deviceId"`
		Name     string `json:"name"`
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
	preset, ok := icuepresets.Find(req.Name)
	if !ok {
		resp.Message = "Preset not found"
		return
	}
	values := make(map[int]float64, len(preset.Bands))
	for i, gain := range preset.Bands {
		values[i+1] = gain
	}
	results := devices.CallDeviceMethod(req.DeviceId, "UpdateEqualizer", values)
	// 2 means the bands already had these values.
	if len(results) == 0 || (results[0].Uint() != 1 && results[0].Uint() != 2) {
		resp.Message = "Unable to apply preset"
		return
	}
	resp.Status = 1
	resp.Message = "Preset applied: " + preset.Name
}

func changeEqClippingProtection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	resp := &Response{Code: http.StatusOK}
	defer resp.Send(w)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		resp.Message = "Invalid request"
		return
	}
	if err := audio.SetClippingProtection(req.Enabled); err != nil {
		resp.Message = err.Error()
		return
	}
	resp.Status = 1
	if req.Enabled {
		resp.Message = "EQ lowers the volume so boosted bands never distort"
	} else {
		resp.Message = "EQ keeps the full volume, like iCUE"
	}
}

func changeDialOptions(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceId   string `json:"deviceId"`
		Brightness *int   `json:"brightness"`
		SyncColor  *bool  `json:"syncColor"`
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
	var results []reflect.Value
	switch {
	case req.Brightness != nil:
		results = devices.CallDeviceMethod(req.DeviceId, "SetKeyboardBrightness", *req.Brightness)
	case req.SyncColor != nil:
		results = devices.CallDeviceMethod(req.DeviceId, "SetDialSyncColor", *req.SyncColor)
	}
	if len(results) == 0 || results[0].Uint() != 1 {
		resp.Message = "Unable to change control dial"
		return
	}
	resp.Status = 1
	resp.Message = "Control dial updated"
}

func importIcueBindings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Profile string `json:"profile"`
	}
	resp := &Response{Code: http.StatusOK}
	defer resp.Send(w)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Profile == "" {
		resp.Message = "Invalid request"
		return
	}
	results, err := icuebindings.Import(req.Profile)
	if err != nil {
		resp.Message = err.Error()
		return
	}
	resp.Status = 1
	resp.Data = results
}

func changeSniperColor(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceId string  `json:"deviceId"`
		Red      float64 `json:"red"`
		Green    float64 `json:"green"`
		Blue     float64 `json:"blue"`
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
	results := devices.CallDeviceMethod(req.DeviceId, "SetSniperColor", rgb.Color{Red: req.Red, Green: req.Green, Blue: req.Blue})
	if len(results) == 0 || results[0].Uint() != 1 {
		resp.Message = "Unable to change sniper colour"
		return
	}
	resp.Status = 1
	resp.Message = "Sniper DPI colour saved"
}
