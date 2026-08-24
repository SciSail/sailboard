//go:build darwin

package platform

/*
#cgo LDFLAGS: -framework Carbon
#include <stdlib.h>
#include "hotkey_darwin.h"
*/
import "C"

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// darwinVKByLetter/darwinVKByDigit/darwinVKByFKey map the key names SailBoard's settings UI
// accepts to macOS (HIToolbox) virtual keycodes. Unlike Windows, where VK_A..VK_Z/VK_0..VK_9
// match ASCII (see keyToVK in hotkey_windows.go), Mac keycodes are physical-position codes with
// no arithmetic relationship to the character or to each other, so this has to be a literal
// table — one row per physical key, sourced from Carbon's HIToolbox/Events.h kVK_* constants.
var darwinVKByLetter = map[byte]uint32{
	'A': 0x00, 'B': 0x0B, 'C': 0x08, 'D': 0x02, 'E': 0x0E, 'F': 0x03, 'G': 0x05, 'H': 0x04,
	'I': 0x22, 'J': 0x26, 'K': 0x28, 'L': 0x25, 'M': 0x2E, 'N': 0x2D, 'O': 0x1F, 'P': 0x23,
	'Q': 0x0C, 'R': 0x0F, 'S': 0x01, 'T': 0x11, 'U': 0x20, 'V': 0x09, 'W': 0x0D, 'X': 0x07,
	'Y': 0x10, 'Z': 0x06,
}

var darwinVKByDigit = map[byte]uint32{
	'0': 0x1D, '1': 0x12, '2': 0x13, '3': 0x14, '4': 0x15, '5': 0x17, '6': 0x16, '7': 0x1A,
	'8': 0x1C, '9': 0x19,
}

var darwinVKByFKey = map[int]uint32{
	1: 0x7A, 2: 0x78, 3: 0x63, 4: 0x76, 5: 0x60, 6: 0x61, 7: 0x62, 8: 0x64, 9: 0x65, 10: 0x6D,
	11: 0x67, 12: 0x6F, 13: 0x69, 14: 0x6B, 15: 0x71, 16: 0x6A, 17: 0x40, 18: 0x4F, 19: 0x50, 20: 0x5A,
}

// darwinKeyToVK is the macOS counterpart to keyToVK (hotkey_windows.go). Mac keyboards top out at
// F20 (no F21-F24 kVK_* constant exists), unlike the settings UI's generic F1-F24 capture regex —
// selecting F21+ on macOS falls through to the "unsupported" error below, same as any other
// unmapped key.
func darwinKeyToVK(key string) (uint32, error) {
	if len(key) == 1 {
		if vk, ok := darwinVKByLetter[key[0]]; ok {
			return vk, nil
		}
		if vk, ok := darwinVKByDigit[key[0]]; ok {
			return vk, nil
		}
	}
	if len(key) >= 2 && key[0] == 'F' {
		n := 0
		valid := true
		for _, c := range key[1:] {
			if c < '0' || c > '9' {
				valid = false
				break
			}
			n = n*10 + int(c-'0')
		}
		if valid {
			if vk, ok := darwinVKByFKey[n]; ok {
				return vk, nil
			}
		}
	}
	switch key {
	case "SPACE":
		return 0x31, nil
	case "TAB":
		return 0x30, nil
	case "ESC", "ESCAPE":
		return 0x35, nil
	case "ENTER", "RETURN":
		return 0x24, nil
	}
	return 0, fmt.Errorf("unsupported hotkey key %q", key)
}

// Carbon modifier-mask bits (HIToolbox/Events.h: cmdKey/shiftKey/optionKey/controlKey). Hardcoded
// rather than pulled from the Carbon header on the Go side since cgo constants would need their
// own C shim anyway — these four values have been stable since classic Mac OS.
const (
	darwinModCmd     = 0x0100
	darwinModShift   = 0x0200
	darwinModOption  = 0x0800
	darwinModControl = 0x1000
)

