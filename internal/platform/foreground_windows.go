//go:build windows

package platform

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

type foregroundToken struct{ hwnd uintptr }

// findWindowByTitle looks up a top-level window by its exact title. SailBoard uses this instead
// of stashing an HWND because the value is needed from contexts (goroutines spawned well after
// window creation, callbacks) that never had one handed to them, and Wails doesn't expose its
// main window's HWND through any public API.
func findWindowByTitle(title string) uintptr {
	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return 0
	}
	hwnd, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	return hwnd
}

// captureForeground records whatever window currently owns focus, per design doc §25: this must
// be refreshed on every show, never cached across app sessions.
func captureForeground() ForegroundToken {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return nil
	}
	return foregroundToken{hwnd: hwnd}
}

func restoreForeground(token ForegroundToken) {
	t, ok := token.(foregroundToken)
	if !ok || t.hwnd == 0 {
		return
	}
	procSetForegroundWindow.Call(t.hwnd)
}

// focusSelf forces keyboard focus onto SailBoard's own top-level window (found by its title, so
// this only needs a string, not a stashed HWND) after the backend shows it.
//
// Windows normally refuses SetForegroundWindow from a background process — see the
// SetForegroundWindow remarks on MSDN — and Wails' own WindowShow() already tries and silently
// fails: our hotkey handler runs on a goroutine spawned off the message-loop thread (see
// winmsg_windows.go), so by the time it calls into the Wails runtime, Windows no longer
// considers it "still handling" the WM_HOTKEY input that would normally grant the exemption.
// The documented workaround is to temporarily attach our thread's input queue to the current
// foreground window's thread via AttachThreadInput, which makes Windows treat the two threads as
// sharing input focus state and allows SetForegroundWindow to succeed. Without this, the panel
// appears on screen but arrow keys/Enter/Esc/typing all keep going to whatever app was focused
// before the hotkey — every keyboard interaction silently does nothing.
func focusSelf(title string) {
	hwnd := findWindowByTitle(title)
	if hwnd == 0 {
		return
	}

	fg, _, _ := procGetForegroundWindow.Call()
	curThread, _, _ := procGetCurrentThreadId.Call()
	var fgThread uintptr
	attached := false
	if fg != 0 && fg != hwnd {
		fgThread, _, _ = procGetWindowThreadPID.Call(fg, 0)
		if fgThread != 0 && fgThread != curThread {
			if ok, _, _ := procAttachThreadInput.Call(curThread, fgThread, 1); ok != 0 {
				attached = true
			}
		}
	}

	procSetForegroundWindow.Call(hwnd)
	procBringWindowToTop.Call(hwnd)
	procSetFocus.Call(hwnd)

	if attached {
		procAttachThreadInput.Call(curThread, fgThread, 0)
	}

	// See markSelfFocused's doc comment: don't rely on the WinEventHook observing this same
	// SetForegroundWindow call back in time — tell the focus-loss watcher directly.
	markSelfFocused()
}

// positionSelf moves and resizes SailBoard's own window (found by title) directly via
// SetWindowPos, in absolute screen coordinates.
//
// This deliberately bypasses Wails' runtime.WindowSetPosition: its Windows backend
// (winc.ControlBase.SetPos) computes the target as `currentMonitorWorkArea.Origin + (x, y)` —
// i.e. it treats (x, y) as an offset from the work area of whichever monitor the window
// currently happens to be on, not as absolute screen coordinates. That happens to be
// indistinguishable from correct on a single primary monitor (whose work area origin is (0,0)),
// which is how this went unnoticed, but it silently misplaces the window on any setup where the
// target monitor isn't the window's current one and doesn't start at (0,0) — exactly the
// multi-monitor case WorkAreaNearCursor exists to handle. SetWindowPos takes plain absolute
// coordinates, sidestepping the quirk entirely.
func positionSelf(title string, x, y, width, height int) {
	hwnd := findWindowByTitle(title)
	if hwnd == 0 {
		return
	}
	procSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(width), uintptr(height), swpNoZOrder|swpNoActivate)
}

// sendPaste simulates Ctrl+V via SendInput. This can fail silently (return success but do
// nothing) when the foreground app runs elevated and SailBoard doesn't — design doc §24 accepts
// that as expected first-version behaviour: the content is still on the clipboard, so the UI
// falls back to "Copied. Press Ctrl+V to paste." when PasteToPreviousApp fails outright.
func sendPaste() error {
	time.Sleep(40 * time.Millisecond) // let the restored foreground window finish activating

	down := func(vk uint16) input {
		return input{Type: inputKeyboard, Ki: keybdInput{WVk: vk}}
	}
	up := func(vk uint16) input {
		return input{Type: inputKeyboard, Ki: keybdInput{WVk: vk, DwFlags: keyEventFKeyUp}}
	}

	events := []input{down(vkControl), down(vkV), up(vkV), up(vkControl)}
	n := len(events)
	ret, _, err := procSendInput.Call(
		uintptr(n),
		uintptr(unsafe.Pointer(&events[0])),
		unsafe.Sizeof(events[0]),
	)
	if int(ret) != n {
		return fmt.Errorf("SendInput: only %d/%d events accepted: %w", ret, n, err)
	}
	return nil
}
