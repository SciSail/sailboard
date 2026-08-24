//go:build windows

package platform

import (
	"sync"
	"syscall"
)

// focusWatchState is package-level (not tied to a *msgWindow instance) because the WinEvent
// callback below is a bare C-callable trampoline with a fixed signature — it has no way to
// receive extra context per call, so the title/callback it needs are stashed here instead.
// SailBoard only ever watches its own single window, so one shared instance is sufficient.
var focusWatchState struct {
	mu          sync.Mutex
	titles      []string
	wasSelf     bool
	onLostFocus func()
}

var winEventProcCallback = syscall.NewCallback(func(hWinEventHook, event, hwnd, idObject, idChild, idEventThread, dwmsEventTime uintptr) uintptr {
	if uint32(event) != eventSystemForeground {
		return 0
	}
	focusWatchState.mu.Lock()
	titles := focusWatchState.titles
	wasSelf := focusWatchState.wasSelf
	cb := focusWatchState.onLostFocus
	focusWatchState.mu.Unlock()
	if len(titles) == 0 {
		return 0
	}

	isSelf := false
	if hwnd != 0 {
		for _, title := range titles {
			if hwnd == findWindowByTitle(title) {
				isSelf = true
				break
			}
		}
	}

	focusWatchState.mu.Lock()
	focusWatchState.wasSelf = isSelf
	focusWatchState.mu.Unlock()

	// Only fire on the transition *away* from our own window, never on unrelated foreground
	// changes among other apps (which fire constantly during normal desktop use) and never
	// spuriously while we were never the foreground window to begin with.
	if wasSelf && !isSelf && cb != nil {
		go cb()
	}
	return 0
})

// markSelfFocused tells the focus-loss watcher "we just successfully focused one of our own
// windows" directly, rather than waiting to observe it back through the WinEventHook above.
// focusSelf calls this right after SetForegroundWindow — needed because, unlike a real user
// click, that synthetic call doesn't reliably produce a same-tick EVENT_SYSTEM_FOREGROUND the
// hook can catch in time (most visible at app startup: nothing else has genuine foreground focus
// to AttachThreadInput onto yet, so the hook's wasSelf can stay stuck false even though the panel
// is genuinely showing — the bug this fixes was "click away doesn't auto-hide until you first
// click the panel itself", i.e. wasSelf silently never having gone true).
func markSelfFocused() {
	focusWatchState.mu.Lock()
	focusWatchState.wasSelf = true
	focusWatchState.mu.Unlock()
}

// watchFocusLoss installs a system-wide foreground-window-change hook (via SetWinEventHook) and
// calls onLostFocus exactly when the foreground window transitions away from every window titled
// by one of titles. It must be installed from a thread that pumps a standard Win32 message loop
// for the out-of-context callback to actually fire, so this runs via win.runSync onto the same
// message-loop thread that owns hotkey/tray delivery (see winmsg_windows.go).
func watchFocusLoss(win *msgWindow, titles []string, onLostFocus func()) {
	focusWatchState.mu.Lock()
	focusWatchState.titles = titles
	focusWatchState.onLostFocus = onLostFocus
	focusWatchState.wasSelf = false
	focusWatchState.mu.Unlock()

	win.runSync(func() {
		procSetWinEventHook.Call(
			eventSystemForeground, eventSystemForeground,
			0, // hmodWinEventProc: unused for WINEVENT_OUTOFCONTEXT hooks
			winEventProcCallback,
			0, 0, // idProcess, idThread = 0: watch system-wide, not one process/thread
			winEventOutOfContext,
		)
	})
}
