package platform

import (
	"fmt"
	"strings"
)

// HotkeySpec is a parsed, platform-neutral representation of a shortcut like "Ctrl+Shift+V".
// Parsing is kept OS-agnostic (pure string logic, no syscalls) so it can be unit tested without
// a build tag; each controller_<os>.go maps the modifiers/Key to its own virtual-key constants.
type HotkeySpec struct {
	Ctrl, Shift, Alt, Super bool
	Key                     string // normalised upper-case key name, e.g. "V", "F5", "SPACE"
}

// ParseHotkeySpec parses a "+"-separated combo. Modifier aliases (ctrl/control, cmd/command/
// super/win/windows/meta) are case-insensitive; exactly one non-modifier token must remain.
func ParseHotkeySpec(spec string) (HotkeySpec, error) {
	var hk HotkeySpec
	var key string
	for _, raw := range strings.Split(spec, "+") {
		part := strings.TrimSpace(raw)
		if part == "" {
			continue
		}
		switch strings.ToLower(part) {
		case "ctrl", "control":
			hk.Ctrl = true
		case "shift":
			hk.Shift = true
		case "alt", "option":
			hk.Alt = true
		case "cmd", "command", "super", "win", "windows", "meta":
			hk.Super = true
		default:
			if key != "" {
				return HotkeySpec{}, fmt.Errorf("invalid hotkey %q: more than one key given", spec)
			}
			key = strings.ToUpper(part)
		}
	}
	if key == "" {
		return HotkeySpec{}, fmt.Errorf("invalid hotkey %q: no key given", spec)
	}
	hk.Key = key
	return hk, nil
}
