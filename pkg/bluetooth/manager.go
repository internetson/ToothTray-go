package bluetooth

import (
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

// Manager coordinates Bluetooth device discovery and connections.
type Manager struct{}

// NewManager creates a new Bluetooth Manager instance.
func NewManager() *Manager {
	return &Manager{}
}

// GetDevices scans and aggregates all paired Bluetooth devices with their audio endpoints.
func (m *Manager) GetDevices() ([]*Device, error) {
	pairedList, err := EnumeratePairedDevices()
	if err != nil {
		return nil, err
	}

	audioEndpoints, _ := EnumerateAudioEndpoints()

	deviceMap := make(map[string]*Device)

	for _, p := range pairedList {
		deviceMap[p.AddressStr] = &Device{
			Name:        p.Name,
			Address:     p.Address,
			AddressStr:  p.AddressStr,
			Connected:   p.Connected,
			IsAudio:     false,
			Services:    p.Services,
		}
	}

	// Associate audio endpoints with paired devices
	// Pass 1: Match by MAC address and populate ContainerID on the device
	unmatchedEndpoints := make([]AudioEndpointInfo, 0, len(audioEndpoints))
	for _, ep := range audioEndpoints {
		var matchedDev *Device
		if ep.MacAddress != "" {
			matchedDev = deviceMap[ep.MacAddress]
		}

		if matchedDev != nil {
			matchedDev.IsAudio = true
			matchedDev.AudioEndpointDeviceIDs = append(matchedDev.AudioEndpointDeviceIDs, ep.ConnectorDeviceID)
			if ep.Active {
				matchedDev.Connected = true
			}
			if ep.ContainerID != (windows.GUID{}) {
				matchedDev.ContainerID = ep.ContainerID
			}
		} else {
			unmatchedEndpoints = append(unmatchedEndpoints, ep)
		}
	}

	// Pass 2: Match remaining endpoints (e.g. HFP / bthhfenum) by ContainerID
	for _, ep := range unmatchedEndpoints {
		var matchedDev *Device
		if ep.ContainerID != (windows.GUID{}) {
			for _, d := range deviceMap {
				if d.ContainerID == ep.ContainerID {
					matchedDev = d
					break
				}
			}
		}

		if matchedDev != nil {
			matchedDev.IsAudio = true
			matchedDev.AudioEndpointDeviceIDs = append(matchedDev.AudioEndpointDeviceIDs, ep.ConnectorDeviceID)
			if ep.Active {
				matchedDev.Connected = true
			}
		}
	}

	var result []*Device
	for _, d := range deviceMap {
		result = append(result, d)
	}

	// Sort: connected first, then alphabetical
	sort.Slice(result, func(i, j int) bool {
		if result[i].Connected != result[j].Connected {
			return result[i].Connected
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	return result, nil
}

// Connect attempts to connect the Bluetooth device.
func (m *Manager) Connect(dev *Device) error {
	if dev.IsAudio && len(dev.AudioEndpointDeviceIDs) > 0 {
		return SetAudioEndpointConnection(dev.AudioEndpointDeviceIDs, true)
	}

	// Fallback to enabling services
	if len(dev.Services) > 0 {
		raw := BLUETOOTH_DEVICE_INFO{
			dwSize:  uint32(560),
			Address: dev.Address,
		}
		return SetServicesState(&raw, dev.Services, true)
	}

	return nil
}

// Disconnect attempts to disconnect the Bluetooth device.
func (m *Manager) Disconnect(dev *Device) error {
	if dev.IsAudio && len(dev.AudioEndpointDeviceIDs) > 0 {
		return SetAudioEndpointConnection(dev.AudioEndpointDeviceIDs, false)
	}

	// Fallback to disabling services
	if len(dev.Services) > 0 {
		raw := BLUETOOTH_DEVICE_INFO{
			dwSize:  uint32(560),
			Address: dev.Address,
		}
		return SetServicesState(&raw, dev.Services, false)
	}

	return nil
}

// ToggleConnection toggles between connect and disconnect.
func (m *Manager) ToggleConnection(dev *Device) (bool, error) {
	if dev.Connected {
		err := m.Disconnect(dev)
		if err == nil {
			dev.Connected = false
		}
		return false, err
	}

	err := m.Connect(dev)
	if err == nil {
		// Wait a brief moment to allow driver to begin connection sequence
		time.Sleep(100 * time.Millisecond)
		dev.Connected = true
	}
	return true, err
}
