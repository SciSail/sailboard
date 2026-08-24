#include "foreground_darwin.h"
#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>

static void sb_fg_main(void (^block)(void)) {
    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_sync(dispatch_get_main_queue(), block);
    }
}

int sb_capture_foreground(void) {
    __block int pid = 0;
    sb_fg_main(^{
        @autoreleasepool {
            NSRunningApplication *app = [[NSWorkspace sharedWorkspace] frontmostApplication];
            if (app) {
                pid = (int)app.processIdentifier;
            }
        }
    });
    return pid;
}

void sb_restore_foreground(int pid) {
    if (pid == 0) {
        return;
    }
    sb_fg_main(^{
        @autoreleasepool {
            NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:(pid_t)pid];
            if (app) {
                [app activateWithOptions:NSApplicationActivateAllWindows];
            }
        }
    });
}

int sb_accessibility_trusted(int prompt) {
    NSDictionary *opts = @{(__bridge id)kAXTrustedCheckOptionPrompt : (prompt ? @YES : @NO)};
    return AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)opts) ? 1 : 0;
}

// kVK_Command = 0x37, kVK_ANSI_V = 0x09 — same HIToolbox virtual-keycode constants as hotkey_
// darwin.go's darwinVKByLetter['V'].
void sb_send_paste(void) {
    CGEventSourceRef source = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
    CGEventRef cmdDown = CGEventCreateKeyboardEvent(source, 0x37, true);
    CGEventRef vDown = CGEventCreateKeyboardEvent(source, 0x09, true);
    CGEventRef vUp = CGEventCreateKeyboardEvent(source, 0x09, false);
    CGEventRef cmdUp = CGEventCreateKeyboardEvent(source, 0x37, false);
    CGEventSetFlags(vDown, kCGEventFlagMaskCommand);
    CGEventSetFlags(vUp, kCGEventFlagMaskCommand);

    CGEventPost(kCGHIDEventTap, cmdDown);
    CGEventPost(kCGHIDEventTap, vDown);
    CGEventPost(kCGHIDEventTap, vUp);
    CGEventPost(kCGHIDEventTap, cmdUp);

    CFRelease(cmdDown);
    CFRelease(vDown);
    CFRelease(vUp);
    CFRelease(cmdUp);
    CFRelease(source);
}
