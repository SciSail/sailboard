//go:build !windows && !darwin

package platform

import "fmt"

// New returns the controller for the current OS. On platforms without a native implementation at
// all this is the plain no-op stub; SailBoard still runs, just without global hotkeys, tray icon,
// auto-paste, correct panel positioning, etc. Windows has its own New in controller_windows.go,
// macOS in controller_darwin.go — see those files for what's implemented there.
func New(appName string) (Controller, error) {
	return stubController{}, nil
}

// OpenFolderDirect mirrors folder_windows.go/folder_darwin.go's standalone helper of the same
// name for platforms without one yet.
func OpenFolderDirect(path string) error {
	return fmt.Errorf("opening a folder is not implemented on this platform")
}

// SetAutoLaunchDirect, AutoLaunchEnabledDirect, and NotifySettingsChanged mirror the Windows/
// macOS standalone helpers of the same name for platforms without a native Controller yet.
func SetAutoLaunchDirect(appName string, enabled bool) error { return nil }
func AutoLaunchEnabledDirect(appName string) (bool, error)   { return false, nil }
func NotifySettingsChanged()                                 {}

// FixSettingsWindowDirect is the Windows-only settings window DPI/Alt-menu fix (see
// settingswindow_windows.go); nothing to do on a platform without a native Controller at all.
func FixSettingsWindowDirect(title string, logicalWidth, logicalHeight int) {}
func SetOnSystemMenuKeyDirect(callback func(resolvedSpec string))          {}
func SuspendHotkeyDirect()                                                 {}
func ResumeHotkeyDirect()                                                  {}

// HideDockIconDirect is the macOS-only Dock-icon-hiding fix for the standalone settings window
// process (see settingswindow_darwin.go); nothing to do on a platform without a Dock at all.
func HideDockIconDirect() {}

// AccessibilityTrustedDirect is a macOS-only concept (see foreground_darwin.go's doc comment) —
// always reports trusted on a platform with no such permission to check.
func AccessibilityTrustedDirect() bool { return true }

// AcquireSingleInstanceLock always reports success on platforms without a real implementation
// yet, rather than ever refusing to launch.
func AcquireSingleInstanceLock() (bool, error) { return true, nil }
func RequestShowMainWindow()                   {}
