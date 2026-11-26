package bluetooth

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sys/windows"
)

// Manager coordinates Bluetooth device discovery and connections.
type Manager struct {
	mu            sync.RWMutex
	cachedDevices []*Device
	isScanning    int32
	lastScan      time.Time
}

// NewManager creates a new Bluetooth Manager instance and begins initial device scan in background.
func NewManager() *Manager {
	m := &Manager{}
	go m.RefreshDevices()

	// Periodic background refresh to keep connection state accurate
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		for range ticker.C {
			m.RefreshDevices()
		}
	}()

	return m
}

func (m *Manager) scanDevices() ([]*Device, error) {
	pairedList, err := EnumeratePairedDevices()
	if err != nil {
		return nil, err
	}

	audioEndpoints, _ := EnumerateAudioEndpoints()

	deviceMap := make(map[string]*Device)

	for _, p := range pairedList {
		deviceMap[p.AddressStr] = &Device{
			Name:       p.Name,
			Address:    p.Address,
			AddressStr: p.AddressStr,
			Connected:  p.Connected,
			IsAudio:    false,
			Services:   p.Services,
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

func (m *Manager) cloneDevicesLocked() []*Device {
	res := make([]*Device, len(m.cachedDevices))
	for i, d := range m.cachedDevices {
		copyD := *d
		res[i] = &copyD
	}
	return res
}

// RefreshDevices initiates a scan and updates the cache.
func (m *Manager) RefreshDevices() ([]*Device, error) {
	if !atomic.CompareAndSwapInt32(&m.isScanning, 0, 1) {
		m.mu.RLock()
		defer m.mu.RUnlock()
		return m.cloneDevicesLocked(), nil
	}
	defer atomic.StoreInt32(&m.isScanning, 0)

	devices, err := m.scanDevices()
	if err == nil {
		m.mu.Lock()
		m.cachedDevices = devices
		m.lastScan = time.Now()
		m.mu.Unlock()
	}
	return devices, err
}

// GetDevices returns cached devices immediately to prevent UI thread freezing.
// If cache is older than 5 seconds, it triggers a background refresh.
// If cache is empty and an initial scan is underway, it waits briefly for it to complete.
func (m *Manager) GetDevices() ([]*Device, error) {
	m.mu.RLock()
	hasCache := len(m.cachedDevices) > 0 || !m.lastScan.IsZero()
	if hasCache {
		res := m.cloneDevicesLocked()
		elapsed := time.Since(m.lastScan)
		m.mu.RUnlock()

		if elapsed > 5*time.Second {
			go m.RefreshDevices()
		}
		return res, nil
	}
	m.mu.RUnlock()

	// Cache is empty (e.g. cold start). If a background scan is in progress, wait up to 1 second
	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		m.mu.RLock()
		if len(m.cachedDevices) > 0 || !m.lastScan.IsZero() {
			res := m.cloneDevicesLocked()
			m.mu.RUnlock()
			return res, nil
		}
		m.mu.RUnlock()
	}

	return m.RefreshDevices()
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
		go m.RefreshDevices()
		return false, err
	}

	err := m.Connect(dev)
	if err == nil {
		// Wait a brief moment to allow driver to begin connection sequence
		time.Sleep(100 * time.Millisecond)
		dev.Connected = true
	}
	go m.RefreshDevices()
	return true, err
}
