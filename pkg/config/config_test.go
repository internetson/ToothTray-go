package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoadAndSave(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// 1. Initial load should create default config with notifications enabled
	cfg := LoadFromFile(configPath)
	if !cfg.IsNotificationsEnabled() {
		t.Fatalf("expected notifications to be enabled by default")
	}

	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config.json to be created: %v", err)
	}

	// 2. Disable notifications and save
	if err := cfg.SetNotificationsEnabled(false); err != nil {
		t.Fatalf("SetNotificationsEnabled failed: %v", err)
	}
	if cfg.IsNotificationsEnabled() {
		t.Fatalf("expected notifications to be disabled")
	}

	// 3. Reload config from disk
	cfg2 := LoadFromFile(configPath)
	if cfg2.IsNotificationsEnabled() {
		t.Fatalf("expected reloaded config to have notifications disabled")
	}

	// 4. Re-enable notifications
	if err := cfg2.SetNotificationsEnabled(true); err != nil {
		t.Fatalf("SetNotificationsEnabled failed: %v", err)
	}
	if !cfg2.IsNotificationsEnabled() {
		t.Fatalf("expected notifications to be enabled again")
	}

	cfg3 := LoadFromFile(configPath)
	if !cfg3.IsNotificationsEnabled() {
		t.Fatalf("expected reloaded config to have notifications enabled")
	}
}
