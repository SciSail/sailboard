#ifndef SAILBOARD_CONTROLLER_DARWIN_H
#define SAILBOARD_CONTROLLER_DARWIN_H

// A screen-space rectangle in pixels, top-left origin — matches platform.Rect's convention
// (shared with the Windows implementation) rather than Cocoa's native bottom-left-origin
// NSRect, so Go-side code never has to think about the two coordinate systems.
typedef struct {
    double x, y, w, h;
} sb_rect;

// Returns the work area (menu bar/Dock excluded) of the screen under the mouse cursor, falling
// back to the primary screen if none contains the cursor. *ok is set to 0 if no screen could be
// found at all (e.g. called before NSApp has any screens, which shouldn't happen in practice).
sb_rect sb_work_area_near_cursor(int *ok);

// Returns the current frame of the window titled title, top-left origin. *ok is 0 if no such
// window exists.
sb_rect sb_get_frame_topleft(const char *title, int *ok);

// Moves/resizes the window titled title to the given top-left-origin rectangle, without changing
// activation or key-window state — mirrors positionSelf's SetWindowPos(SWP_NOACTIVATE) on
// Windows. No-op if no such window exists.
void sb_set_frame_topleft(const char *title, double x, double y, double w, double h);

// Makes the window titled title visible (ordered front) without activating the app or stealing
// key focus — the counterpart to Windows' ShowWindow(SW_SHOW) in slideReveal. No-op if no such
// window exists.
void sb_show_no_activate(const char *title);

// Hides the window titled title (orderOut) without closing it. No-op if no such window exists.
void sb_hide(const char *title);

// Activates SailBoard and makes the window titled title key, forcing OS keyboard focus onto it —
// the Cocoa counterpart to focusSelf's AttachThreadInput workaround on Windows (window shown from
// a background thread doesn't reliably receive focus on its own). No-op if no such window exists.
void sb_focus_self(const char *title);

// Reports whether SailBoard — this process or, since it shares the same bundle identifier, the
// standalone settings window process (main.go's runSettingsWindow) — is the frontmost app. See
// controller_darwin.m's doc comment for why this isn't simply [NSApp isActive].
int sb_app_is_active(void);

// Forces the app's activation policy to Accessory (no Dock icon, no Cmd+Tab entry, menu-bar-only
// presence) — see controller_darwin.go's New() doc comment for why this has to be called from Go
// at all: Wails' own AppDelegate.m unconditionally sets Regular in applicationWillFinishLaunching,
// which runs before our own startup code and silently overrides Info.plist's LSUIElement key no
// matter what it says.
void sb_set_activation_policy_accessory(void);

#endif
