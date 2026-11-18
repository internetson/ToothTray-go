package autostart

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	appName    = "ToothTray-go"
)

// IsEnabled returns true if ToothTray-go is set to start on Windows boot.
func IsEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	val, _, err := k.GetStringValue(appName)
	if err != nil {
		return false
	}

	exePath, err := os.Executable()
	if err != nil {
		return false
	}
	exePath = filepath.Clean(exePath)

	valClean := strings.Trim(val, "\"")
	valClean = filepath.Clean(valClean)

	return strings.EqualFold(valClean, exePath)
}

// SetEnabled enables or disables autostart on Windows boot.
func SetEnabled(enable bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if enable {
		exePath, err := os.Executable()
		if err != nil {
			return err
		}
		exePath = filepath.Clean(exePath)
		cmd := `"` + exePath + `"`
		return k.SetStringValue(appName, cmd)
	}

	err = k.DeleteValue(appName)
	if err != nil && err != registry.ErrNotExist {
		return err
	}
	return nil
}
