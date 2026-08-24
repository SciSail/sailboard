#include "clipboard_files_darwin.h"
#import <Cocoa/Cocoa.h>
#include <string.h>

void sb_read_clipboard_files(char ***outPaths, int *outCount) {
    *outPaths = NULL;
    *outCount = 0;
    @autoreleasepool {
        NSPasteboard *pb = [NSPasteboard generalPasteboard];
        NSDictionary *options = @{NSPasteboardURLReadingFileURLsOnlyKey : @YES};
        NSArray<NSURL *> *urls = [pb readObjectsForClasses:@[ [NSURL class] ] options:options];
        if (urls.count == 0) {
            return;
        }

        char **arr = malloc(sizeof(char *) * urls.count);
        if (!arr) {
            return;
        }
        NSUInteger n = 0;
        for (NSURL *url in urls) {
            NSString *path = url.path;
            const char *utf8 = path ? [path UTF8String] : NULL;
            if (!utf8) {
                continue;
            }
            size_t len = strlen(utf8) + 1;
            char *buf = malloc(len);
            if (!buf) {
                continue;
            }
            memcpy(buf, utf8, len);
            arr[n++] = buf;
        }
        if (n == 0) {
            free(arr);
            return;
        }
        *outPaths = arr;
        *outCount = (int)n;
    }
}

void sb_free_string_array(char **paths, int count) {
    if (!paths) {
        return;
    }
    for (int i = 0; i < count; i++) {
        free(paths[i]);
    }
    free(paths);
}

void sb_write_clipboard_files(char **paths, int count) {
    if (count <= 0) {
        return;
    }
    @autoreleasepool {
        NSMutableArray<NSURL *> *urls = [NSMutableArray arrayWithCapacity:count];
        for (int i = 0; i < count; i++) {
            NSString *path = [NSString stringWithUTF8String:paths[i]];
            if (!path) {
                continue;
            }
            NSURL *url = [NSURL fileURLWithPath:path];
            if (url) {
                [urls addObject:url];
            }
        }
        if (urls.count == 0) {
            return;
        }
        NSPasteboard *pb = [NSPasteboard generalPasteboard];
        [pb clearContents];
        [pb writeObjects:urls];
    }
}
