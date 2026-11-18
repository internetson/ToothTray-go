package bluetooth

import (
	"testing"
)

func TestGetDevices(t *testing.T) {
	mgr := NewManager()
	devices, err := mgr.GetDevices()
	if err != nil {
		t.Fatalf("GetDevices failed: %v", err)
	}

	t.Logf("Discovered %d Bluetooth devices:", len(devices))
	for i, d := range devices {
		t.Logf("[%d] %s (Audio: %v, Connected: %v, Endpoints: %d)",
			i, d.Name, d.IsAudio, d.Connected, len(d.AudioEndpointDeviceIDs))
	}

	if len(devices) == 0 {
		t.Log("No Bluetooth devices found (radio might be off or no paired devices)")
	}
}
