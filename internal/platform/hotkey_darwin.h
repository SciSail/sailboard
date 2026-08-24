#ifndef SAILBOARD_HOTKEY_DARWIN_H
#define SAILBOARD_HOTKEY_DARWIN_H
#include <stdint.h>

// Registers a global hotkey via Carbon's RegisterEventHotKey (keyCode/modifiers are HIToolbox
// virtual-keycode/modifier-mask values — see hotkey_darwin.go's darwinKeyToVK/
// darwinHotkeyModifiers) and returns an opaque reference for sb_unregister_hotkey, or 0 on
// failure. Carbon hotkeys, unlike a global NSEvent monitor, don't require the Accessibility/Input
// Monitoring permission grant — this is the same approach most third-party macOS launchers use.
// Firing invokes the exported Go function sbHotkeyFired(hotkeyID) (see hotkey_darwin.go).
uint64_t sb_register_hotkey(uint32_t keyCode, uint32_t modifiers, uint32_t hotkeyID);

// Unregisters a hotkey previously returned by sb_register_hotkey. No-op if ref is 0.
void sb_unregister_hotkey(uint64_t ref);

#endif
