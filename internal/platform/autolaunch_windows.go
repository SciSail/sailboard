//go:build windows

package platform

import (
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

// SetAutoLaunchDirect and AutoLaunchEnabledDirect are standalone entry points to the same
// registry logic the Controller's SetAutoLaunch/AutoLaunchEnabled use, for callers (the settings
// window process) that only need this one capability and shouldn't have to spin up a full
// Controller (hidden message window, tray, hotkey machinery) just to reach it.
func SetAutoLaunchDirect(appName string, enabled bool) error { return setAutoLaunch(appName, enabled) }
func AutoLaunchEnabledDirect(appName string) (bool, error)   { return autoLaunchEnabled(appName) }

func setAutoLaunch(appName string, enabled bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	if !enabled {
		if err := key.DeleteValue(appName); err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	return key.SetStringValue(appName, `"`+exePath+`"`)
}

// autoLaunchEnabled reports whether SailBoard will actually launch at the next login — not just
// whether some Run-key value happens to exist under this name. A value surviving after the exe it
// points at was deleted or moved (no uninstaller ran, or the app folder was replaced by hand)
// would never launch anything; that's reported as disabled here rather than as a stale false
// positive, since "did we once write a registry string" and "will this really run" are different
// questions and callers (the settings UI) only care about the second one.
func autoLaunchEnabled(appName string) (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, err
	}
	defer key.Close()

	value, _, err := key.GetStringValue(appName)
	if err == registry.ErrNotExist {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	path := strings.Trim(value, `"`)
	if _, statErr := os.Stat(path); statErr != nil {
		return false, nil
	}
	return true, nil
}
