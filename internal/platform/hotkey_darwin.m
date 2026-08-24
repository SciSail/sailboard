#include "hotkey_darwin.h"
#import <Carbon/Carbon.h>
#include "_cgo_export.h"

// Carbon hotkey events are delivered through the application event target regardless of which
// keycode/id fired, so one application-wide handler routes every registered hotkey — installed
// lazily on first use rather than at process start, since a headless/CLI build of this package
// (go test, go vet) never needs it.
static BOOL sHandlerInstalled = NO;

static OSStatus sb_hotkey_handler(EventHandlerCallRef nextHandler, EventRef theEvent, void *userData) {
    EventHotKeyID hkID;
    OSStatus err = GetEventParameter(theEvent, kEventParamDirectObject, typeEventHotKeyID, NULL, sizeof(hkID), NULL, &hkID);
    if (err == noErr) {
        sbHotkeyFired(hkID.id);
    }
    return noErr;
}

static void sb_install_handler_once(void) {
    if (sHandlerInstalled) {
        return;
    }
    sHandlerInstalled = YES;
    EventTypeSpec eventType = { kEventClassKeyboard, kEventHotKeyPressed };
    InstallApplicationEventHandler(&sb_hotkey_handler, 1, &eventType, NULL, NULL);
}

uint64_t sb_register_hotkey(uint32_t keyCode, uint32_t modifiers, uint32_t hotkeyID) {
    sb_install_handler_once();
    EventHotKeyID hkID;
    hkID.signature = 'SBRD';
    hkID.id = hotkeyID;
    EventHotKeyRef ref = NULL;
    OSStatus err = RegisterEventHotKey(keyCode, modifiers, hkID, GetApplicationEventTarget(), 0, &ref);
    if (err != noErr || ref == NULL) {
        return 0;
    }
    return (uint64_t)(uintptr_t)ref;
}

void sb_unregister_hotkey(uint64_t ref) {
    if (ref == 0) {
        return;
    }
    UnregisterEventHotKey((EventHotKeyRef)(uintptr_t)ref);
}
