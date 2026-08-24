//go:build windows

package platform

import (
	"syscall"
	"unsafe"
)

// singleInstanceMutexName is process-wide (Local\ prefix — no cross-session/Terminal-Services
// sharing needed) and namespaced with a fixed random suffix so it can never collide with an
// unrelated app's mutex.
const singleInstanceMutexName = `Local\SailBoard-SingleInstance-3F1E8B2C`

// AcquireSingleInstanceLock claims a process-wide named mutex so only one SailBoard main-window
// process ever runs at a time. ok is false when another process already holds it — main.go should
// then call RequestShowMainWindow to hand off to that instance and exit immediately rather than
// starting a redundant second copy (which would double-capture the clipboard, double-register the
// hotkey, fight over the tray icon, etc.).
//
// The returned handle is intentionally never closed: it must stay held for this process's entire
// lifetime, and Windows releases it automatically on process exit.
func AcquireSingleInstanceLock() (ok bool, err error) {
	namePtr, err := syscall.UTF16PtrFromString(singleInstanceMutexName)
	if err != nil {
		return false, err
	}
	ret, _, callErr := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(namePtr)))
	if ret == 0 {
		return false, callErr
	}
	// CreateMutex uniquely sets GetLastError to ERROR_ALREADY_EXISTS (even though the call itself
	// still succeeds and hands back a valid, usable handle) when the named mutex already existed
	// — the documented way to detect this, not a real failure.
	return callErr != syscall.ERROR_ALREADY_EXISTS, nil
}
