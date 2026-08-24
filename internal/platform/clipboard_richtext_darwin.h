#ifndef SAILBOARD_CLIPBOARD_RICHTEXT_DARWIN_H
#define SAILBOARD_CLIPBOARD_RICHTEXT_DARWIN_H

// Reads NSPasteboardTypeHTML/NSPasteboardTypeRTF alongside NSPasteboardTypeString, mirroring the
// "genuine formatted copy" heuristic already used on Windows (see clipboard_richtext_windows.go's
// readClipboardRichText doc comment): *ok is 1 only when there's non-empty plain text *and* at
// least one rich format present — that combination is what a real Office/browser formatted-text
// copy always produces, as opposed to e.g. a bare image copy that happens to carry incidental
// markup with no real text alongside it. outHTML/outRTF/outText are malloc'd, NUL-terminated
// UTF-8 (caller frees each with sb_free, clipboard_darwin.h); outHTML/outRTF may independently
// come back NULL even when *ok is 1 (e.g. only one of the two rich formats present).
void sb_read_clipboard_richtext(char **outHTML, char **outRTF, char **outText, int *ok);

// Writes html and/or rtf (whichever is non-empty) back onto the clipboard alongside text as plain
// NSPasteboardTypeString, so a paste into an app that understands HTML/RTF keeps formatting while
// a plain-text-only app falls back transparently — mirrors writeClipboardRichText on Windows.
void sb_write_clipboard_richtext(const char *html, const char *rtf, const char *text);

#endif
