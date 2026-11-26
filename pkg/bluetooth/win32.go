package bluetooth

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	bthprops                               = windows.NewLazySystemDLL("bthprops.cpl")
	procBluetoothFindFirstDev              = bthprops.NewProc("BluetoothFindFirstDevice")
	procBluetoothFindNextDev               = bthprops.NewProc("BluetoothFindNextDevice")
	procBluetoothFindDevClose              = bthprops.NewProc("BluetoothFindDeviceClose")
	procBluetoothEnumerateInstalledServices = bthprops.NewProc("BluetoothEnumerateInstalledServices")
	procBluetoothSetServiceState           = bthprops.NewProc("BluetoothSetServiceState")
)

const (
	BLUETOOTH_SERVICE_DISABLE = 0
	BLUETOOTH_SERVICE_ENABLE  = 1
)

type BLUETOOTH_DEVICE_SEARCH_PARAMS struct {
	dwSize               uint32
	fReturnAuthenticated int32
	fReturnRemembered    int32
	fReturnUnknown       int32
	fReturnConnected     int32
	fIssueInquiry        int32
	cTimeoutMultiplier   uint8
	_                    [3]byte
	hRadio               windows.Handle
}

type SYSTEMTIME struct {
	wYear, wMonth, wDayOfWeek, wDay, wHour, wMinute, wSecond, wMilliseconds uint16
}

type BLUETOOTH_DEVICE_INFO struct {
	dwSize          uint32
	_               uint32
	Address         uint64
	ulClassofDevice uint32
	fConnected      int32
	fRemembered     int32
	fAuthenticated  int32
	stLastSeen      SYSTEMTIME
	stLastUsed      SYSTEMTIME
	szName          [248]uint16
}

// Win32DeviceInfo contains information from Win32 Bluetooth API about a paired device.
type Win32DeviceInfo struct {
	Name       string
	Address    uint64
	AddressStr string
	Connected  bool
	RawInfo    BLUETOOTH_DEVICE_INFO
	Services   []windows.GUID
}

// EnumeratePairedDevices returns all paired (remembered) Bluetooth devices on Windows.
func EnumeratePairedDevices() ([]Win32DeviceInfo, error) {
	var searchParams BLUETOOTH_DEVICE_SEARCH_PARAMS
	searchParams.dwSize = uint32(unsafe.Sizeof(searchParams))
	searchParams.fReturnAuthenticated = 1
	searchParams.fReturnRemembered = 1
	searchParams.fReturnUnknown = 0
	searchParams.fReturnConnected = 1
	searchParams.fIssueInquiry = 0
	searchParams.cTimeoutMultiplier = 0
	searchParams.hRadio = 0

	var devInfo BLUETOOTH_DEVICE_INFO
	devInfo.dwSize = uint32(unsafe.Sizeof(devInfo))

	hFind, _, _ := procBluetoothFindFirstDev.Call(
		uintptr(unsafe.Pointer(&searchParams)),
		uintptr(unsafe.Pointer(&devInfo)),
	)
	if hFind == 0 {
		return nil, nil // No devices or bluetooth radio disabled
	}
	defer procBluetoothFindDevClose.Call(hFind)

	var list []Win32DeviceInfo

	for {
		name := syscall.UTF16ToString(devInfo.szName[:])
		addrStr := fmt.Sprintf("%012X", devInfo.Address)

		list = append(list, Win32DeviceInfo{
			Name:       name,
			Address:    devInfo.Address,
			AddressStr: addrStr,
			Connected:  devInfo.fConnected != 0,
			RawInfo:    devInfo,
			Services:   nil, // Services queried on-demand when toggling non-audio devices
		})

		devInfo.dwSize = uint32(unsafe.Sizeof(devInfo))
		nextRet, _, _ := procBluetoothFindNextDev.Call(hFind, uintptr(unsafe.Pointer(&devInfo)))
		if nextRet == 0 {
			break
		}
	}

	return list, nil
}

// EnumerateDeviceServices queries the installed Bluetooth services for a specific device on demand.
func EnumerateDeviceServices(rawInfo *BLUETOOTH_DEVICE_INFO) []windows.GUID {
	var numServices uint32 = 32
	services := make([]windows.GUID, numServices)
	ret, _, _ := procBluetoothEnumerateInstalledServices.Call(
		0,
		uintptr(unsafe.Pointer(rawInfo)),
		uintptr(unsafe.Pointer(&numServices)),
		uintptr(unsafe.Pointer(&services[0])),
	)
	if ret == 0 && numServices > 0 {
		return services[:numServices]
	}
	return nil
}

// SetServicesState toggles installed Bluetooth services on/off for non-audio or fallback devices.
func SetServicesState(rawInfo *BLUETOOTH_DEVICE_INFO, services []windows.GUID, enable bool) error {
	state := uint32(BLUETOOTH_SERVICE_DISABLE)
	if enable {
		state = uint32(BLUETOOTH_SERVICE_ENABLE)
	}

	var lastErr error
	for _, svc := range services {
		ret, _, _ := procBluetoothSetServiceState.Call(
			0,
			uintptr(unsafe.Pointer(rawInfo)),
			uintptr(unsafe.Pointer(&svc)),
			uintptr(state),
		)
		if ret != 0 && ret != 0x80070057 { // ignore invalid arg or already enabled
			lastErr = fmt.Errorf("BluetoothSetServiceState failed: 0x%x", ret)
		}
	}
	return lastErr
}
