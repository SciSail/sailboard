#include "activeapp_darwin.h"
#import <Cocoa/Cocoa.h>
#include <string.h>

static void sb_main_sync_activeapp(void (^block)(void)) {
    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_sync(dispatch_get_main_queue(), block);
    }
}

static char *sb_strdup(NSString *s) {
    if (!s) {
        return NULL;
    }
    const char *utf8 = [s UTF8String];
    if (!utf8) {
        return NULL;
    }
    size_t n = strlen(utf8) + 1;
    char *buf = malloc(n);
    if (!buf) {
        return NULL;
    }
    memcpy(buf, utf8, n);
    return buf;
}

void sb_active_app(char **outName, char **outPath, unsigned char **outIconData, long *outIconLen, int *ok) {
    *outName = NULL;
    *outPath = NULL;
    *outIconData = NULL;
    *outIconLen = 0;
    *ok = 0;
    // NSRunningApplication's icon property triggers actual drawing when resized below, so this
    // whole read is marshaled onto the main thread like the window-management calls in
    // controller_darwin.m, not left unguarded like clipboard_darwin.m's pure data reads.
    sb_main_sync_activeapp(^{
        @autoreleasepool {
            NSRunningApplication *app = [[NSWorkspace sharedWorkspace] frontmostApplication];
            if (!app) {
                return;
            }
            *outName = sb_strdup(app.localizedName);
            *outPath = sb_strdup(app.bundleURL.path);

            NSImage *icon = app.icon;
            if (icon) {
                NSSize targetSize = NSMakeSize(64, 64);
                NSImage *resized = [[NSImage alloc] initWithSize:targetSize];
                [resized lockFocus];
                [icon drawInRect:NSMakeRect(0, 0, targetSize.width, targetSize.height)
                         fromRect:NSZeroRect
                        operation:NSCompositingOperationSourceOver
                         fraction:1.0];
                [resized unlockFocus];
                NSData *tiff = [resized TIFFRepresentation];
                NSBitmapImageRep *rep = tiff ? [NSBitmapImageRep imageRepWithData:tiff] : nil;
                NSData *png = rep ? [rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}] : nil;
                if (png && png.length > 0) {
                    void *buf = malloc(png.length);
                    if (buf) {
                        memcpy(buf, png.bytes, png.length);
                        *outIconData = (unsigned char *)buf;
                        *outIconLen = (long)png.length;
                    }
                }
            }
            *ok = 1;
        }
    });
}
