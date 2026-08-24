#ifndef SAILBOARD_IPC_DARWIN_H
#define SAILBOARD_IPC_DARWIN_H

// Registers this process as a listener for the two distributed notifications SailBoard uses for
// cross-process signaling between the main window process and the standalone settings window
// process (see ipc_windows.go's hidden message-only window + PostMessage for the Windows
// equivalent — mac has no window/message-loop link between unrelated processes to hang a message
// on, so NSDistributedNotificationCenter, the standard system-wide process-to-process broadcast
// mechanism, stands in for it instead). Firing invokes the exported Go functions
// sbSettingsChangedNotification/sbShowMainNotification (ipc_darwin.go). Idempotent — safe to call
// more than once; only the first call actually registers anything.
void sb_watch_distributed_notifications(void);

// Posts the "settings changed" / "show main window" distributed notifications, observed by
// sb_watch_distributed_notifications in the main window process, if one is running. No-op (safe,
// finds nothing) if no main process is listening.
void sb_notify_settings_changed(void);
void sb_notify_show_main(void);

// Posts the "suspend hotkey" / "resume hotkey" distributed notifications — the macOS counterpart
// to ipc_windows.go's SuspendHotkeyDirect/ResumeHotkeyDirect (see Controller.
// OnHotkeySuspendRequested's doc comment in types.go for why the settings window needs these
// while capturing a new shortcut). Observed by the same sb_watch_distributed_notifications
// registration as the two above. No-op (safe, finds nothing) if no main process is listening.
void sb_notify_suspend_hotkey(void);
void sb_notify_resume_hotkey(void);

// Reports whether a window titled title — belonging to any process, not just this one — exists
// on-screen system-wide, and if so, activates its owning application and reports 1. Used to avoid
// opening a duplicate settings window: the settings window is a separate OS process, so
// controller_darwin.m's sb_find_window (which only sees this process's own windows via [NSApp
// windows]) can't see it — this uses CGWindowListCopyWindowInfo's system-wide window list
// instead, the mac counterpart to focusIfExists's plain (system-wide by default) FindWindow on
// Windows.
int sb_focus_if_exists(const char *title);

#endif
