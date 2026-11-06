package server

import (
	"OpenLinkHub/src/audio"
	"OpenLinkHub/src/common"
	"OpenLinkHub/src/devices"
	"OpenLinkHub/src/devices/hs80rgbW"
	"OpenLinkHub/src/hotreload"
	"OpenLinkHub/src/icuebindings"
	"OpenLinkHub/src/icuepresets"
	"OpenLinkHub/src/inputmanager"
	"OpenLinkHub/src/keyvariants"
	"OpenLinkHub/src/macro"
	"OpenLinkHub/src/rgb"
	"OpenLinkHub/src/shortcuts"
	"encoding/json"
	"net/http"
	"reflect"
	"runtime/pprof"
	"strings"
	"time"
)

// Routes for features added on top of upstream live here, so server.go only needs one hook.
func registerExtensionRoutes(r *http.ServeMux) {
	handleFunc(r, "/api/mouse/globalDpi", http.MethodPost, changeGlobalDpi)
	handleFunc(r, "/api/headset/micVolume", http.MethodPost, changeMicVolume)
	handleFunc(r, "/api/headset/micBoost", http.MethodPost, changeMicBoost)
	handleFunc(r, "/api/headset/voicePrompts", http.MethodPost, changeVoicePrompts)
	handleFunc(r, "/api/headset/micLed", http.MethodPost, changeMicLed)
	handleFunc(r, "/api/headset/eqPresets", http.MethodGet, getEqPresets)
	handleFunc(r, "/api/headset/eqPreset", http.MethodPost, applyEqPreset)
	handleFunc(r, "/api/headset/eqClippingProtection", http.MethodPost, changeEqClippingProtection)
	handleFunc(r, "/api/keyboard/dial/options", http.MethodPost, changeDialOptions)
	handleFunc(r, "/api/import/icueBindings", http.MethodPost, importIcueBindings)
	handleFunc(r, "/api/mouse/sniperColor", http.MethodPost, changeSniperColor)
	handleFunc(r, "/api/dev/reload", http.MethodGet, hotreload.Handler)
	handleFunc(r, "/api/dev/goroutines", http.MethodGet, dumpGoroutines)
	handleFunc(r, "/api/devices/headsets", http.MethodGet, getHeadsets)
	r.HandleFunc("/api/keyVariants", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			getKeyVariants(w, req)
		case http.MethodPost:
			saveKeyVariants(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
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

func changeVoicePrompts(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceId string          `json:"deviceId"`
		Enabled  bool            `json:"enabled"`
		Prompts  map[string]bool `json:"prompts"`
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
	if req.Prompts != nil {
		results := devices.CallDeviceMethod(req.DeviceId, "SetVoicePromptSelection", req.Prompts)
		if len(results) == 0 || results[0].Uint() != 1 {
			resp.Message = "Unable to change voice prompts"
			return
		}
	}
	results := devices.CallDeviceMethod(req.DeviceId, "SetVoicePrompts", req.Enabled)
	if len(results) == 0 || results[0].Uint() != 1 {
		resp.Message = "Unable to change voice prompts"
		return
	}
	resp.Status = 1
	resp.Message = "Voice prompts updated"
}

func changeMicBoost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Boost float64 `json:"boost"`
	}
	resp := &Response{Code: http.StatusOK}
	defer resp.Send(w)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		resp.Message = "Invalid request"
		return
	}
	if err := audio.SetMicBoost(req.Boost); err != nil {
		resp.Message = err.Error()
		return
	}
	resp.Status = 1
	resp.Message = "Mic boost updated"
}

