//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "controller_darwin.h"
*/
import "C"

import (
	"fmt"
	"sync"
	"time"
	"unsafe"
)

// darwinMainThreadCallbacks decouples "a Cocoa/Carbon callback fired, synchronously, on the main
// thread" from "run the Go handler it names". hotkey_darwin.go's sbHotkeyFired and tray_darwin.
// go's sbTray*Clicked are both cgo-exported functions Objective-C calls directly from the main
// thread's event loop (Carbon's hotkey dispatch, AppKit's menu-item target-action) — running the
// handler in-place from there crashed with SIGSEGV (see hotkey_darwin.go's sbHotkeyFired doc
// comment for the full diagnosis): the handler is app.go's ShowWindow, which spawns its own
// goroutine to run SlideReveal's animation, and a goroutine spawned from code that is itself
// executing as a nested cgo callback reliably faulted on its first call back into Cocoa. Routing
// every such handler through this channel and the one persistent, ordinarily-`go`-started
// goroutine below means the handler always runs from an normal goroutine — never from inside a
// Carbon/AppKit callback's own stack — sidestepping the whole class of problem.
var darwinMainThreadCallbacks = make(chan func(), 8)

func init() {
	go func() {
		for fn := range darwinMainThreadCallbacks {
			fn()
		}
	}()
}

// darwinController implements the pieces of Controller that were making the macOS build behave
// like a floating, undismissable window instead of a Windows-style bottom sheet: real work-area
// geometry, absolute positioning, the reveal/dismiss slide, keyboard focus, focus-loss auto-hide,
// a menu-bar tray icon, and the global hotkey — all via Cocoa/Carbon (see controller_darwin.m,
// hotkey_darwin.*, tray_darwin.*). Everything else (paste injection, native clipboard formats,
// autostart, source-app lookup) is still the no-op stub inherited from stubController; see
// progress.md's macOS roadmap for what's left.
//
// Unlike the Windows implementation, this one uses cgo: Cocoa (NSWindow/NSScreen/NSApplication)
// is only reachable via Objective-C message sends, and Wails' own darwin frontend already forces
// CGO_ENABLED=1 for this OS, so there's no "avoid cgo" benefit to chase here the way there is on
// Windows (whose files use raw syscall.NewLazyDLL specifically to avoid cgo, per that file's own
// doc comments).
type darwinController struct {
	stubController

	appName        string
	focusWatchOnce sync.Once
	stopFocusWatch chan struct{}
}

// New creates the macOS controller. Most Controller methods are still the inherited stub; see
// darwinController's doc comment for what's real here. The Dock-icon-hiding policy switch lives
// in ShowTray (see darwinHideDockIcon and that method's doc comment for why the ordering matters).
func New(appName string) (Controller, error) {
	return &darwinController{appName: appName}, nil
}

func (c *darwinController) ClipboardSnapshotSupported() bool          { return true }
func (c *darwinController) ClipboardChanges() (<-chan struct{}, bool) { return nil, false }
func (c *darwinController) ReadClipboardSnapshot() (ClipboardSnapshot, error) {
	return readClipboardSnapshotDarwin(c)
}

// darwinHideDockIcon forces the app's activation policy to Accessory (no Dock icon, no Cmd+Tab
// entry, menu-bar-only presence) — called from tray_darwin.go's ShowTray, not here; see that
// method's doc comment for why Info.plist's LSUIElement key alone isn't enough and why ordering
// relative to creating the status item matters.
func darwinHideDockIcon() {
	C.sb_set_activation_policy_accessory()
}

func (c *darwinController) Close() {
	if c.stopFocusWatch != nil {
		close(c.stopFocusWatch)
	}
}

// withCTitle converts title to a C string for the duration of fn, freeing it afterward. Every
// cgo entry point below is synchronous (dispatch_sync on the Objective-C side), so the string
// never needs to outlive this call.
func withCTitle(title string, fn func(*C.char)) {
	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))
	fn(cTitle)
}

func (c *darwinController) FocusSelf(title string) {
	withCTitle(title, func(t *C.char) { C.sb_focus_self(t) })
}

func (c *darwinController) PositionSelf(title string, x, y, width, height int) {
	withCTitle(title, func(t *C.char) {
		C.sb_set_frame_topleft(t, C.double(x), C.double(y), C.double(width), C.double(height))
	})
}

// WorkAreaNearCursor's scale is always 1.0 on macOS: AppKit's coordinate space (points) is
// already DPI-independent — a Retina display's higher backingScaleFactor changes pixel density,
// not the point-based sizes windows are placed/sized in, so there's no analogue of Windows'
// physical-pixel scaling bug here.
func (c *darwinController) WorkAreaNearCursor() (Rect, float64, bool) {
	var ok C.int
	r := C.sb_work_area_near_cursor(&ok)
	if ok == 0 {
		return Rect{}, 1, false
	}
	return Rect{X: int(r.x), Y: int(r.y), Width: int(r.w), Height: int(r.h)}, 1, true
}

