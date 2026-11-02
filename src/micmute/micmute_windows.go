package micmute

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	clsidMMDeviceEnumerator = windows.GUID{Data1: 0xBCDE0395, Data2: 0xE52F, Data3: 0x467C, Data4: [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	iidIMMDeviceEnumerator  = windows.GUID{Data1: 0xA95664D2, Data2: 0x9614, Data3: 0x4F35, Data4: [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	iidIAudioEndpointVolume = windows.GUID{Data1: 0x5CDF2C82, Data2: 0x841E, Data3: 0x4546, Data4: [8]byte{0x97, 0x22, 0x0C, 0xF7, 0x40, 0x78, 0x22, 0x9A}}
	pkeyDeviceFriendlyName  = propertyKey{fmtid: windows.GUID{Data1: 0xA45C254E, Data2: 0xDF1C, Data3: 0x4EFD, Data4: [8]byte{0x80, 0x20, 0x67, 0xD1, 0x46, 0xA8, 0x50, 0xE0}}, pid: 14}

	ole32            = windows.NewLazySystemDLL("ole32.dll")
	procCoCreate     = ole32.NewProc("CoCreateInstance")
	procPropVarClear = ole32.NewProc("PropVariantClear")
)

const (
	eCapture           = 1
	deviceStateActive  = 1
	clsctxInprocServer = 1
	stgmRead           = 0
	rpcEChangedMode    = 0x80010106

	// vtable slots
	enumEnumAudioEndpoints = 3
	collGetCount           = 3
	collItem               = 4
	devActivate            = 3
	devOpenPropertyStore   = 4
	storeGetValue          = 5
	volSetMasterScalar     = 7
	volGetMasterScalar     = 9
	volSetMute             = 14
	volGetMute             = 15
	release                = 2
)

type propertyKey struct {
	fmtid windows.GUID
	pid   uint32
}

type propVariant struct {
	vt  uint16
	_   [6]byte
	val *uint16
	_   uintptr
}

type comObject struct {
	vtbl *[32]uintptr
}

func (o *comObject) call(slot int, args ...uintptr) uintptr {
	all := append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)
	hr, _, _ := syscall.SyscallN(o.vtbl[slot], all...)
	return hr
}

func (o *comObject) release() {
	if o != nil {
		o.call(release)
	}
}

func check(hr uintptr, what string) error {
	if int32(hr) < 0 {
		return fmt.Errorf("%s failed: 0x%08x", what, uint32(hr))
	}
	return nil
}

// withEndpoint runs fn against the active capture endpoint whose name contains match.
func withEndpoint(match string, fn func(volume *comObject) error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED); err != nil {
		var errno syscall.Errno
		if !errors.As(err, &errno) || uint32(errno) != rpcEChangedMode {
			// S_FALSE (already initialised) is reported as success by CoInitializeEx.
			return fmt.Errorf("CoInitializeEx failed: %w", err)
		}
	} else {
		defer windows.CoUninitialize()
	}

	var enumerator *comObject
	hr, _, _ := procCoCreate.Call(
		uintptr(unsafe.Pointer(&clsidMMDeviceEnumerator)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIMMDeviceEnumerator)), uintptr(unsafe.Pointer(&enumerator)),
	)
	if err := check(hr, "CoCreateInstance(MMDeviceEnumerator)"); err != nil {
		return err
	}
	defer enumerator.release()

	var collection *comObject
	if err := check(enumerator.call(enumEnumAudioEndpoints, eCapture, deviceStateActive, uintptr(unsafe.Pointer(&collection))), "EnumAudioEndpoints"); err != nil {
		return err
	}
	defer collection.release()

	var count uint32
	if err := check(collection.call(collGetCount, uintptr(unsafe.Pointer(&count))), "GetCount"); err != nil {
		return err
	}

	match = strings.ToLower(match)
	for i := uint32(0); i < count; i++ {
		var device *comObject
		if collection.call(collItem, uintptr(i), uintptr(unsafe.Pointer(&device))) != 0 {
			continue
		}
		name := friendlyName(device)
		if !strings.Contains(strings.ToLower(name), match) {
			device.release()
			continue
		}

		var volume *comObject
		hr := device.call(devActivate, uintptr(unsafe.Pointer(&iidIAudioEndpointVolume)), clsctxInprocServer, 0, uintptr(unsafe.Pointer(&volume)))
		device.release()
		if err := check(hr, "Activate(IAudioEndpointVolume)"); err != nil {
			return err
		}
		defer volume.release()
		return fn(volume)
	}
	return fmt.Errorf("no active microphone matching %q", match)
}

func friendlyName(device *comObject) string {
	var store *comObject
	if device.call(devOpenPropertyStore, stgmRead, uintptr(unsafe.Pointer(&store))) != 0 {
		return ""
	}
	defer store.release()

	var value propVariant
	if store.call(storeGetValue, uintptr(unsafe.Pointer(&pkeyDeviceFriendlyName)), uintptr(unsafe.Pointer(&value))) != 0 {
		return ""
	}
	defer procPropVarClear.Call(uintptr(unsafe.Pointer(&value)))
	if value.val == nil {
		return ""
	}
	return windows.UTF16PtrToString(value.val)
}

// SetMuted mutes or unmutes the microphone whose endpoint name contains match.
func SetMuted(match string, muted bool) error {
	return withEndpoint(match, func(volume *comObject) error {
		var flag uintptr
		if muted {
			flag = 1
		}
		return check(volume.call(volSetMute, flag, 0), "SetMute")
	})
}

// Muted reports whether the matching microphone is muted.
func Muted(match string) (bool, error) {
	var muted int32
	err := withEndpoint(match, func(volume *comObject) error {
		return check(volume.call(volGetMute, uintptr(unsafe.Pointer(&muted))), "GetMute")
	})
	return muted != 0, err
}

// SetVolume sets the matching microphone level, 0-100.
func SetVolume(match string, percent int) error {
	if percent < 0 || percent > 100 {
		return errors.New("volume must be between 0 and 100")
	}
	level := float32(percent) / 100
	return withEndpoint(match, func(volume *comObject) error {
		return check(volume.call(volSetMasterScalar, uintptr(*(*uint32)(unsafe.Pointer(&level))), 0), "SetMasterVolumeLevelScalar")
	})
}

// Volume returns the matching microphone level, 0-100.
func Volume(match string) (int, error) {
	var level float32
	err := withEndpoint(match, func(volume *comObject) error {
		return check(volume.call(volGetMasterScalar, uintptr(unsafe.Pointer(&level))), "GetMasterVolumeLevelScalar")
	})
	return int(level*100 + 0.5), err
}
