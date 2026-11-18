package bluetooth

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	ole32                = windows.NewLazySystemDLL("ole32.dll")
	procCoCreateInstance = ole32.NewProc("CoCreateInstance")
	procCoTaskMemFree    = ole32.NewProc("CoTaskMemFree")
	procPropVariantClear = ole32.NewProc("PropVariantClear")

	CLSID_MMDeviceEnumerator = windows.GUID{Data1: 0xBCDE0395, Data2: 0xE52F, Data3: 0x467C, Data4: [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	IID_IMMDeviceEnumerator  = windows.GUID{Data1: 0xA95664D2, Data2: 0x9614, Data3: 0x4F35, Data4: [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	IID_IDeviceTopology      = windows.GUID{Data1: 0x2A07407E, Data2: 0x6497, Data3: 0x4A18, Data4: [8]byte{0x97, 0x87, 0x32, 0xF7, 0x9B, 0xD0, 0xD9, 0x8F}}
	IID_IConnector           = windows.GUID{Data1: 0x9C2C4058, Data2: 0x23F5, Data3: 0x41DE, Data4: [8]byte{0x87, 0x7A, 0xDF, 0x3A, 0xF2, 0x36, 0xA0, 0x9E}}
	IID_IPart                = windows.GUID{Data1: 0xAE2DE0E4, Data2: 0x5BCA, Data3: 0x4F2D, Data4: [8]byte{0xAA, 0x46, 0x5D, 0x13, 0xF8, 0xFD, 0xB3, 0xA9}}
	IID_IKsControl           = windows.GUID{Data1: 0x28F54685, Data2: 0x06FD, Data3: 0x11D2, Data4: [8]byte{0xB2, 0x7A, 0x00, 0xA0, 0xC9, 0x22, 0x31, 0x96}}

	KSPROPSETID_BtAudio = windows.GUID{
		Data1: 0x7FA06C40,
		Data2: 0xB8F6,
		Data3: 0x4C7E,
		Data4: [8]byte{0x85, 0x56, 0xE8, 0xC3, 0x3A, 0x12, 0xE5, 0x4D},
	}

	// PKEY_Device_ContainerId = {8C7ED206-3F8A-4827-B3AB-AE9E1FAEFC6C}, 2
	PKEY_Device_ContainerId = PROPERTYKEY{
		fmtid: windows.GUID{Data1: 0x8c7ed206, Data2: 0x3f8a, Data3: 0x4827, Data4: [8]byte{0xb3, 0xab, 0xae, 0x9e, 0x1f, 0xae, 0xfc, 0x6c}},
		pid:   2,
	}

	// Prop {2BD67D8B-8BEB-48D5-87E0-6CDA3428040A} / 1 contains the Bluetooth MAC address
	PKEY_Bth_Address = PROPERTYKEY{
		fmtid: windows.GUID{Data1: 0x2bd67d8b, Data2: 0x8beb, Data3: 0x48d5, Data4: [8]byte{0x87, 0xe0, 0x6c, 0xda, 0x34, 0x28, 0x04, 0x0a}},
		pid:   1,
	}
)

const (
	CLSCTX_ALL                   = 23
	eRender                      = 0
	DEVICE_STATEMASK_ALL         = 0x0000000F
	DEVICE_STATE_ACTIVE          = 1
	STGM_READ                    = 0
	VT_LPWSTR                    = 31
	VT_CLSID                     = 72
	KSPROPERTY_TYPE_GET          = 0x00000001
	KSPROPERTY_ONESHOT_RECONNECT = 0
	KSPROPERTY_ONESHOT_DISCONNECT = 1
)

type PROPERTYKEY struct {
	fmtid windows.GUID
	pid   uint32
}

type PROPVARIANT struct {
	vt         uint16
	wReserved1 uint16
	wReserved2 uint16
	wReserved3 uint16
	val        uintptr
	_          uintptr
}

type KSPROPERTY struct {
	Set   windows.GUID
	Id    uint32
	Flags uint32
}

type AudioEndpointInfo struct {
	EndpointID        string
	ConnectorDeviceID string
	ContainerID       windows.GUID
	MacAddress        string
	Active            bool
}

func call(vtbl uintptr, index int, args ...uintptr) uintptr {
	ptr := *(*uintptr)(unsafe.Pointer(vtbl))
	fn := *(*uintptr)(unsafe.Pointer(ptr + uintptr(index)*unsafe.Sizeof(uintptr(0))))
	allArgs := append([]uintptr{vtbl}, args...)
	r1, _, _ := syscall.SyscallN(fn, allArgs...)
	return r1
}

func release(vtbl uintptr) {
	if vtbl != 0 {
		call(vtbl, 2)
	}
}

var macRegex = regexp.MustCompile(`(?i)&([0-9a-f]{12})_`)

// EnumerateAudioEndpoints scans all Windows Core Audio render endpoints for Bluetooth devices.
func EnumerateAudioEndpoints() ([]AudioEndpointInfo, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)
	defer windows.CoUninitialize()

	var pEnumerator uintptr
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&CLSID_MMDeviceEnumerator)),
		0,
		uintptr(CLSCTX_ALL),
		uintptr(unsafe.Pointer(&IID_IMMDeviceEnumerator)),
		uintptr(unsafe.Pointer(&pEnumerator)),
	)
	if hr != 0 {
		return nil, fmt.Errorf("CoCreateInstance MMDeviceEnumerator failed: 0x%x", hr)
	}
	defer release(pEnumerator)

	var pDevices uintptr
	r := call(pEnumerator, 3, uintptr(eRender), uintptr(DEVICE_STATEMASK_ALL), uintptr(unsafe.Pointer(&pDevices)))
	if r != 0 || pDevices == 0 {
		return nil, fmt.Errorf("EnumAudioEndpoints failed: 0x%x", r)
	}
	defer release(pDevices)

	var count uint32
	call(pDevices, 3, uintptr(unsafe.Pointer(&count)))

	var endpoints []AudioEndpointInfo

	for i := uint32(0); i < count; i++ {
		var pDevice uintptr
		call(pDevices, 4, uintptr(i), uintptr(unsafe.Pointer(&pDevice)))
		if pDevice == 0 {
			continue
		}

		var state uint32
		call(pDevice, 6, uintptr(unsafe.Pointer(&state)))

		var pDeviceId uintptr
		call(pDevice, 5, uintptr(unsafe.Pointer(&pDeviceId)))
		deviceIdStr := ""
		if pDeviceId != 0 {
			deviceIdStr = windows.UTF16PtrToString((*uint16)(unsafe.Pointer(pDeviceId)))
			procCoTaskMemFree.Call(pDeviceId)
		}

		var containerId windows.GUID
		var pPropStore uintptr
		call(pDevice, 4, uintptr(STGM_READ), uintptr(unsafe.Pointer(&pPropStore)))
		if pPropStore != 0 {
			var pv PROPVARIANT
			call(pPropStore, 5, uintptr(unsafe.Pointer(&PKEY_Device_ContainerId)), uintptr(unsafe.Pointer(&pv)))
			if pv.vt == VT_CLSID && pv.val != 0 {
				containerId = *(*windows.GUID)(unsafe.Pointer(pv.val))
			}
			procPropVariantClear.Call(uintptr(unsafe.Pointer(&pv)))
			release(pPropStore)
		}

		// Activate IDeviceTopology to find the connected KS filter
		var pTopology uintptr
		call(pDevice, 3, uintptr(unsafe.Pointer(&IID_IDeviceTopology)), uintptr(CLSCTX_ALL), 0, uintptr(unsafe.Pointer(&pTopology)))
		if pTopology != 0 {
			var connCount uint32
			call(pTopology, 3, uintptr(unsafe.Pointer(&connCount)))
			for c := uint32(0); c < connCount; c++ {
				var pConnector uintptr
				call(pTopology, 4, uintptr(c), uintptr(unsafe.Pointer(&pConnector)))
				if pConnector == 0 {
					continue
				}

				var pOtherConn uintptr
				call(pConnector, 8, uintptr(unsafe.Pointer(&pOtherConn))) // GetConnectedTo
				release(pConnector)
				if pOtherConn == 0 {
					continue
				}

				var pPart uintptr
				call(pOtherConn, 0, uintptr(unsafe.Pointer(&IID_IPart)), uintptr(unsafe.Pointer(&pPart)))
				release(pOtherConn)
				if pPart == 0 {
					continue
				}

				var pOtherTopology uintptr
				call(pPart, 12, uintptr(unsafe.Pointer(&pOtherTopology))) // GetTopologyObject
				release(pPart)
				if pOtherTopology == 0 {
					continue
				}

				var pOtherDeviceId uintptr
				call(pOtherTopology, 8, uintptr(unsafe.Pointer(&pOtherDeviceId))) // GetDeviceId
				release(pOtherTopology)
				if pOtherDeviceId != 0 {
					otherDevIdStr := windows.UTF16PtrToString((*uint16)(unsafe.Pointer(pOtherDeviceId)))
					procCoTaskMemFree.Call(pOtherDeviceId)

					lowerOther := strings.ToLower(otherDevIdStr)
					if strings.HasPrefix(lowerOther, "{2}.\\\\?\\bth") {
						mac := ""
						m := macRegex.FindStringSubmatch(otherDevIdStr)
						if len(m) > 1 {
							mac = strings.ToUpper(m[1])
						}

						// If MAC not in string, check other device properties
						var pOtherDevice uintptr
						call(pEnumerator, 5, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(otherDevIdStr))), uintptr(unsafe.Pointer(&pOtherDevice)))
						if pOtherDevice != 0 {
							var pOtherPropStore uintptr
							call(pOtherDevice, 4, uintptr(STGM_READ), uintptr(unsafe.Pointer(&pOtherPropStore)))
							if pOtherPropStore != 0 {
								if mac == "" {
									var pvBth PROPVARIANT
									call(pOtherPropStore, 5, uintptr(unsafe.Pointer(&PKEY_Bth_Address)), uintptr(unsafe.Pointer(&pvBth)))
									if pvBth.vt == VT_LPWSTR && pvBth.val != 0 {
										mac = strings.ToUpper(windows.UTF16PtrToString((*uint16)(unsafe.Pointer(pvBth.val))))
									}
									procPropVariantClear.Call(uintptr(unsafe.Pointer(&pvBth)))
								}
								if containerId == (windows.GUID{}) {
									var pvCont PROPVARIANT
									call(pOtherPropStore, 5, uintptr(unsafe.Pointer(&PKEY_Device_ContainerId)), uintptr(unsafe.Pointer(&pvCont)))
									if pvCont.vt == VT_CLSID && pvCont.val != 0 {
										containerId = *(*windows.GUID)(unsafe.Pointer(pvCont.val))
									}
									procPropVariantClear.Call(uintptr(unsafe.Pointer(&pvCont)))
								}
								release(pOtherPropStore)
							}
							release(pOtherDevice)
						}

						endpoints = append(endpoints, AudioEndpointInfo{
							EndpointID:        deviceIdStr,
							ConnectorDeviceID:  otherDevIdStr,
							ContainerID:       containerId,
							MacAddress:        mac,
							Active:            state == DEVICE_STATE_ACTIVE,
						})
					}
				}
			}
			release(pTopology)
		}
		release(pDevice)
	}

	return endpoints, nil
}