func changeMicLed(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceId string `json:"deviceId"`
		hs80rgbW.MicLed
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
	results := devices.CallDeviceMethod(req.DeviceId, "SetMicLed", req.MicLed)
	if len(results) == 0 || results[0].Uint() != 1 {
		resp.Message = "Unable to change the mic LED"
		return
	}
	resp.Status = 1
	resp.Message = "Mic LED updated"
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

func getKeyVariants(w http.ResponseWriter, r *http.Request) {
	resp := &Response{Code: http.StatusOK}
	defer resp.Send(w)

	type deviceOption struct {
		Serial  string `json:"serial"`
		Product string `json:"product"`
	}
	type actionOption struct {
		Id    uint16 `json:"id"`
		Name  string `json:"name"`
		Media bool   `json:"media"`
		Mouse bool   `json:"mouse"`
	}
	type macroOption struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
	}

	var options, mice, headsets []deviceOption
	for serial, d := range devices.GetDevices() {
		if d == nil {
			continue
		}
		instance := reflect.ValueOf(d.Instance)
		if instance.MethodByName("AssignableKeys").IsValid() {
			options = append(options, deviceOption{Serial: serial, Product: d.Product})
		}
		if instance.MethodByName("ToggleSniperMode").IsValid() {
			mice = append(mice, deviceOption{Serial: serial, Product: d.Product})
		}
		if instance.MethodByName("ToggleMicMute").IsValid() {
			headsets = append(headsets, deviceOption{Serial: serial, Product: d.Product})
		}
	}
	actions := make([]actionOption, 0)
	for id, a := range inputmanager.GetInputActions() {
		actions = append(actions, actionOption{Id: id, Name: a.Name, Media: a.Media, Mouse: a.Mouse})
	}
	macros := make([]macroOption, 0)
	for id, m := range macro.GetProfiles() {
		macros = append(macros, macroOption{Id: id, Name: m.Name})
	}

	data := map[string]interface{}{"devices": options, "mice": mice, "headsets": headsets, "actions": actions, "macros": macros}
	deviceId := r.URL.Query().Get("deviceId")
	if deviceId != "" {
		if !common.AlphanumericDashRegex.MatchString(deviceId) || devices.GetDevice(deviceId) == nil {
			resp.Message = "Device not found"
			return
		}
		if res := devices.CallDeviceMethod(deviceId, "AssignableKeys"); len(res) > 0 {
			data["keys"] = res[0].Interface()
		}
		data["variants"] = keyvariants.Get(deviceId)
		instance := reflect.ValueOf(devices.GetDevice(deviceId))
		if instance.Kind() == reflect.Ptr {
			if types := instance.Elem().FieldByName("KeyAssignmentTypes"); types.IsValid() {
				data["types"] = types.Interface()
			}
		}
		data["mouse"] = instance.MethodByName("ToggleSniperMode").IsValid()
	}
	resp.Status = 1
	resp.Data = data
}

func saveKeyVariants(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceId string `json:"deviceId"`
		Key      int    `json:"key"`
		keyvariants.KeyVariants
	}
	resp := &Response{Code: http.StatusOK}
	defer resp.Send(w)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		resp.Message = "Invalid request"
		return
	}
	if !common.AlphanumericDashRegex.MatchString(req.DeviceId) {
		resp.Message = "Device not found"
		return
	}
	if err := keyvariants.Set(req.DeviceId, req.Key, req.KeyVariants); err != nil {
		resp.Message = err.Error()
		return
	}
	resp.Status = 1
	resp.Message = "Key variants saved"
}

// getHeadsets lists headsets whose microphone a key can mute, as serial -> product.
func getHeadsets(w http.ResponseWriter, _ *http.Request) {
	headsets := map[string]string{}
	for serial, d := range devices.GetDevices() {
		if d != nil && reflect.ValueOf(d.Instance).MethodByName("ToggleMicMute").IsValid() {
			headsets[serial] = d.Product
		}
	}
	resp := &Response{Code: http.StatusOK, Status: 1, Data: headsets}
	resp.Send(w)
}

// pageWaitLimit caps how long a page waits for device initialization, so one stuck device
// cannot keep the UI from loading.
const pageWaitLimit = 20 * time.Second

// waitForDevices holds page requests until startup device initialization is done; a page
// rendered earlier lists only the devices that were ready by then.
func waitForDevices(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") && !strings.HasPrefix(r.URL.Path, "/static/") {
			select {
			case <-devices.Initialized():
			case <-time.After(pageWaitLimit):
			case <-r.Context().Done():
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// dumpGoroutines shows every goroutine's stack in developer mode, to find a call that hangs.
func dumpGoroutines(w http.ResponseWriter, r *http.Request) {
	if !hotreload.Enabled() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_ = pprof.Lookup("goroutine").WriteTo(w, 2)
}
