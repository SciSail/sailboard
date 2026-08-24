#ifndef SAILBOARD_CLIPBOARD_DARWIN_H
#define SAILBOARD_CLIPBOARD_DARWIN_H

// Reads the current clipboard image (from the classic NSPasteboardTypeTIFF representation, which
// every Mac app that puts an image on the pasteboard provides for compatibility) and re-encodes
// it as PNG into malloc'd memory the caller must free with sb_free. *ok is 0 if there's no image
// data on the pasteboard — deliberately checking only the raw TIFF/PNG representations rather
// than the more permissive NSImage-from-pasteboard reader, which can also synthesize an image
// from a plain file:// URL (e.g. a Finder copy of an image *file*, which should be a file
// reference, not inline image bytes — see clipboard_darwin.m's doc comment).
void sb_read_clipboard_image(unsigned char **outData, long *outLen, int *outWidth, int *outHeight, int *ok);

// Frees memory sb_read_clipboard_image (or any other sb_* out-param) allocated via malloc.
void sb_free(void *ptr);

// Writes a PNG-encoded image back to the system clipboard (as the classic TIFF representation,
// matching what sb_read_clipboard_image checks for), so a pasted history item behaves like a
// fresh copy. Returns 1 on success, 0 if data isn't a decodable image or the pasteboard write
// failed.
int sb_write_clipboard_image(const unsigned char *data, long len);

// NSPasteboard.changeCount: a monotonically increasing counter that changes every time the
// clipboard content changes, letting the watcher skip a full read on unchanged ticks.
unsigned int sb_clipboard_change_count(void);

#endif
