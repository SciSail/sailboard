package main

import (
	"context"
	"errors"
	"path/filepath"

	"SailBoard/internal/platform"
	"SailBoard/internal/storage"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// SettingsApp is the Wails-bound API for the standalone settings window (see main.go's
// runSettingsWindow). It talks to the same SQLite database as the main process (safe: WAL mode,
// enabled in storage.Open, supports concurrent readers/writers across processes) rather than
// going through the main process at all — the two are only coupled by that shared file plus the
// one-shot NotifySettingsChanged nudge after a save, so the main process's hotkey/autostart/
// cleanup logic reloads promptly instead of waiting on its own next natural sync point.
type SettingsApp struct {
	ctx        context.Context
	repository *storage.Repository
	dataDir    string
}

func NewSettingsApp() *SettingsApp { return &SettingsApp{} }

func (s *SettingsApp) startup(ctx context.Context) {
	s.ctx = ctx
	// macOS-only Dock-icon fix (see platform.HideDockIconDirect's doc comment); a no-op on other
	// platforms. Runs before the window is shown for the same reason as FixSettingsWindowDirect
	// below, so the Dock icon never has a chance to flash in before disappearing.
	platform.HideDockIconDirect()
	// Windows-only DPI/Alt-key fix (see platform.FixSettingsWindowDirect's doc comment); a no-op
	// on other platforms. Runs before the window is shown (Wails calls OnStartup right after
	// creating the native window but before RunMainLoop's Show()), so there's no visible resize.
	platform.FixSettingsWindowDirect(settingsWindowTitle, settingsWindowWidth, settingsWindowHeight)
	// See platform.SetOnSystemMenuKeyDirect's doc comment: fires whenever the user's keypress was
	// one Windows reserves for the system menu (bare Alt/F10, or Alt+Space) — a combo that never
	// reaches the frontend as a normal keydown, so it has no other way to notice a capture attempt
	// happened, let alone resolve which combo it was. The event payload is the resolved "Mod+KEY"
	// spec when recognized (currently just Alt+Space), or "" otherwise. A no-op registration on
	// platforms without a real implementation.
	platform.SetOnSystemMenuKeyDirect(func(resolvedSpec string) { runtime.EventsEmit(ctx, "hotkey:reserved", resolvedSpec) })
	dataDir, err := appDataDir()
	if err != nil {
		runtime.LogErrorf(ctx, "settings window: create app data directory: %v", err)
		return
	}
	s.dataDir = dataDir
	repository, err := storage.Open(filepath.Join(dataDir, "sailboard.db"))
	if err != nil {
		runtime.LogErrorf(ctx, "settings window: open history database: %v", err)
		return
	}
	s.repository = repository
}

func (s *SettingsApp) shutdown(ctx context.Context) {
	// Safety net for a window closed mid-capture (e.g. Alt+F4, or the titlebar X) without ever
	// reaching EndShortcutCapture: unconditional and idempotent, since ResumeHotkeyDirect just
	// re-reads and re-applies the saved shortcut whether or not it was actually suspended.
	platform.ResumeHotkeyDirect()
	if s.repository != nil {
		_ = s.repository.Close()
	}
}

// BeginShortcutCapture/EndShortcutCapture bracket the settings UI's shortcut-capture flow (see
// Settings.tsx) — see platform.Controller.OnHotkeySuspendRequested's doc comment for why the
// *current* hotkey needs to stop firing while a new one is being tried out.
func (s *SettingsApp) BeginShortcutCapture() { platform.SuspendHotkeyDirect() }
func (s *SettingsApp) EndShortcutCapture()   { platform.ResumeHotkeyDirect() }

func (s *SettingsApp) ready() error {
	if s.repository == nil {
		return errors.New("settings window is still starting")
	}
	return nil
}

func (s *SettingsApp) GetSettings() (storage.Settings, error) {
	if err := s.ready(); err != nil {
		return storage.Settings{}, err
	}
	settings, err := s.repository.GetSettings(s.ctx)
	if err != nil {
		return settings, err
	}
	// The DB's launch_at_login only records the last choice made in this UI, which can silently
	// drift from what's actually going to happen at next login (the exe was moved/deleted since
	// without an uninstaller running, or the user flipped it directly via Windows' own Task
	// Manager/Settings startup toggle). Report the verified, live state instead so the checkbox
	// never lies — and since a subsequent Save persists whatever this returns, one open of the
	// settings window is enough to repair a stale DB value too.
	if enabled, err := platform.AutoLaunchEnabledDirect(appWindowTitle); err == nil {
		settings.LaunchAtLogin = enabled
	}
	return settings, nil
}

// SaveSettings persists settings, applies the launch-at-login registry change directly (a
// standalone registry write — see SetAutoLaunchDirect — needs no shared state with the main
// process), runs cleanup under the new retention/space limits, and finally pokes the main
// process so it reloads and re-registers the hotkey immediately rather than on its next show.
func (s *SettingsApp) SaveSettings(settings storage.Settings) error {
	if err := s.ready(); err != nil {
		return err
	}
	if err := s.repository.SaveSettings(s.ctx, settings); err != nil {
		return err
	}
	if err := s.repository.Cleanup(s.ctx, settings); err != nil {
		runtime.LogErrorf(s.ctx, "settings window: cleanup: %v", err)
	}
	if err := platform.SetAutoLaunchDirect(appWindowTitle, settings.LaunchAtLogin); err != nil {
		runtime.LogErrorf(s.ctx, "settings window: set launch-at-login: %v", err)
	}
	platform.NotifySettingsChanged()
	return nil
}

func (s *SettingsApp) ClearHistory(includeFavorites bool) error {
	if err := s.ready(); err != nil {
		return err
	}
	if err := s.repository.ClearHistory(s.ctx, includeFavorites); err != nil {
		return err
	}
	platform.NotifySettingsChanged() // prompts the main window to refresh its (now stale) list
	return nil
}

// CloseWindow lets the frontend close this window itself (e.g. after a successful save).
func (s *SettingsApp) CloseWindow() { runtime.Quit(s.ctx) }

// IsAccessibilityTrusted reports whether SailBoard currently has macOS Accessibility permission
// (always true on Windows/other platforms — see platform.AccessibilityTrustedDirect's doc comment).
// Called from Settings.tsx on mount so the "auto-paste needs Accessibility" notice — previously
// only shown reactively, inline in the main panel, after a failed paste — surfaces proactively
// here instead, and does so correctly even right after granting permission in System Settings:
// this settings window is a fresh process every time it's opened, so unlike the long-running main
// process it was checked from before, it's never subject to AXIsProcessTrustedWithOptions' per-
// process staleness (see that doc comment for the full explanation).
func (s *SettingsApp) IsAccessibilityTrusted() bool { return platform.AccessibilityTrustedDirect() }

// GetDefaultSettings reports the settings a fresh install starts with, so the "恢复默认设置"
// button can reset the form to them without the frontend needing to duplicate those values.
func (s *SettingsApp) GetDefaultSettings() storage.Settings { return storage.DefaultSettings() }

// GetCacheDir reports the on-disk directory SailBoard stores its database, cached images,
// content-addressed rich assets, and cached source-app icons under, so the settings UI can
// display it.
func (s *SettingsApp) GetCacheDir() (string, error) {
	if err := s.ready(); err != nil {
		return "", err
	}
	return s.dataDir, nil
}

// OpenCacheFolder opens the cache directory in Explorer.
func (s *SettingsApp) OpenCacheFolder() error {
	if err := s.ready(); err != nil {
		return err
	}
	return platform.OpenFolderDirect(s.dataDir)
}

// repositoryURL is SailBoard's GitHub repository, opened by the settings window's "在 GitHub
// 上查看" button.
const repositoryURL = "https://github.com/SciSail/sailboard"

// OpenGitHub opens SailBoard's GitHub repository in the system's default browser.
func (s *SettingsApp) OpenGitHub() {
	runtime.BrowserOpenURL(s.ctx, repositoryURL)
}

// GetStorageUsage reports the bytes owned by history content (item payloads plus each referenced
// image/rich asset once). It intentionally excludes the SQLite file, source-app icons, and other
// transient cache files so the number matches the quota used by Repository.Cleanup.
func (s *SettingsApp) GetStorageUsage() (int64, error) {
	if err := s.ready(); err != nil {
		return 0, err
	}
	return s.repository.HistoryStorageUsage(s.ctx)
}
