//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices
#include "foreground_darwin.h"
*/
import "C"

import (
	"fmt"
	"sync"
	"time"
)

// darwinForegroundToken carries a pid across the ForegroundToken interface{} boundary — see
// sb_capture_foreground's doc comment for why a pid rather than any Cocoa object handle.
type darwinForegroundToken struct {
	pid int
}

func (c *darwinController) CaptureForeground() ForegroundToken {
	pid := int(C.sb_capture_foreground())
	if pid == 0 {
		return nil
	}
	return darwinForegroundToken{pid: pid}
}

func (c *darwinController) RestoreForeground(token ForegroundToken) {
	t, ok := token.(darwinForegroundToken)
	if !ok || t.pid == 0 {
		return
	}
	C.sb_restore_foreground(C.int(t.pid))
}

// darwinAccessibilityPromptOnce ensures the system's Accessibility permission dialog (triggered
// by sb_accessibility_trusted(1)) is shown at most once per run rather than on every failed paste
// attempt — AXIsProcessTrustedWithOptions will happily re-show it every call while untrusted, and
// re-popping a system permission dialog on every Enter press would be far more annoying than
// SailBoard's existing "copied — press Cmd+V yourself" fallback (which SendPaste's returned error
// already triggers in app.go/the frontend, unchanged from before this file existed).
var darwinAccessibilityPromptOnce sync.Once

func (c *darwinController) SendPaste() error {
	if C.sb_accessibility_trusted(0) == 0 {
		darwinAccessibilityPromptOnce.Do(func() {
			C.sb_accessibility_trusted(1) // triggers the one-time system permission dialog
		})
		// The full "how to fix this" instructions used to live in this error string, shown inline
		// in the main panel's paste-failure notice — now surfaced proactively instead, in the
		// settings window (see AccessibilityTrustedDirect/SettingsApp.IsAccessibilityTrusted),
		// checked fresh every time it opens rather than only after a failed paste. Kept short here
		// since the main panel still shows *something* on a failed paste (the copy itself always
		// succeeds), it just no longer duplicates the settings window's full guidance.
		return fmt.Errorf("accessibility permission not granted")
	}
	time.Sleep(40 * time.Millisecond) // let the restored foreground app finish activating, matches sendPaste's identical wait on Windows
	C.sb_send_paste()
	return nil
}

// AccessibilityTrustedDirect reports whether SailBoard currently has Accessibility permission,
// without triggering the system's one-time permission dialog (see sb_accessibility_trusted's
// prompt:0 argument) — used by SettingsApp.IsAccessibilityTrusted to check and surface a notice
// proactively when the settings window opens, rather than only reactively after a failed paste
// (SendPaste above). A standalone function, not a Controller method, for the same reason as the
// other *Direct helpers: the settings window process has no Controller of its own.
//
// Checking from the settings window process specifically (rather than, say, having the long-
// running main process re-check and relay the result) is deliberate: AXIsProcessTrustedWithOptions
// appears to cache its answer for the querying process's lifetime on at least some macOS versions
// — a well-known quirk across many Accessibility-dependent Mac apps, not something unique to this
// one — so a grant made *after* the main process already asked once can go unnoticed by that same
// long-running process until it's relaunched, even though the OS-level grant is genuinely in
// effect. The settings window is a fresh, short-lived process every time it's opened (see main.go's
// runSettingsWindow), so its own check is never subject to that staleness — reopening settings
// after granting permission in System Settings always reflects the current, correct state.
func AccessibilityTrustedDirect() bool {
	return C.sb_accessibility_trusted(0) != 0
}
