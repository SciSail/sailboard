#include "file_thumbnail_darwin.h"
#import <Cocoa/Cocoa.h>

static void sb_ft_main(void (^block)(void)) {
    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_sync(dispatch_get_main_queue(), block);
    }
}

void sb_file_icon(const char *path, int size, unsigned char **outData, long *outLen, int *ok) {
    *outData = NULL;
    *outLen = 0;
    *ok = 0;
    sb_ft_main(^{
        @autoreleasepool {
            NSString *nsPath = [NSString stringWithUTF8String:path];
            if (!nsPath) {
                return;
            }
            NSImage *icon = [[NSWorkspace sharedWorkspace] iconForFile:nsPath];
            if (!icon) {
                return;
            }

            NSSize targetSize = NSMakeSize(size, size);
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
            if (!png || png.length == 0) {
                return;
            }

            void *buf = malloc(png.length);
            if (!buf) {
                return;
            }
            memcpy(buf, png.bytes, png.length);
            *outData = (unsigned char *)buf;
            *outLen = (long)png.length;
            *ok = 1;
        }
    });
}
