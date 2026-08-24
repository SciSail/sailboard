//go:build darwin

package platform

// FixSettingsWindowDirect is a Windows-only fix (see settingswindow_windows.go's doc comment for
// what it corrects) with nothing to do on macOS: AppKit windows are sized/positioned in points,
// not physical pixels, so there's no logical/physical DPI mismatch to correct here, and macOS has
// no equivalent of Windows' Alt-key system-menu behaviour to swallow.
func FixSettingsWindowDirect(title string, logicalWidth, logicalHeight int) {}

// SetOnSystemMenuKeyDirect pairs with FixSettingsWindowDirect's system-menu-key swallow above —
// nothing to register on macOS since there's nothing being swallowed here.
func SetOnSystemMenuKeyDirect(callback func(resolvedSpec string)) {}

// SuspendHotkeyDirect/ResumeHotkeyDirect's real macOS implementation lives in ipc_darwin.go
// (alongside NotifySettingsChanged/RequestShowMainWindow, the same standalone-function/
// NSDistributedNotificationCenter pattern) rather than here as no-ops.

// HideDockIconDirect switches the settings window process's own activation policy to Accessory
// (no Dock icon, no Cmd+Tab entry) — see darwinHideDockIcon's doc comment for the mechanism and
// why this can't just be Info.plist's LSUIElement key. The settings window (main.go's
// runSettingsWindow) is launched as its own separate OS process (Wails v2 has no supported
// multi-window API for a single process), and Wails' AppDelegate.m unconditionally sets Regular
// policy for every process it runs in, main panel and settings window alike — so without this,
// every open of the settings window puts a second "SailBoard" icon in the Dock (and leaves it
// there if the window is later closed without quitting the process). Unlike ShowTray's call to the
// same underlying function, there's no menu-bar status item being created here to worry about
// ordering against — the settings window has none of its own — so this can just run directly,
// called from SettingsApp.startup before the window is shown.
func HideDockIconDirect() { darwinHideDockIcon() }
