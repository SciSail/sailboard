// Package platform isolates every OS-specific capability (global hotkeys, foreground-window
// tracking, paste injection, source-app lookup, monitor geometry, autostart, tray icon) behind
// a single Controller interface. app.go and internal/clipboard never call an OS API directly;
// they only depend on this interface, so adding macOS/Linux later only means adding another
// controller_<os>.go file.
package platform

// Rect is a screen-space rectangle in pixels (top-left origin).
type Rect struct {
	X, Y, Width, Height int
}

// AppInfo describes the application that currently owns keyboard focus.
type AppInfo struct {
	Name           string
	ExecutablePath string
	IconPNG        []byte
}

// ForegroundToken opaquely identifies whatever window had focus before SailBoard was shown, so
// it can be restored after the user picks a history item. It must not be persisted across app
// restarts.
type ForegroundToken interface{}

// TrayOptions wires tray-menu actions back into the App layer.
type TrayOptions struct {
	// IconPNG is the tray icon image, PNG-encoded. Nil/empty falls back to a generic system
	// icon (only the stub controller and, on Windows, a malformed PNG take this path).
	IconPNG       []byte
	OnShow        func()
	OnTogglePause func()
	OnQuit        func()
}

// Controller is the full set of native capabilities SailBoard needs from the host OS.
type Controller interface {
	Close()

	// RegisterHotkey binds a global shortcut (e.g. "Ctrl+Shift+V") to handler. The returned
	// func unregisters it; call it before registering a replacement binding.
	RegisterHotkey(spec string, handler func()) (unregister func(), err error)

	// CaptureForeground records whatever window currently has focus, so it can be restored
	// after SailBoard hides itself.
	CaptureForeground() ForegroundToken
	RestoreForeground(token ForegroundToken)

	// SendPaste simulates the platform paste shortcut (Ctrl+V / Cmd+V) in the foreground app.
	SendPaste() error

	// FocusSelf forces OS keyboard focus onto SailBoard's own window (identified by its title,
	// matching the app.Title set in main.go) after the backend shows it. Without this, a window
	// shown from a background process (as our hotkey handler is) can appear on screen without
	// ever receiving keyboard focus, silently breaking every keyboard interaction.
	FocusSelf(title string)

	// PositionSelf moves and resizes SailBoard's own window (found by title, same convention as
	// FocusSelf) to an absolute screen rectangle. Use this instead of the Wails runtime's
	// WindowSetPosition/WindowSetSize for multi-monitor placement — see PositionSelf's Windows
	// doc comment for why.
	PositionSelf(title string, x, y, width, height int)

	// WatchFocusLoss calls onLostFocus exactly when the foreground window changes away from every
	// window titled by one of titles, to some window titled by none of them. Pass every window
	// SailBoard itself owns (the main panel and, while it's open, its standalone settings window)
	// so switching between them never counts as "losing focus" — only switching to some other
	// app's window does. Install once; it stays armed for the process lifetime and simply won't
	// fire while some other window is already foreground.
	WatchFocusLoss(titles []string, onLostFocus func())

	// FocusIfExists brings a window titled title to the foreground and reports true, or reports
	// false without side effects if no such window exists. Used to avoid opening a second
	// settings window when one is already open.
	FocusIfExists(title string) bool

	// OnSettingsChanged registers callback to run whenever the settings (window) process signals
	// a save via NotifySettingsChanged. Most saved settings only need to take effect at the next
	// natural sync point, but a changed global hotkey can't wait for one — the callback should
	// reload settings and re-register the hotkey.
	OnSettingsChanged(callback func())

	// OnShowRequested registers callback to run whenever a second, redundant launch of SailBoard
	// hands off to this (the already-running) process instead of starting its own copy — see
	// AcquireSingleInstanceLock and RequestShowMainWindow. The callback should show the panel.
	OnShowRequested(callback func())

	// OnHotkeySuspendRequested/OnHotkeyResumeRequested register callbacks the settings window
	// process triggers (via SuspendHotkeyDirect/ResumeHotkeyDirect) while the user is capturing a
	// new global shortcut in the settings UI: the *current* hotkey is still live and global until
	// the new one is saved, so without this, pressing it while trying out combinations pops the
	// main panel over the settings window mid-capture. The suspend callback should unregister the
	// current hotkey; the resume callback should reload settings and re-register it — same pair
	// of actions OnSettingsChanged's callback already does on save, just triggered earlier/without
	// a save. Both are safe to call when already in the requested state (e.g. resume without a
	// prior suspend just re-registers the same hotkey).
	OnHotkeySuspendRequested(callback func())
	OnHotkeyResumeRequested(callback func())

	// SlideReveal shows SailBoard's own window (found by title) already positioned at
	// (x, y, width, height), then physically slides the whole window up from just off the bottom
	// of the screen to that position over durationMs — replacing PositionSelf + Wails' plain
	// instant WindowShow for the animated reveal path. The background moves together with
	// .sheet's content because it's the same physical window carrying both, rather than a
	// separately-timed animation kept in sync with CSS. See slide_windows.go's doc comment for
	// why this replaced two earlier (alpha-based) designs. Blocks for durationMs — call it from a
	// goroutine, never the UI/message-loop thread.
	SlideReveal(title string, x, y, width, height, durationMs int) error

	// SlideDismiss is SlideReveal's counterpart: reads the window's current rectangle and slides
	// it back down by its own height (off-screen at the bottom) before hiding it. Blocks for
	// durationMs, same as SlideReveal.
	SlideDismiss(title string, durationMs int) error

	// ActiveApp identifies the application currently owning keyboard focus, used to tag newly
	// captured clipboard items with their source app.
	ActiveApp() (AppInfo, error)

	// WorkAreaNearCursor returns the usable (taskbar-excluded) area of the monitor under the
	// mouse cursor, falling back to the primary display, along with that monitor's DPI scale
	// factor (1.0 = 100% scaling, 2.0 = 200%, etc). PositionSelf/SlideReveal both place the
	// window using this same Rect's coordinate space (physical pixels on Windows, under this
	// app's Per-Monitor-V2 DPI awareness — see build/windows/wails.exe.manifest), so any length
	// meant to render at a fixed logical/CSS size on screen — like the panel's target height —
	// must be multiplied by scale before being passed to those calls, or it silently ends up
	// half its intended on-screen size at 200% scaling (and so on for other scale factors). On
	// platforms where window placement is already DPI-independent (macOS's point-based
	// coordinates), scale is always 1.0. ok is false if geometry is unavailable.
	WorkAreaNearCursor() (area Rect, scale float64, ok bool)

	// ReadClipboardImage returns the current clipboard image re-encoded as PNG, if present.
	ReadClipboardImage() (data []byte, width, height int, ok bool)

	// WriteClipboardImage writes a PNG-encoded image back to the system clipboard as a native
	// image format, so a pasted history item behaves like a fresh copy.
	WriteClipboardImage(data []byte) error

	// ReadClipboardRichText returns "HTML Format"/"Rich Text Format" markup alongside plain text,
	// if the clipboard currently holds a genuine formatted-text copy (see
	// clipboard_richtext_windows.go's doc comment for the "genuine" heuristic — this must report
	// ok=false for a bare image copy that happens to carry incidental markup, not just "some rich
	// format is present"). Checked before ReadClipboardImage each tick in watcher.go, since Office
	// always attaches a preview bitmap to a text copy that should be captured as text, not image.
	ReadClipboardRichText() (html, rtf, text string, ok bool)

	// WriteClipboardRichText writes html and/or rtf back onto the clipboard alongside text as
	// plain CF_UNICODETEXT, so pasting into an app that understands the rich format keeps
	// formatting, while a plain-text-only app transparently falls back to text — the receiving
	// app picks, SailBoard doesn't need to know which app is focused.
	WriteClipboardRichText(html, rtf, text string) error

	// FileThumbnail returns a best-effort PNG thumbnail for path: a real, downscaled preview for
	// image files SailBoard knows how to decode, or the OS's generic per-type/per-folder icon
	// otherwise. ok is false if path no longer exists or nothing could be produced.
	FileThumbnail(path string) (data []byte, ok bool)

	// PreviewFile toggles the OS-native Quick Look preview panel (macOS's Space-to-preview, the
	// same panel Finder/Mail show) for the given file path(s) — a multi-file capture previews all
	// of them together, navigable inside the panel itself. If the panel is already open, this
	// closes it instead, mirroring the system-wide spacebar toggle; the frontend only calls this
	// from a fresh Space keypress, never to silently refresh an already-open panel. Returns true
	// if the panel is now showing, false if it was closed (or nothing in paths exists on disk).
	// Windows/stub implementations always return false — there's no native Quick Look equivalent
	// there, so the frontend's Space handler harmlessly no-ops on that result.
	PreviewFile(paths []string) bool

	// ReadClipboardFiles returns the file/folder paths currently on the clipboard (CF_HDROP), if
	// present.
	ReadClipboardFiles() (paths []string, ok bool)

	// WriteClipboardFiles writes paths back onto the system clipboard as a native file drop
	// (CF_HDROP) with a "copy" preferred drop effect, so a pasted history item results in a real
	// copy of the original file(s)/folder(s), never a move.
	WriteClipboardFiles(paths []string) error

	// ClipboardSequence returns a monotonically increasing counter that changes every time the
	// clipboard content changes, letting the watcher avoid polling by string diff. ok is false
	// if the platform doesn't expose one, and callers should fall back to content comparison.
	ClipboardSequence() (uint32, bool)

	SetAutoLaunch(enabled bool) error
	AutoLaunchEnabled() (bool, error)

	// ShowTray creates (or updates) the system tray icon. Safe to call multiple times.
	ShowTray(opts TrayOptions)
	UpdateTrayPaused(paused bool)
}
