package bluetooth

import (
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

// Manager coordinates Bluetooth device discovery and connections.
type Manager struct {
	mu            sync.RWMutex
	cachedDevices []*Device
	lastScan      time.Time
	scanMu        sync.Mutex
}

// NewManager creates a new Bluetooth Manager instance.
func NewManager() *Manager {
	return &Manager{}
}

// GetDevices returns the scanned Bluetooth devices, refreshing the list.
func (m *Manager) GetDevices() ([]*Device, error) {
	return m.ScanDevices()
}

// GetCachedDevices returns a copy of current cached devices.
func (m *Manager) GetCachedDevices() []*Device {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]*Device, len(m.cachedDevices))
	copy(res, m.cachedDevices)
	return res
}

// GetDevicesQuick returns the device list quickly.
// If a fresh scan does not finish within timeout, it falls back to cached devices
// so the UI never freezes or blocks.
func (m *Manager) GetDevicesQuick(timeout time.Duration) []*Device {
	m.mu.RLock()
	cached := m.cachedDevices
	age := time.Since(m.lastScan)
	m.mu.RUnlock()

	// If cache is fresh (less than 2 seconds old) and non-empty, use it immediately
	if len(cached) > 0 && age < 2*time.Second {
		return cached
	}

	type scanResult struct {
		devices []*Device
		err     error
	}

	done := make(chan scanResult, 1)
	go func() {
		devs, err := m.ScanDevices()
		done <- scanResult{devices: devs, err: err}
	}()

	select {
	case res := <-done:
		if res.err == nil && len(res.devices) > 0 {
			return res.devices
		}
		if len(cached) > 0 {
			return cached
		}
		return res.devices
	case <-time.After(timeout):
		// Scan took longer than timeout (e.g. driver waking from sleep);
		// return whatever is in cache to prevent UI from freezing
		return cached
	}
}

// ScanDevices performs an active scan of paired Bluetooth devices and Core Audio endpoints.
func (m *Manager) ScanDevices() ([]*Device, error) {
	m.scanMu.Lock()
	defer m.scanMu.Unlock()

	// Check if another scan just completed in the last 500ms
	m.mu.RLock()
	if len(m.cachedDevices) > 0 && time.Since(m.lastScan) < 500*time.Millisecond {
		res := m.cachedDevices
		m.mu.RUnlock()
		return res, nil
	}
	m.mu.RUnlock()

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
			RawInfo:     p.RawInfo,
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

	m.mu.Lock()
	m.cachedDevices = result
	m.lastScan = time.Now()
	m.mu.Unlock()

	return result, nil
}

// Connect attempts to connect the Bluetooth device.
func (m *Manager) Connect(dev *Device) error {
	if dev.IsAudio && len(dev.AudioEndpointDeviceIDs) > 0 {
		return SetAudioEndpointConnection(dev.AudioEndpointDeviceIDs, true)
	}

	// Fallback to enabling services for non-audio devices
	if len(dev.Services) == 0 {
		dev.Services = EnumerateDeviceServices(&dev.RawInfo)
	}
	if len(dev.Services) > 0 {
		return SetServicesState(&dev.RawInfo, dev.Services, true)
	}

	return nil
}

// Disconnect attempts to disconnect the Bluetooth device.
func (m *Manager) Disconnect(dev *Device) error {
	if dev.IsAudio && len(dev.AudioEndpointDeviceIDs) > 0 {
		return SetAudioEndpointConnection(dev.AudioEndpointDeviceIDs, false)
	}

	// Fallback to disabling services for non-audio devices
	if len(dev.Services) == 0 {
		dev.Services = EnumerateDeviceServices(&dev.RawInfo)
	}
	if len(dev.Services) > 0 {
		return SetServicesState(&dev.RawInfo, dev.Services, false)
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
