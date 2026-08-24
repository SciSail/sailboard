//go:build windows

package platform

import "fmt"

// keyToVK maps the small set of key names SailBoard's settings UI accepts to Windows virtual-key
// codes. Letters/digits use their ASCII codes (Windows VK_A..VK_Z, VK_0..VK_9 match ASCII), and
// function keys follow VK_F1=0x70 sequentially.
func keyToVK(key string) (uint32, error) {
	if len(key) == 1 {
		c := key[0]
		if c >= 'A' && c <= 'Z' {
			return uint32(c), nil
		}
		if c >= '0' && c <= '9' {
			return uint32(c), nil
		}
	}
	if len(key) >= 2 && key[0] == 'F' {
		n := 0
		for _, c := range key[1:] {
			if c < '0' || c > '9' {
				n = -1
				break
			}
			n = n*10 + int(c-'0')
		}
		if n >= 1 && n <= 24 {
			return uint32(0x70 + n - 1), nil
		}
	}
	switch key {
	case "SPACE":
		return 0x20, nil
	case "TAB":
		return 0x09, nil
	case "ESC", "ESCAPE":
		return 0x1B, nil
	case "ENTER", "RETURN":
		return 0x0D, nil
	}
	return 0, fmt.Errorf("unsupported hotkey key %q", key)
}

func hotkeyModifiers(hk HotkeySpec) uint32 {
	var mods uint32
	if hk.Ctrl {
		mods |= modControl
	}
	if hk.Shift {
		mods |= modShift
	}
	if hk.Alt {
		mods |= modAlt
	}
	if hk.Super {
		mods |= modWin
	}
	return mods
}
