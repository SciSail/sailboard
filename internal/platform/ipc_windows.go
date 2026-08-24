//go:build windows

package platform

import (
	"syscall"
	"unsafe"
)

// NotifySettingsChanged tells a running main SailBoard process (if any) that settings changed on
// disk, so it can reload and reapply them. This exists because most settings (retention, max
// storage, launch-at-login) only need to take effect at the next natural sync point, but the
// global hotkey can't wait for one — the *old* hotkey stops working the moment it's
// unregistered, and nothing would ever trigger a reload of the *new* one without this nudge. A
// standalone function (not a Controller method) because the settings window is a separate
// process that has no Controller/message-loop of its own — it just needs to poke the one main
// process that does. Safe to call even when no main process is running (finds nothing, no-ops).
//
// FindWindowExW with HWND_MESSAGE as the parent is the documented way to locate a message-only
// window cross-process; a plain FindWindow only sees ordinary top-level windows.
func NotifySettingsChanged() {
	titlePtr, err := syscall.UTF16PtrFromString(msgWindowClassName)
	if err != nil {
		return
	}
	namePtr := uintptr(unsafe.Pointer(titlePtr))
	hwnd, _, _ := procFindWindowEx.Call(hwndMessage, 0, namePtr, namePtr)
	if hwnd == 0 {
		return
	}
	procPostMessage.Call(hwnd, wmSettingsChanged, 0, 0)
}

// RequestShowMainWindow tells a running main SailBoard process (if any) to show its panel. Called
// by a second, redundant launch of the app (see main.go's runMainWindow / AcquireSingleInstanceLock)
// right before it exits, so "open SailBoard again" behaves like "bring the existing one to the
// front" instead of starting a second copy. Same standalone-function/message-only-window lookup
// pattern as NotifySettingsChanged — see its doc comment for why this isn't a Controller method.
func RequestShowMainWindow() {
	titlePtr, err := syscall.UTF16PtrFromString(msgWindowClassName)
	if err != nil {
		return
	}
	namePtr := uintptr(unsafe.Pointer(titlePtr))
	hwnd, _, _ := procFindWindowEx.Call(hwndMessage, 0, namePtr, namePtr)
	if hwnd == 0 {
		return
	}
	procPostMessage.Call(hwnd, wmShowMain, 0, 0)
}

// SuspendHotkeyDirect and ResumeHotkeyDirect tell a running main SailBoard process (if any) to
// unregister/re-register its current global hotkey — see Controller.OnHotkeySuspendRequested's
// doc comment for why the settings window (which has no Controller/message-loop of its own,
// same reasoning as NotifySettingsChanged above) calls these around its shortcut-capture UI
// rather than just letting the old hotkey answer while a new one is being tried out. Safe to call
// even when no main process is running (finds nothing, no-ops).
func SuspendHotkeyDirect() {
	titlePtr, err := syscall.UTF16PtrFromString(msgWindowClassName)
	if err != nil {
		return
	}
	namePtr := uintptr(unsafe.Pointer(titlePtr))
	hwnd, _, _ := procFindWindowEx.Call(hwndMessage, 0, namePtr, namePtr)
	if hwnd == 0 {
		return
	}
	procPostMessage.Call(hwnd, wmSuspendHotkey, 0, 0)
}

func ResumeHotkeyDirect() {
	titlePtr, err := syscall.UTF16PtrFromString(msgWindowClassName)
	if err != nil {
		return
	}
	namePtr := uintptr(unsafe.Pointer(titlePtr))
	hwnd, _, _ := procFindWindowEx.Call(hwndMessage, 0, namePtr, namePtr)
	if hwnd == 0 {
		return
	}
	procPostMessage.Call(hwnd, wmResumeHotkey, 0, 0)
}

// focusIfExists reports whether a top-level window titled title exists and, if so, brings it to
// the foreground — used to avoid spawning a second settings window when one is already open.
func focusIfExists(title string) bool {
	if findWindowByTitle(title) == 0 {
		return false
	}
	focusSelf(title)
	return true
}
