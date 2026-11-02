package server

import (
	"OpenLinkHub/src/common"
	"OpenLinkHub/src/devices"
	"encoding/json"
	"net/http"
)

// Routes for features added on top of upstream live here, so server.go only needs one hook.
func registerExtensionRoutes(r *http.ServeMux) {
	handleFunc(r, "/api/mouse/globalDpi", http.MethodPost, changeGlobalDpi)
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
