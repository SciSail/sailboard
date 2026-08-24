//go:build windows

package platform

import (
	"math"
	"syscall"
	"unsafe"
)

// settingsOrigWndProc holds the settings window's original window procedure, saved by
// suppressSystemMenuKey so its subclass proc can forward everything it doesn't swallow. A plain
// package variable, not a map keyed by hwnd, because the standalone settings process (see
// main.go's runSettingsWindow) only ever creates this one top-level window in its lifetime.
var settingsOrigWndProc uintptr

var settingsWndProcCallback = syscall.NewCallback(settingsWndProcDispatch)

// settingsOnReservedKey, if set via SetOnSystemMenuKeyDirect, is notified every time
// settingsWndProcDispatch swallows an SC_KEYMENU — i.e. every time the user's keypress was one
// Windows itself reserves for the system menu (a bare Alt/F10 tap, or Alt+Space — classic combos
// that never reach the DOM as a 'keydown' at all, per suppressSystemMenuKey's doc comment, so
// Settings.tsx's own keydown handler has no way to notice a capture attempt happened at all, let
// alone which one). The callback's string argument is the resolved "Mod+KEY" spec when
// settingsWndProcDispatch could work out exactly which reserved combo this was (currently just
// Alt+Space — see its doc comment), or "" when it can't be resolved (a bare Alt/F10 tap has no
// second key to report). Firing unconditionally regardless of whether the settings UI is actually
// mid-capture is fine: the frontend only reacts while its own capturing state is true.
var settingsOnReservedKey func(resolvedSpec string)

// SetOnSystemMenuKeyDirect registers callback to run whenever a reserved Alt/F10 combo is
// swallowed (see settingsOnReservedKey above). A standalone function, not a Controller method,
// for the same reason as the other *Direct helpers: the settings window process has no
// Controller of its own.
func SetOnSystemMenuKeyDirect(callback func(resolvedSpec string)) {
	settingsOnReservedKey = callback
}

// HideDockIconDirect is a macOS-only concept (see settingswindow_darwin.go) — Windows has no Dock,
// so there's nothing to hide the settings window process from here.
func HideDockIconDirect() {}

// AccessibilityTrustedDirect is a macOS-only concept (see foreground_darwin.go's doc comment) —
// Windows' paste injection (SendInput) needs no comparable permission, so always report trusted.
func AccessibilityTrustedDirect() bool { return true }

// FixSettingsWindowDirect corrects two Windows-only problems in the standalone settings window
// (main.go's runSettingsWindow) that Wails' own window creation doesn't handle for this app —
// found by title (see main.go's settingsWindowTitle) since, like findWindowByTitle's other
// callers, nothing in this process ever gets handed the HWND directly:
//
//  1. Re-sizes the window to logicalWidth x logicalHeight scaled by the *actual* DPI of the
//     monitor it lands on (GetDpiForMonitor — the same call screen_windows.go's
//     workAreaNearCursor uses for the main panel's own DPI fix), via a direct SetWindowPos rather
//     than trusting Wails' internal DPI-scaled sizing. This project has repeatedly found Wails'
//     own window geometry handling unreliable under non-100% scaling (see CLAUDE.md's
//     MonitorFromPoint/WindowSetPosition/panelHeight gotchas), and Settings.css's content already
//     fills the window with little headroom at 100% scaling — any shortfall in the actual
//     on-screen size at higher scale factors shows up immediately as a scrollbar. If Wails' own
//     scaling already produced the right physical size this is a same-size no-op.
//  2. Subclasses the window to swallow WM_SYSCOMMAND/SC_KEYMENU, so a bare Alt keypress doesn't
//     pop the (item-less, since this app never calls SetMenu) default system menu and steal
//     keyboard focus mid-capture — see Settings.tsx's shortcut-capture flow, which needs Alt to
//     reach its own keydown handler uninterrupted to record Alt-combo hotkeys. WM_SYSCOMMAND is
//     still forwarded for every other command (close/minimize/maximize/move/size all use
//     different SC_ values), so the title bar's own controls are unaffected.
func FixSettingsWindowDirect(title string, logicalWidth, logicalHeight int) {
	hwnd := findWindowByTitle(title)
	if hwnd == 0 {
		return
	}
	resizeToActualMonitorScale(hwnd, logicalWidth, logicalHeight)
	suppressSystemMenuKey(hwnd)
}