// darwinHotkeyModifiers is the macOS counterpart to hotkeyModifiers (hotkey_windows.go). Super
// (the settings UI's "Win"/Cmd modifier) maps to Cmd here, matching the Mac convention the
// default shortcut ("Cmd+Shift+V", see repository.DefaultSettings' darwin override in app.go)
// already assumes.
func darwinHotkeyModifiers(hk HotkeySpec) uint32 {
	var mods uint32
	if hk.Ctrl {
		mods |= darwinModControl
	}
	if hk.Shift {
		mods |= darwinModShift
	}
	if hk.Alt {
		mods |= darwinModOption
	}
	if hk.Super {
		mods |= darwinModCmd
	}
	return mods
}

// darwinHotkeyHandlers maps the small integer IDs sb_register_hotkey hands to Carbon back to the
// Go callback to run when that hotkey fires (sbHotkeyFired below) — Carbon's EventHotKeyID is a
// plain uint32, not big enough to carry a Go func value or pointer safely across the cgo
// boundary, so the ID is just an index into this map instead.
var (
	darwinNextHotkeyID   uint32
	darwinHotkeyHandlers sync.Map // uint32 -> func()
)

// RegisterHotkey implements Controller.RegisterHotkey via Carbon's RegisterEventHotKey — see
// hotkey_darwin.h's doc comment for why Carbon rather than a global NSEvent monitor (no
// Accessibility permission prompt).
func (c *darwinController) RegisterHotkey(spec string, handler func()) (func(), error) {
	hk, err := ParseHotkeySpec(spec)
	if err != nil {
		return nil, err
	}
	vk, err := darwinKeyToVK(hk.Key)
	if err != nil {
		return nil, err
	}
	mods := darwinHotkeyModifiers(hk)

	id := atomic.AddUint32(&darwinNextHotkeyID, 1)
	darwinHotkeyHandlers.Store(id, handler)

	ref := uint64(C.sb_register_hotkey(C.uint32_t(vk), C.uint32_t(mods), C.uint32_t(id)))
	if ref == 0 {
		darwinHotkeyHandlers.Delete(id)
		return nil, fmt.Errorf("register hotkey %q: RegisterEventHotKey failed (already taken by another app?)", spec)
	}

	unregister := func() {
		C.sb_unregister_hotkey(C.uint64_t(ref))
		darwinHotkeyHandlers.Delete(id)
	}
	return unregister, nil
}

// sbHotkeyFired is called synchronously from Carbon's event handler on the main thread, still
// inside that native callstack. Running the handler in-place from there — as an earlier version
// of this file did — crashed with SIGSEGV: the handler is app.go's ShowWindow, which spawns its
// own goroutine to run SlideReveal's animation, and that goroutine's first cgo call into Cocoa
// (PositionSelf) reliably faulted. The distinguishing factor, confirmed by comparison with the
// otherwise-identical animation spawned from the ordinary (non-callback) 300ms post-startup
// ShowWindow call in app.go, which never crashed across repeated testing: a goroutine spawned
// from code that is itself executing as a nested cgo callback (C called into Go, which spawned a
// goroutine that immediately calls back into C) lands somewhere the Go runtime doesn't set up
// safely for a fresh Objective-C/GCD call, even though the same call from an ordinarily-spawned
// goroutine is fine. Routing through darwinMainThreadCallbacks (controller_darwin.go) means the
// handler always runs from that channel's ordinary dispatcher goroutine — never from inside the
// Carbon callback's stack — sidestepping the whole class of problem rather than chasing its exact
// cause. tray_darwin.go's sbTray*Clicked functions hit the identical hazard (AppKit menu-item
// target-action dispatch is the same kind of main-thread callback) and share this same channel.
//
//export sbHotkeyFired
func sbHotkeyFired(id uint32) {
	select {
	case darwinMainThreadCallbacks <- func() {
		if v, ok := darwinHotkeyHandlers.Load(id); ok {
			if handler, ok := v.(func()); ok {
				handler()
			}
		}
	}:
	default: // dispatcher goroutine is somehow backed up; drop rather than block Carbon's event thread
	}
}