// darwinSlideStepMs matches slide_windows.go's own step interval so the reveal/dismiss animation
// feels identical on both platforms. Named distinctly (not shared) so this file never has to
// import from a _windows.go file, keeping the two platforms' build graphs fully independent.
const darwinSlideStepMs = 10

// SlideReveal mirrors slideReveal on Windows: position off-screen one panel-height below the
// final rectangle, show without activating, then animate the vertical position up over
// durationMs. See slide_windows.go's doc comment for why physically moving the window (rather
// than an alpha cross-fade) is what reads as smooth here.
func (c *darwinController) SlideReveal(title string, x, y, width, height, durationMs int) error {
	var ok C.int
	withCTitle(title, func(t *C.char) { C.sb_get_frame_topleft(t, &ok) })
	if ok == 0 {
		return fmt.Errorf("window titled %q not found", title)
	}
	startY := y + height // one panel-height below the final position: fully off-screen at the bottom
	c.PositionSelf(title, x, startY, width, height)
	withCTitle(title, func(t *C.char) { C.sb_show_no_activate(t) })
	c.slideY(title, x, width, height, startY, y, durationMs)
	return nil
}

// SlideDismiss mirrors slideDismiss on Windows: read the window's current rectangle back (its
// native frame is the source of truth, not anything cached Go-side), slide it down by its own
// height, then hide it.
func (c *darwinController) SlideDismiss(title string, durationMs int) error {
	var ok C.int
	var r C.sb_rect
	withCTitle(title, func(t *C.char) { r = C.sb_get_frame_topleft(t, &ok) })
	if ok == 0 {
		return fmt.Errorf("window titled %q not found", title)
	}
	x, y, w, h := int(r.x), int(r.y), int(r.w), int(r.h)
	c.slideY(title, x, w, h, y, y+h, durationMs)
	withCTitle(title, func(t *C.char) { C.sb_hide(t) })
	return nil
}

// slideY animates the window titled title from fromY to toY over durationMs, holding x/width/
// height fixed — same ease-out cubic curve as slide_windows.go's slideY, duplicated rather than
// shared so this file never touches (or risks) the Windows build.
func (c *darwinController) slideY(title string, x, width, height, fromY, toY, durationMs int) {
	move := func(y int) { c.PositionSelf(title, x, y, width, height) }
	if durationMs <= 0 || fromY == toY {
		move(toY)
		return
	}
	total := time.Duration(durationMs) * time.Millisecond
	start := time.Now()
	for {
		t := float64(time.Since(start)) / float64(total)
		if t >= 1 {
			break
		}
		eased := 1 - (1-t)*(1-t)*(1-t) // ease-out cubic
		move(fromY + int(float64(toY-fromY)*eased))
		time.Sleep(darwinSlideStepMs * time.Millisecond)
	}
	move(toY)
}

// WatchFocusLoss polls a frontmost-app check (sb_app_is_active) instead of watching individual
// window notifications. Cocoa's per-window key/main notifications don't distinguish "switched to
// SailBoard's own other window" (main panel <-> settings) from "switched to some other app" — that
// distinction instead comes from sb_app_is_active comparing the system-wide frontmost app's bundle
// identifier against our own, which correctly reads as "still us" whether the frontmost SailBoard
// process is this one or the standalone settings window one (main.go's runSettingsWindow spawns it
// as a separate OS process, not a second window of this process, so plain [NSApp isActive] alone
// can't see past its own process boundary — see controller_darwin.m's doc comment for the bug this
// fixes: opening settings used to make this process's own isActive go genuinely false, incorrectly
// firing onLostFocus and hiding the main panel out from under the settings window that was just
// opened). That's exactly the distinction titles exists to make on Windows (see focuswatch_windows.
// go, which filters WinEventHook notifications by window title for the same reason) — so titles is
// unused here, sb_app_is_active already gets the right semantics for free. Polling avoids the extra
// NSNotificationCenter observer/cgo-callback plumbing an event-driven version would need, at the
// cost of the same latency order as the existing clipboard watcher's own poll interval.
func (c *darwinController) WatchFocusLoss(titles []string, onLostFocus func()) {
	c.focusWatchOnce.Do(func() {
		c.stopFocusWatch = make(chan struct{})
		go func() {
			wasActive := true
			ticker := time.NewTicker(200 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-c.stopFocusWatch:
					return
				case <-ticker.C:
					active := C.sb_app_is_active() != 0
					if wasActive && !active {
						onLostFocus()
					}
					wasActive = active
				}
			}
		}()
	})
}