// SetAudioEndpointConnection triggers KSPROPERTY_ONESHOT_RECONNECT or KSPROPERTY_ONESHOT_DISCONNECT
// on all provided connector device IDs.
func SetAudioEndpointConnection(connectorDeviceIDs []string, connect bool) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)
	defer windows.CoUninitialize()

	var pEnumerator uintptr
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&CLSID_MMDeviceEnumerator)),
		0,
		uintptr(CLSCTX_ALL),
		uintptr(unsafe.Pointer(&IID_IMMDeviceEnumerator)),
		uintptr(unsafe.Pointer(&pEnumerator)),
	)
	if hr != 0 {
		return fmt.Errorf("CoCreateInstance MMDeviceEnumerator failed: 0x%x", hr)
	}
	defer release(pEnumerator)

	propID := uint32(KSPROPERTY_ONESHOT_RECONNECT)
	if !connect {
		propID = uint32(KSPROPERTY_ONESHOT_DISCONNECT)
	}

	ksProp := KSPROPERTY{
		Set:   KSPROPSETID_BtAudio,
		Id:    propID,
		Flags: KSPROPERTY_TYPE_GET,
	}

	var lastErr error
	for _, connID := range connectorDeviceIDs {
		var pOtherDevice uintptr
		hRes := call(pEnumerator, 5, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(connID))), uintptr(unsafe.Pointer(&pOtherDevice)))
		if hRes != 0 || pOtherDevice == 0 {
			lastErr = fmt.Errorf("GetDevice(%s) failed: 0x%x", connID, hRes)
			continue
		}

		var pKsControl uintptr
		hRes = call(pOtherDevice, 3, uintptr(unsafe.Pointer(&IID_IKsControl)), uintptr(CLSCTX_ALL), 0, uintptr(unsafe.Pointer(&pKsControl)))
		if hRes != 0 || pKsControl == 0 {
			release(pOtherDevice)
			lastErr = fmt.Errorf("Activate(IKsControl) failed: 0x%x", hRes)
			continue
		}

		var bytesReturned uint32
		hRes = call(pKsControl, 3, uintptr(unsafe.Pointer(&ksProp)), unsafe.Sizeof(ksProp), 0, 0, uintptr(unsafe.Pointer(&bytesReturned)))
		if hRes != 0 {
			lastErr = fmt.Errorf("KsProperty failed: 0x%x", hRes)
		}

		release(pKsControl)
		release(pOtherDevice)
	}

	return lastErr
}
