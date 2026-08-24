package platform

import "testing"

func TestParseHotkeySpecModifiersAndKey(t *testing.T) {
	hk, err := ParseHotkeySpec("Ctrl+Shift+V")
	if err != nil {
		t.Fatalf("ParseHotkeySpec() error = %v", err)
	}
	if !hk.Ctrl || !hk.Shift || hk.Alt || hk.Super {
		t.Fatalf("ParseHotkeySpec() modifiers = %+v, want ctrl+shift only", hk)
	}
	if hk.Key != "V" {
		t.Fatalf("ParseHotkeySpec() key = %q, want %q", hk.Key, "V")
	}
}

func TestParseHotkeySpecAliasesAndWhitespace(t *testing.T) {
	hk, err := ParseHotkeySpec(" command + option + F5 ")
	if err != nil {
		t.Fatalf("ParseHotkeySpec() error = %v", err)
	}
	if !hk.Super || !hk.Alt || hk.Ctrl || hk.Shift {
		t.Fatalf("ParseHotkeySpec() modifiers = %+v, want super+alt only", hk)
	}
	if hk.Key != "F5" {
		t.Fatalf("ParseHotkeySpec() key = %q, want %q", hk.Key, "F5")
	}
}

func TestParseHotkeySpecRejectsMissingKey(t *testing.T) {
	if _, err := ParseHotkeySpec("Ctrl+Shift"); err == nil {
		t.Fatal("ParseHotkeySpec() expected error for missing key")
	}
}

func TestParseHotkeySpecRejectsMultipleKeys(t *testing.T) {
	if _, err := ParseHotkeySpec("Ctrl+V+B"); err == nil {
		t.Fatal("ParseHotkeySpec() expected error for multiple keys")
	}
}