func resizeToActualMonitorScale(hwnd uintptr, logicalWidth, logicalHeight int) {
	hMonitor, _, _ := procMonitorFromWindow.Call(hwnd, monitorDefaultToNearest)
	if hMonitor == 0 {
		return
	}
	var dpiX, dpiY uint32
	if hr, _, _ := procGetDpiForMonitor.Call(hMonitor, mdtEffectiveDpi, uintptr(unsafe.Pointer(&dpiX)), uintptr(unsafe.Pointer(&dpiY))); hr != 0 || dpiX == 0 {
		return
	}
	scale := float64(dpiX) / stdDpi
	w := int(math.Round(float64(logicalWidth) * scale))
	h := int(math.Round(float64(logicalHeight) * scale))
	procSetWindowPos.Call(hwnd, 0, 0, 0, uintptr(w), uintptr(h), swpNoZOrder|swpNoActivate|swpNoMove)
}

func suppressSystemMenuKey(hwnd uintptr) {
	// gwlpWndProc (-4) is a constant, and Go constant conversion to uintptr rejects negative
	// values outright (no implicit two's-complement wraparound the way a runtime conversion
	// gets); routing it through a plain int32 variable first forces a runtime conversion, which
	// does sign-extend to the 0xFFFFFFFFFFFFFFFC SetWindowLongPtrW expects for this index.
	index := int32(gwlpWndProc)
	old, _, _ := procSetWindowLongPtr.Call(hwnd, uintptr(index), settingsWndProcCallback)
	if old == 0 {
		return
	}
	settingsOrigWndProc = old
}

func settingsWndProcDispatch(hwnd, message, wParam, lParam uintptr) uintptr {
	if uint32(message) == wmSysCommand && wParam&0xFFF0 == scKeyMenu {
		if cb := settingsOnReservedKey; cb != nil {
			go cb(resolveReservedKeySpec())
		}
		return 0
	}
	ret, _, _ := procCallWindowProc.Call(settingsOrigWndProc, hwnd, message, wParam, lParam)
	return ret
}

// resolveReservedKeySpec identifies which reserved combo just triggered SC_KEYMENU, where
// possible, so it can actually be offered as a global hotkey instead of just rejected.
//
// Alt+Space specifically *can* be registered as a real global hotkey — RegisterHotKey(MOD_ALT,
// VK_SPACE) succeeds like any other combo (verified directly against this Win32 API, not assumed)
// — the only reason it couldn't be captured at all before this function existed is that
// suppressSystemMenuKey's swallow (needed so it doesn't pop the system menu and steal focus mid-
// capture) also means Settings.tsx's keydown handler never sees it, the same as a bare Alt tap.
// GetAsyncKeyState(VK_SPACE) at the moment SC_KEYMENU fires is what tells the two apart: Alt+Space
// generates WM_SYSCOMMAND while Space is still physically held down (TranslateMessage turns the
// WM_SYSKEYDOWN for Space into WM_SYSCHAR(' ') essentially immediately, well before any keyup),
// whereas a bare Alt tap's SC_KEYMENU fires on Alt's own keyup with no other key involved — Space
// won't read as down for that case. Registering this as SailBoard's global hotkey does mean every
// Alt+Space press system-wide summons SailBoard instead of reaching whatever window would
// otherwise have shown its system menu — a real trade-off, left for Settings.tsx to surface, not
// silently hidden here.
func resolveReservedKeySpec() string {
	if state, _, _ := procGetAsyncKeyState.Call(vkSpace); state&0x8000 != 0 {
		return "Alt+SPACE"
	}
	return ""
}
