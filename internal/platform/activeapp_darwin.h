#ifndef SAILBOARD_ACTIVEAPP_DARWIN_H
#define SAILBOARD_ACTIVEAPP_DARWIN_H

// Reads the frontmost application's display name, .app bundle path, and a PNG-encoded icon
// (downscaled to 64x64, matching the Windows implementation's large-icon choice — see
// activeapp_windows.go's extractAppIconPNG doc comment for why not the smaller 16px icon).
// outName/outPath/outIconData are malloc'd (NUL-terminated for the two strings); caller frees
// each with sb_free (clipboard_darwin.h). Any of the three may come back NULL independently (e.g.
// an app with no name, or no icon) without *ok being 0 — *ok is only 0 if there's no frontmost
// application at all.
void sb_active_app(char **outName, char **outPath, unsigned char **outIconData, long *outIconLen, int *ok);

#endif
