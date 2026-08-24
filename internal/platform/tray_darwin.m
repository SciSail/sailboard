#include "tray_darwin.h"
#import <Cocoa/Cocoa.h>
#include "_cgo_export.h"

// SBTrayTarget's three action methods are the only way to route an NSMenuItem click back into
// Go — Objective-C blocks can't be assigned as a menu item's target/action, only a real object +
// selector pair (the classic target-action pattern).
@interface SBTrayTarget : NSObject
- (void)onShow:(id)sender;
- (void)onToggle:(id)sender;
- (void)onQuit:(id)sender;
@end

@implementation SBTrayTarget
- (void)onShow:(id)sender {
    sbTrayShowClicked();
}
- (void)onToggle:(id)sender {
    sbTrayToggleClicked();
}
- (void)onQuit:(id)sender {
    sbTrayQuitClicked();
}
@end

static NSStatusItem *sStatusItem = nil;
static SBTrayTarget *sTrayTarget = nil;
static NSMenuItem *sToggleItem = nil;

static void sb_tray_main(void (^block)(void)) {
    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_sync(dispatch_get_main_queue(), block);
    }
}

void sb_show_tray(const unsigned char *iconData, long iconLen) {
    sb_tray_main(^{
        @autoreleasepool {
            if (!sTrayTarget) {
                sTrayTarget = [SBTrayTarget new];
            }
            if (!sStatusItem) {
                // statusItemWithLength: is a convenience constructor (not alloc/new/copy-
                // prefixed), so it returns an autoreleased object under this file's manual
                // reference counting (no -fobjc-arc) — without this retain, sStatusItem would be
                // deallocated (and vanish from the menu bar) the instant this @autoreleasepool
                // block exits, which is exactly why the icon never appeared before this fix.
                sStatusItem = [[[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength] retain];
                NSMenu *menu = [[NSMenu alloc] init];

                NSMenuItem *showItem = [menu addItemWithTitle:@"显示 SailBoard" action:@selector(onShow:) keyEquivalent:@""];
                showItem.target = sTrayTarget;

                sToggleItem = [menu addItemWithTitle:@"暂停记录" action:@selector(onToggle:) keyEquivalent:@""];
                sToggleItem.target = sTrayTarget;

                [menu addItem:[NSMenuItem separatorItem]];

                NSMenuItem *quitItem = [menu addItemWithTitle:@"退出" action:@selector(onQuit:) keyEquivalent:@""];
                quitItem.target = sTrayTarget;

                sStatusItem.menu = menu;
            }
            if (iconData != NULL && iconLen > 0) {
                NSData *data = [NSData dataWithBytes:iconData length:(NSUInteger)iconLen];
                NSImage *image = [[NSImage alloc] initWithData:data];
                if (image) {
                    image.size = NSMakeSize(18, 18);
                    sStatusItem.button.image = image;
                } else {
                    sStatusItem.button.title = @"SB";
                }
            } else {
                sStatusItem.button.title = @"SB";
            }
        }
    });
}

void sb_update_tray_paused(int paused) {
    sb_tray_main(^{
        if (!sToggleItem) {
            return;
        }
        sToggleItem.title = paused ? @"恢复记录" : @"暂停记录";
    });
}
