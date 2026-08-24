#ifndef SAILBOARD_QUICKLOOK_DARWIN_H
#define SAILBOARD_QUICKLOOK_DARWIN_H

// Toggles the shared QLPreviewPanel — the same Quick Look panel Finder/Mail show on Space — for
// the given file paths. If the panel is already visible, orders it out and returns 0 (mirrors the
// system-wide spacebar-to-close behaviour) regardless of paths. Otherwise loads paths as the
// panel's preview items (any that don't exist on disk are skipped), brings it to front above
// SailBoard's own floating-level main window, and returns 1. Returns 0 if none of paths exist.
int sb_quicklook_toggle(char **paths, int count);

#endif
