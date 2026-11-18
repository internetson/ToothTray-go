package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config represents the application settings stored on disk.
type Config struct {
	NotificationsEnabled bool `json:"notifications_enabled"`

	path string
	mu   sync.RWMutex
}

// Default returns a Config with default options.
func Default() *Config {
	return &Config{
		NotificationsEnabled: true,
	}
}

// ConfigFilePath returns the absolute path to config.json located next to the executable.
func ConfigFilePath() string {
	exePath, err := os.Executable()
	if err != nil {
		return "config.json"
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = resolved
	}
	return filepath.Join(filepath.Dir(exePath), "config.json")
}

// Load loads the configuration from config.json next to the executable.
// If the file does not exist, it creates a default configuration.
// If reading fails, it falls back to defaults without erroring out.
func Load() *Config {
	path := ConfigFilePath()
	return LoadFromFile(path)
}

// LoadFromFile loads the configuration from a specific file path.
func LoadFromFile(path string) *Config {
	cfg := Default()
	cfg.path = path

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Save default configuration so the user has a template file
			_ = cfg.saveLocked()
		}
		return cfg
	}

	var diskConfig struct {
		NotificationsEnabled *bool `json:"notifications_enabled"`
	}

	if err := json.Unmarshal(data, &diskConfig); err == nil {
		if diskConfig.NotificationsEnabled != nil {
			cfg.NotificationsEnabled = *diskConfig.NotificationsEnabled
		}
	}

	return cfg
}

// IsNotificationsEnabled returns whether notifications are enabled.
func (c *Config) IsNotificationsEnabled() bool {
	if c == nil {
		return true
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.NotificationsEnabled
}

// SetNotificationsEnabled updates the notifications setting and saves to disk.
func (c *Config) SetNotificationsEnabled(enabled bool) error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.NotificationsEnabled = enabled
	return c.saveLocked()
}

// Save saves the current configuration to disk.
func (c *Config) Save() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.saveLocked()
}

func (c *Config) saveLocked() error {
	if c.path == "" {
		c.path = ConfigFilePath()
	}

	data, err := json.MarshalIndent(struct {
		NotificationsEnabled bool `json:"notifications_enabled"`
	}{
		NotificationsEnabled: c.NotificationsEnabled,
	}, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.path, data, 0644)
}
