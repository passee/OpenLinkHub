package ironclawW

import (
	"OpenLinkHub/src/dispatcher"
	"sort"
)

// actionMicMute toggles a headset microphone; the assignment's DeviceId names the headset.
const actionMicMute = 40

// actionProfileLock pauses or resumes automatic profile switching for running games.
const actionProfileLock = 41

// dispatch reaches other devices, such as the headset for mic mute. There is one Ironclaw per
// receiver, so it lives at package level instead of in the upstream Device struct.
var dispatch dispatcher.DeviceDispatcher

// SetDispatcher is called by the device registry once the mouse is added.
func (d *Device) SetDispatcher(ds dispatcher.DeviceDispatcher) {
	dispatch = ds
}

// SniperToggleSupported tells the key assignment request that sniper buttons without press and
// hold toggle sniper mode here.
func (d *Device) SniperToggleSupported() bool {
	return true
}

// KeyName is one assignable button.
type KeyName struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
}

// AssignableKeys lists the buttons that can carry an assignment.
func (d *Device) AssignableKeys() []KeyName {
	keys := make([]KeyName, 0, len(d.KeyAssignment))
	for index, key := range d.KeyAssignment {
		keys = append(keys, KeyName{Index: index, Name: key.Name})
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Index < keys[j].Index })
	return keys
}

func (d *Device) pressSniper(hold bool) {
	if hold {
		d.sniperMode(true)
		return
	}
	d.ToggleSniperMode()
}

func toggleHeadsetMic(headset string) {
	if dispatch != nil && headset != "" {
		dispatch(headset, "ToggleMicMute")
	}
}
