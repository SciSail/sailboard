//go:build windows

package platform

import "testing"

func TestKeyToVKLettersDigitsAndFunctionKeys(t *testing.T) {
	cases := map[string]uint32{"V": 'V', "A": 'A', "0": '0', "9": '9', "F1": 0x70, "F12": 0x7B, "SPACE": 0x20}
	for key, want := range cases {
		got, err := keyToVK(key)
		if err != nil {
			t.Fatalf("keyToVK(%q) error = %v", key, err)
		}
		if got != want {
			t.Errorf("keyToVK(%q) = %#x, want %#x", key, got, want)
		}
	}
}

func TestKeyToVKRejectsUnknownKey(t *testing.T) {
	if _, err := keyToVK("XYZ"); err == nil {
		t.Fatal("keyToVK() expected error for unsupported key")
	}
}

func TestHotkeyModifiersBitmask(t *testing.T) {
	got := hotkeyModifiers(HotkeySpec{Ctrl: true, Shift: true})
	want := uint32(modControl | modShift)
	if got != want {
		t.Fatalf("hotkeyModifiers() = %#x, want %#x", got, want)
	}
}
