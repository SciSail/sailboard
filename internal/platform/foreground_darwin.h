#ifndef SAILBOARD_FOREGROUND_DARWIN_H
#define SAILBOARD_FOREGROUND_DARWIN_H

// Returns the process identifier of the frontmost application, or 0 if none/unavailable — the
// macOS counterpart to captureForeground's GetForegroundWindow on Windows (see foreground_
// windows.go). A pid is used instead of any Cocoa object handle so the token can cross the cgo
// boundary as a plain int and be safely stashed in app.go's ForegroundToken for a while.
int sb_capture_foreground(void);

// Reactivates the application identified by pid (previously returned by sb_capture_foreground).
// No-op if pid is 0 or the process is no longer running.
void sb_restore_foreground(int pid);

// Reports whether this process is currently trusted for Accessibility — required before
// sb_send_paste's synthetic keyboard events actually reach other applications; without it,
// CGEventPost silently does nothing to anyone but this process. If not trusted and prompt is
// non-zero, this also triggers the system permission-request dialog, which adds SailBoard to
// System Settings > Privacy & Security > Accessibility (unchecked) for the user to enable.
int sb_accessibility_trusted(int prompt);

// Simulates Cmd+V via a synthetic keyboard event posted to the HID event tap — the macOS
// counterpart to sendPaste's SendInput on Windows (see foreground_windows.go). Requires this
// process to be Accessibility-trusted (see sb_accessibility_trusted); called regardless when
// trusted, since CGEventPost itself has no way to report "nobody was listening".
void sb_send_paste(void);

#endif
