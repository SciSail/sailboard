#include "clipboard_richtext_darwin.h"
#import <Cocoa/Cocoa.h>
#include <string.h>

// Same decode-with-fallback approach as the rest of this package's text-carrying pasteboard
// reads: HTML/RTF clipboard fragments are UTF-8 in the overwhelming majority of real-world Office/
// browser copies, but fall back to Latin-1 (which never fails to decode any byte sequence) rather
// than silently dropping the content for the rare non-UTF-8 fragment.
static NSString *sb_rt_decode(NSData *data) {
    if (!data) {
        return nil;
    }
    NSString *s = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
    if (!s) {
        s = [[NSString alloc] initWithData:data encoding:NSISOLatin1StringEncoding];
    }
    return s;
}

static char *sb_rt_strdup(NSString *s) {
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

void sb_read_clipboard_richtext(char **outHTML, char **outRTF, char **outText, int *ok) {
    *outHTML = NULL;
    *outRTF = NULL;
    *outText = NULL;
    *ok = 0;
    @autoreleasepool {
        NSPasteboard *pb = [NSPasteboard generalPasteboard];
        NSString *html = sb_rt_decode([pb dataForType:NSPasteboardTypeHTML]);
        NSString *rtf = sb_rt_decode([pb dataForType:NSPasteboardTypeRTF]);
        NSString *text = [pb stringForType:NSPasteboardTypeString];

        NSString *trimmed = [text stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceAndNewlineCharacterSet]];
        BOOL hasText = trimmed.length > 0;
        BOOL hasRich = html.length > 0 || rtf.length > 0;
        if (!hasText || !hasRich) {
            return;
        }

        *outHTML = sb_rt_strdup(html);
        *outRTF = sb_rt_strdup(rtf);
        *outText = sb_rt_strdup(text);
        *ok = 1;
    }
}

void sb_write_clipboard_richtext(const char *html, const char *rtf, const char *text) {
    @autoreleasepool {
        NSPasteboard *pb = [NSPasteboard generalPasteboard];
        [pb clearContents];

        NSMutableArray<NSPasteboardType> *types = [NSMutableArray arrayWithObject:NSPasteboardTypeString];
        if (html && strlen(html) > 0) {
            [types addObject:NSPasteboardTypeHTML];
        }
        if (rtf && strlen(rtf) > 0) {
            [types addObject:NSPasteboardTypeRTF];
        }
        [pb declareTypes:types owner:nil];

        if (text) {
            [pb setString:[NSString stringWithUTF8String:text] forType:NSPasteboardTypeString];
        }
        if (html && strlen(html) > 0) {
            [pb setString:[NSString stringWithUTF8String:html] forType:NSPasteboardTypeHTML];
        }
        if (rtf && strlen(rtf) > 0) {
            [pb setString:[NSString stringWithUTF8String:rtf] forType:NSPasteboardTypeRTF];
        }
    }
}
