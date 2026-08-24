#include "clipboard_darwin.h"
#import <Cocoa/Cocoa.h>
#include <string.h>

// NSPasteboard reads are documented safe to call from any single thread (just not concurrently
// from multiple threads at once) — unlike NSWindow/NSApp, no main-thread marshaling needed here.
// The watcher (internal/clipboard/watcher.go) already calls every Read* function sequentially
// from one goroutine, so there's no concurrency to guard against.

void sb_read_clipboard_image(unsigned char **outData, long *outLen, int *outWidth, int *outHeight, int *ok) {
    *outData = NULL;
    *outLen = 0;
    *outWidth = 0;
    *outHeight = 0;
    *ok = 0;
    @autoreleasepool {
        NSPasteboard *pb = [NSPasteboard generalPasteboard];
        NSData *tiff = [pb dataForType:NSPasteboardTypeTIFF];
        if (!tiff) {
            return;
        }
        NSBitmapImageRep *rep = [NSBitmapImageRep imageRepWithData:tiff];
        if (!rep) {
            return;
        }
        NSData *png = [rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
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
        *outWidth = (int)rep.pixelsWide;
        *outHeight = (int)rep.pixelsHigh;
        *ok = 1;
    }
}

void sb_free(void *ptr) {
    free(ptr);
}

int sb_write_clipboard_image(const unsigned char *data, long len) {
    if (!data || len <= 0) {
        return 0;
    }
    @autoreleasepool {
        NSData *pngData = [NSData dataWithBytes:data length:(NSUInteger)len];
        NSImage *image = [[NSImage alloc] initWithData:pngData];
        if (!image) {
            return 0;
        }
        NSData *tiff = [image TIFFRepresentation];
        if (!tiff) {
            return 0;
        }
        NSPasteboard *pb = [NSPasteboard generalPasteboard];
        [pb clearContents];
        BOOL ok = [pb setData:tiff forType:NSPasteboardTypeTIFF];
        return ok ? 1 : 0;
    }
}

unsigned int sb_clipboard_change_count(void) {
    return (unsigned int)[[NSPasteboard generalPasteboard] changeCount];
}
