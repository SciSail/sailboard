#ifndef SAILBOARD_CLIPBOARD_FILES_DARWIN_H
#define SAILBOARD_CLIPBOARD_FILES_DARWIN_H

// Returns the file:// URLs currently on the clipboard (a Finder copy of file(s)/folder(s)), as
// filesystem paths. *outPaths is a malloc'd array of malloc'd NUL-terminated UTF-8 C strings;
// *outCount is 0 (and *outPaths NULL) if nothing's there. Caller frees with
// sb_free_string_array. Reading file:// URLs specifically (not the broader "any object NSImage/
// NSURL can synthesize from the pasteboard") is what keeps this from misclassifying things — see
// clipboard_darwin.m's doc comment on why ReadClipboardImage deliberately checks only the raw
// TIFF representation rather than the same permissive NSImage-from-pasteboard reader.
void sb_read_clipboard_files(char ***outPaths, int *outCount);

// Frees the array sb_read_clipboard_files returned.
void sb_free_string_array(char **paths, int count);

// Writes paths back onto the clipboard as file:// URLs, so Finder (or any app) can paste/drag
// them as real file references — mirrors writeClipboardFiles's CF_HDROP write on Windows.
void sb_write_clipboard_files(char **paths, int count);

#endif
