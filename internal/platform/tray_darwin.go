//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "tray_darwin.h"
*/
import "C"

import "unsafe"

// darwinTrayOpts is package-level (not a darwinController field) because the click callbacks
// below are cgo-exported free functions, not methods — Objective-C's target-action pattern calls
// a selector on a plain object, with no way to thread a Go receiver through, so tray_darwin.m's
// SBTrayTarget just calls these directly.
var darwinTrayOpts TrayOptions

// ShowTray creates the status item, then hides the Dock icon — deliberately in that order.
// Wails' own AppDelegate.m sets the activation policy to Regular during launch (see
// darwinHideDockIcon's doc comment), and empirically, switching to Accessory *before* the status
// item exists leaves it invisible — the item never renders even though creation reports success.
// Creating it while still Regular (matching how every other Wails/Cocoa app creates one) and only
// then switching policy avoids that: the item was already valid before the Dock icon disappears
// out from under it.
func (c *darwinController) ShowTray(opts TrayOptions) {
	darwinTrayOpts = opts
	if len(opts.IconPNG) == 0 {
		C.sb_show_tray(nil, 0)
	} else {
		C.sb_show_tray((*C.uchar)(unsafe.Pointer(&opts.IconPNG[0])), C.long(len(opts.IconPNG)))
	}
	darwinHideDockIcon()
}

func (c *darwinController) UpdateTrayPaused(paused bool) {
	p := C.int(0)
	if paused {
		p = 1
	}
	C.sb_update_tray_paused(p)
}

// sbTray*Clicked are invoked synchronously from AppKit's menu-item target-action dispatch, still
// on the main thread's own callstack — routed through darwinMainThreadCallbacks (controller_
// darwin.go) rather than run in-place for the same reason sbHotkeyFired is (see that function's
// doc comment): OnShow ends up in app.go's ShowWindow, which spawns its own goroutine for the
// slide animation, and doing that from directly inside this callback crashes.

//export sbTrayShowClicked
func sbTrayShowClicked() {
	select {
	case darwinMainThreadCallbacks <- func() {
		if darwinTrayOpts.OnShow != nil {
			darwinTrayOpts.OnShow()
		}
	}:
	default:
	}
}

//export sbTrayToggleClicked
func sbTrayToggleClicked() {
	select {
	case darwinMainThreadCallbacks <- func() {
		if darwinTrayOpts.OnTogglePause != nil {
			darwinTrayOpts.OnTogglePause()
		}
	}:
	default:
	}
}

//export sbTrayQuitClicked
func sbTrayQuitClicked() {
	select {
	case darwinMainThreadCallbacks <- func() {
		if darwinTrayOpts.OnQuit != nil {
			darwinTrayOpts.OnQuit()
		}
	}:
	default:
	}
}
