#include "quicklook_darwin.h"
#import <Cocoa/Cocoa.h>
#import <Quartz/Quartz.h>

// QLPreviewPanel drives its data source by re-querying it (numberOfPreviewItemsInPreviewPanel:/
// previewItemAtIndex:) rather than taking a one-shot array, so the items it's currently showing
// have to be held somewhere that outlives this function call — a static, same as
// sbQuickLookDataSource below.
static NSArray<NSURL *> *sbQuickLookItems = nil;

@interface SBQuickLookDataSource : NSObject <QLPreviewPanelDataSource>
@end

@implementation SBQuickLookDataSource
- (NSInteger)numberOfPreviewItemsInPreviewPanel:(QLPreviewPanel *)panel {
    return sbQuickLookItems.count;
}
- (id<QLPreviewItem>)previewPanel:(QLPreviewPanel *)panel previewItemAtIndex:(NSInteger)index {
    return sbQuickLookItems[index];
}
@end

// alloc/init already retains this (unlike tray_darwin.m's convenience-constructed status item —
// see that file's comment on the distinction), but it's kept in a static and reused across calls
// anyway so the panel's dataSource identity doesn't churn on every toggle.
static SBQuickLookDataSource *sbQuickLookDataSource = nil;

int sb_quicklook_toggle(char **paths, int count) {
    __block int shown = 0;
    void (^work)(void) = ^{
        @autoreleasepool {
            QLPreviewPanel *panel = [QLPreviewPanel sharedPreviewPanel];
            if (panel.isVisible) {
                [panel orderOut:nil];
                return;
            }
            NSMutableArray<NSURL *> *items = [NSMutableArray arrayWithCapacity:count];
            for (int i = 0; i < count; i++) {
                NSString *path = [NSString stringWithUTF8String:paths[i]];
                if (!path || ![[NSFileManager defaultManager] fileExistsAtPath:path]) {
                    continue;
                }
                [items addObject:[NSURL fileURLWithPath:path]];
            }
            if (items.count == 0) {
                return;
            }
            sbQuickLookItems = [items copy];
            if (!sbQuickLookDataSource) {
                sbQuickLookDataSource = [[SBQuickLookDataSource alloc] init];
            }
            panel.dataSource = sbQuickLookDataSource;
            // main.go's AlwaysOnTop pins SailBoard's own window at NSFloatingWindowLevel (see
            // wails' AppDelegate.m). Without raising the preview panel above that too, it would
            // render behind SailBoard's own always-on-top panel instead of on top of it.
            panel.level = NSFloatingWindowLevel + 1;
            [panel reloadData];
            [NSApp activateIgnoringOtherApps:YES];
            [panel makeKeyAndOrderFront:nil];
            shown = 1;
        }
    };
    if ([NSThread isMainThread]) {
        work();
    } else {
        dispatch_sync(dispatch_get_main_queue(), work);
    }
    return shown;
}
