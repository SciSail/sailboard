#ifndef SAILBOARD_TRAY_DARWIN_H
#define SAILBOARD_TRAY_DARWIN_H

// Creates the menu bar status item on first call (later calls just update its icon), using
// iconData/iconLen as a PNG-encoded icon. Menu item clicks invoke the exported Go functions
// sbTrayShowClicked/sbTrayToggleClicked/sbTrayQuitClicked (see tray_darwin.go).
void sb_show_tray(const unsigned char *iconData, long iconLen);

// Updates the pause/resume menu item's label to reflect paused. No-op if sb_show_tray hasn't
// been called yet.
void sb_update_tray_paused(int paused);

#endif
