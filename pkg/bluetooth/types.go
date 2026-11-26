package bluetooth

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// Device represents a paired Bluetooth device on Windows.
type Device struct {
	Name                   string
	Address                uint64
	AddressStr             string
	Connected              bool
	IsAudio                bool
	AudioEndpointDeviceIDs []string
	ContainerID            windows.GUID
	RawInfo                BLUETOOTH_DEVICE_INFO
	Services               []windows.GUID
}

func (d *Device) String() string {
	status := "Disconnected"
	if d.Connected {
		status = "Connected"
	}
	devType := "Generic"
	if d.IsAudio {
		devType = "Audio"
	}
	return fmt.Sprintf("%s [%s] (%s, %s)", d.Name, d.AddressStr, devType, status)
}
