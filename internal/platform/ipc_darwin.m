#include "ipc_darwin.h"
#import <Cocoa/Cocoa.h>
#import <CoreGraphics/CoreGraphics.h>
#include "_cgo_export.h"

static NSString *const kSailBoardSettingsChangedName = @"com.wails.SailBoard.SettingsChanged";
static NSString *const kSailBoardShowMainName = @"com.wails.SailBoard.ShowMain";
static NSString *const kSailBoardSuspendHotkeyName = @"com.wails.SailBoard.SuspendHotkey";
static NSString *const kSailBoardResumeHotkeyName = @"com.wails.SailBoard.ResumeHotkey";

static BOOL sObservingDistributedNotifications = NO;

void sb_watch_distributed_notifications(void) {
    if (sObservingDistributedNotifications) {
        return;
    }
    sObservingDistributedNotifications = YES;

    NSDistributedNotificationCenter *center = [NSDistributedNotificationCenter defaultCenter];
    // queue:[NSOperationQueue mainQueue] means these blocks always run on the main thread, still
    // inside a native callback's own stack — same hazard class as sbHotkeyFired/sbTray*Clicked,
    // so both exported functions below route through darwinMainThreadCallbacks rather than
    // running their handler in-place (see hotkey_darwin.go's sbHotkeyFired doc comment for the
    // full SIGSEGV diagnosis this is avoiding).
    [center addObserverForName:kSailBoardSettingsChangedName
                         object:nil
                          queue:[NSOperationQueue mainQueue]
                     usingBlock:^(NSNotification *note) {
                       sbSettingsChangedNotification();
                     }];
    [center addObserverForName:kSailBoardShowMainName
                         object:nil
                          queue:[NSOperationQueue mainQueue]
                     usingBlock:^(NSNotification *note) {
                       sbShowMainNotification();
                     }];
    [center addObserverForName:kSailBoardSuspendHotkeyName
                         object:nil
                          queue:[NSOperationQueue mainQueue]
                     usingBlock:^(NSNotification *note) {
                       sbSuspendHotkeyNotification();
                     }];
    [center addObserverForName:kSailBoardResumeHotkeyName
                         object:nil
                          queue:[NSOperationQueue mainQueue]
                     usingBlock:^(NSNotification *note) {
                       sbResumeHotkeyNotification();
                     }];
}

void sb_notify_settings_changed(void) {
    [[NSDistributedNotificationCenter defaultCenter] postNotificationName:kSailBoardSettingsChangedName
                                                                     object:nil
                                                                   userInfo:nil
                                                         deliverImmediately:YES];
}

void sb_notify_show_main(void) {
    [[NSDistributedNotificationCenter defaultCenter] postNotificationName:kSailBoardShowMainName
                                                                     object:nil
                                                                   userInfo:nil
                                                         deliverImmediately:YES];
}

void sb_notify_suspend_hotkey(void) {
    [[NSDistributedNotificationCenter defaultCenter] postNotificationName:kSailBoardSuspendHotkeyName
                                                                     object:nil
                                                                   userInfo:nil
                                                         deliverImmediately:YES];
}

void sb_notify_resume_hotkey(void) {
    [[NSDistributedNotificationCenter defaultCenter] postNotificationName:kSailBoardResumeHotkeyName
                                                                     object:nil
                                                                   userInfo:nil
                                                         deliverImmediately:YES];
}

static void sb_ipc_main(void (^block)(void)) {
    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_sync(dispatch_get_main_queue(), block);
    }
}

int sb_focus_if_exists(const char *title) {
    __block int found = 0;
    sb_ipc_main(^{
        @autoreleasepool {
            NSString *nsTitle = [NSString stringWithUTF8String:title];
            CFArrayRef windowList = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly, kCGNullWindowID);
            NSArray *windows = (__bridge NSArray *)windowList;
            for (NSDictionary *info in windows) {
                NSString *name = info[(id)kCGWindowName];
                if (!name || ![name isEqualToString:nsTitle]) {
                    continue;
                }
                NSNumber *pidNum = info[(id)kCGWindowOwnerPID];
                if (pidNum) {
                    NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:(pid_t)pidNum.intValue];
                    if (app) {
                        [app activateWithOptions:NSApplicationActivateAllWindows];
                        found = 1;
                    }
                }
                break;
            }
            CFRelease(windowList);
        }
    });
    return found;
}
