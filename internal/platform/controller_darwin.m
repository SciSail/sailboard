#include "controller_darwin.h"
#import <Cocoa/Cocoa.h>

// AppKit calls must happen on the main thread. Every entry point below funnels its actual work
// through dispatch_sync onto the main queue so it's safe to call from any Go goroutine — mirrors
// winmsg_windows.go's runSync marshaling calls onto the Win32 message-loop thread for the exact
// same reason (RegisterHotKey/DestroyWindow there, NSWindow/NSApp mutation here).
static void sb_main_sync(void (^block)(void)) {
    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_sync(dispatch_get_main_queue(), block);
    }
}

// SailBoard only ever has (at most) two top-level windows — the main panel and, while open, the
// standalone settings window — so a linear scan by title on every call is cheap and needs no
// caching. This mirrors findWindowByTitle on Windows, which re-resolves the HWND by title on
// every call for the same reason (no Wails API exposes the native handle to cache).
static NSWindow *sb_find_window(NSString *title) {
    for (NSWindow *w in [NSApp windows]) {
        if ([w.title isEqualToString:title]) {
            return w;
        }
    }
    return nil;
}

sb_rect sb_work_area_near_cursor(int *ok) {
    __block sb_rect result = {0, 0, 0, 0};
    __block int found = 0;
    sb_main_sync(^{
        @autoreleasepool {
            NSArray<NSScreen *> *screens = [NSScreen screens];
            if (screens.count == 0) {
                return;
            }
            NSPoint mouseLoc = [NSEvent mouseLocation];
            NSScreen *target = nil;
            for (NSScreen *screen in screens) {
                if (NSPointInRect(mouseLoc, screen.frame)) {
                    target = screen;
                    break;
                }
            }
            if (!target) {
                target = screens[0];
            }
            // Cocoa's global desktop coordinate system is bottom-left origin, anchored to
            // screens[0] (the screen containing the menu bar). Flipping to top-left origin (this
            // package's Rect convention, shared with the Windows implementation) needs that
            // screen's height as the reference, not the target screen's own — a screen above/
            // below the primary one would otherwise flip around the wrong axis.
            CGFloat primaryHeight = screens[0].frame.size.height;
            NSRect visible = target.visibleFrame;
            result.x = visible.origin.x;
            result.y = primaryHeight - (visible.origin.y + visible.size.height);
            result.w = visible.size.width;
            result.h = visible.size.height;
            found = 1;
        }
    });
    *ok = found;
    return result;
}

sb_rect sb_get_frame_topleft(const char *title, int *ok) {
    __block sb_rect result = {0, 0, 0, 0};
    __block int found = 0;
    sb_main_sync(^{
        @autoreleasepool {
            NSWindow *win = sb_find_window([NSString stringWithUTF8String:title]);
            if (!win) {
                return;
            }
            NSArray<NSScreen *> *screens = [NSScreen screens];
            CGFloat primaryHeight = screens.count ? screens[0].frame.size.height : 0;
            NSRect f = win.frame;
            result.x = f.origin.x;
            result.y = primaryHeight - (f.origin.y + f.size.height);
            result.w = f.size.width;
            result.h = f.size.height;
            found = 1;
        }
    });
    *ok = found;
    return result;
}

void sb_set_frame_topleft(const char *title, double x, double y, double w, double h) {
    sb_main_sync(^{
        @autoreleasepool {
            NSWindow *win = sb_find_window([NSString stringWithUTF8String:title]);
            if (!win) {
                return;
            }
            NSArray<NSScreen *> *screens = [NSScreen screens];
            CGFloat primaryHeight = screens.count ? screens[0].frame.size.height : 0;
            CGFloat cocoaY = primaryHeight - (y + h);
            [win setFrame:NSMakeRect(x, cocoaY, w, h) display:YES];
        }
    });
}

void sb_show_no_activate(const char *title) {
    sb_main_sync(^{
        @autoreleasepool {
            NSWindow *win = sb_find_window([NSString stringWithUTF8String:title]);
            if (!win) {
                return;
            }
            [win orderFrontRegardless];
        }
    });
}

void sb_hide(const char *title) {
    sb_main_sync(^{
        @autoreleasepool {
            NSWindow *win = sb_find_window([NSString stringWithUTF8String:title]);
            if (!win) {
                return;
            }
            [win orderOut:nil];
        }
    });
}

void sb_focus_self(const char *title) {
    sb_main_sync(^{
        @autoreleasepool {
            [NSApp activateIgnoringOtherApps:YES];
            NSWindow *win = sb_find_window([NSString stringWithUTF8String:title]);
            if (!win) {
                return;
            }
            [win makeKeyAndOrderFront:nil];
        }
    });
}

int sb_app_is_active(void) {
    __block int active = 0;
    sb_main_sync(^{
        // [NSApp isActive] only reflects *this process's* activation state, but the settings
        // window (main.go's runSettingsWindow) is a second, independent OS process — not a second
        // window of this one — launched from the identical .app bundle. When it takes focus, this
        // process's own isActive genuinely goes false (a different process really did become
        // frontmost), which WatchFocusLoss's caller (controller_darwin.go) was relying on to mean
        // "the user switched to some other app" and hid the main panel — even though the user just
        // opened SailBoard's own settings. Comparing the system-wide frontmost application's bundle
        // identifier against our own catches "switched to our own settings window" correctly (both
        // processes share the same bundle ID, since they're the same .app launched twice), unlike
        // isActive which can't see past its own process boundary. Falls back to the old isActive
        // check when the bundle identifier is unavailable — e.g. an unbundled `wails dev`/debug
        // binary launched straight from the terminal, which has no real Info.plist to read.
        NSString *ownBundleID = [[NSBundle mainBundle] bundleIdentifier];
        NSRunningApplication *front = [[NSWorkspace sharedWorkspace] frontmostApplication];
        if (ownBundleID != nil && front != nil && [front.bundleIdentifier isEqualToString:ownBundleID]) {
            active = 1;
        } else if (ownBundleID == nil) {
            active = [NSApp isActive] ? 1 : 0;
        } else {
            active = 0;
        }
    });
    return active;
}

void sb_set_activation_policy_accessory(void) {
    sb_main_sync(^{
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
    });
}
