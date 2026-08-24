//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework CoreGraphics
#include <stdlib.h>
#include "ipc_darwin.h"
*/
import "C"

import "unsafe"

// darwinOnSettingsChanged/darwinOnShowRequested/darwinOnHotkeySuspend/darwinOnHotkeyResume are
// package-level (not darwinController fields) because the cgo-exported callback functions below
// are free functions, not methods — same reasoning as tray_darwin.go's darwinTrayOpts.
var (
	darwinOnSettingsChanged func()
	darwinOnShowRequested   func()
	darwinOnHotkeySuspend   func()
	darwinOnHotkeyResume    func()
)

func (c *darwinController) OnSettingsChanged(callback func()) {
	darwinOnSettingsChanged = callback
	C.sb_watch_distributed_notifications()
}

func (c *darwinController) OnShowRequested(callback func()) {
	darwinOnShowRequested = callback
	C.sb_watch_distributed_notifications()
}

// OnHotkeySuspendRequested/OnHotkeyResumeRequested are the macOS counterpart to
// controller_windows.go's identically-named methods — see Controller.OnHotkeySuspendRequested's
// doc comment (types.go) for why the settings window needs these while capturing a new shortcut.
// Same distributed-notification plumbing as OnSettingsChanged/OnShowRequested above, just two more
// notification names.
func (c *darwinController) OnHotkeySuspendRequested(callback func()) {
	darwinOnHotkeySuspend = callback
	C.sb_watch_distributed_notifications()
}

func (c *darwinController) OnHotkeyResumeRequested(callback func()) {
	darwinOnHotkeyResume = callback
	C.sb_watch_distributed_notifications()
}

func (c *darwinController) FocusIfExists(title string) bool {
	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))
	return C.sb_focus_if_exists(cTitle) != 0
}

// NotifySettingsChanged and RequestShowMainWindow are standalone entry points (not Controller
// methods) mirroring ipc_windows.go's identical split — the settings window is a separate process
// with no Controller of its own, and a redundant second main-process launch (see main.go's
// runMainWindow) hasn't built one yet either.
func NotifySettingsChanged() {
	C.sb_notify_settings_changed()
}

func RequestShowMainWindow() {
	C.sb_notify_show_main()
}

// SuspendHotkeyDirect and ResumeHotkeyDirect are the real macOS implementation of the standalone
// helpers settingswindow_darwin.go previously left as no-ops (see that file's history/progress.md
// for why: it needed a Mac dev machine to build and verify the cgo/.m side, which this session
// finally has) — same standalone-function/distributed-notification pattern as
// NotifySettingsChanged/RequestShowMainWindow above, since the settings window process has no
// Controller of its own to call OnHotkeySuspendRequested's callback through directly.
func SuspendHotkeyDirect() {
	C.sb_notify_suspend_hotkey()
}

func ResumeHotkeyDirect() {
	C.sb_notify_resume_hotkey()
}

//export sbSettingsChangedNotification
func sbSettingsChangedNotification() {
	select {
	case darwinMainThreadCallbacks <- func() {
		if darwinOnSettingsChanged != nil {
			darwinOnSettingsChanged()
		}
	}:
	default:
	}
}

//export sbShowMainNotification
func sbShowMainNotification() {
	select {
	case darwinMainThreadCallbacks <- func() {
		if darwinOnShowRequested != nil {
			darwinOnShowRequested()
		}
	}:
	default:
	}
}

//export sbSuspendHotkeyNotification
func sbSuspendHotkeyNotification() {
	select {
	case darwinMainThreadCallbacks <- func() {
		if darwinOnHotkeySuspend != nil {
			darwinOnHotkeySuspend()
		}
	}:
	default:
	}
}

//export sbResumeHotkeyNotification
func sbResumeHotkeyNotification() {
	select {
	case darwinMainThreadCallbacks <- func() {
		if darwinOnHotkeyResume != nil {
			darwinOnHotkeyResume()
		}
	}:
	default:
	}
}
