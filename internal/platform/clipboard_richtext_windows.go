//go:build windows

package platform

import (
	"bytes"
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

// readClipboardRichText reads "HTML Format" and/or "Rich Text Format" (both registered clipboard
// formats, per design doc's Office-copy handling) alongside CF_UNICODETEXT, all within a single
// OpenClipboard session. ok is true only when there's genuine plain text *and* at least one rich
// format present — that combination is what a real Office (or browser) formatted-text copy always
// produces, whereas a bare image copy (e.g. a browser's "Copy image", which often also sets a
// minimal <img>-only HTML Format) typically carries no meaningful CF_UNICODETEXT alongside it.
// This is the signal watcher.go's per-tick check uses to prefer rich text over the accompanying
// preview bitmap Office also puts on the clipboard for a text copy — see Watcher.Start.
//
// html/rtf are stored and handed back to SetClipboardData byte-for-byte on paste (see
// writeClipboardRichText), never parsed: "HTML Format" in particular has its own header with byte
// offsets into itself (Version/StartHTML/EndHTML/StartFragment/EndFragment), and round-tripping
// the exact bytes keeps those offsets valid without SailBoard ever needing to understand them.
func readClipboardRichText() (html, rtf, text string, ok bool) {
	htmlFormat := registeredClipboardFormat("HTML Format")
	rtfFormat := registeredClipboardFormat("Rich Text Format")
	if htmlFormat == 0 && rtfFormat == 0 {
		return "", "", "", false
	}
	htmlAvail := htmlFormat != 0 && isClipboardFormatAvailable(htmlFormat)
	rtfAvail := rtfFormat != 0 && isClipboardFormatAvailable(rtfFormat)
	if !htmlAvail && !rtfAvail {
		return "", "", "", false
	}

	if opened, _, _ := procOpenClipboard.Call(0); opened == 0 {
		return "", "", "", false
	}
	defer procCloseClipboard.Call()

	if htmlAvail {
		html = readClipboardAnsiFormat(htmlFormat)
	}
	if rtfAvail {
		rtf = readClipboardAnsiFormat(rtfFormat)
	}
	text = readClipboardUnicodeTextLocked()

	if strings.TrimSpace(text) == "" || (html == "" && rtf == "") {
		return "", "", "", false
	}
	return html, rtf, text, true
}

// writeClipboardRichText writes html and/or rtf back onto the system clipboard (whichever is
// non-empty) alongside plain text as CF_UNICODETEXT, so a paste into an app that understands
// "HTML Format"/"Rich Text Format" (Word, Excel, PowerPoint, browsers, most rich text editors)
// gets the original formatting back, while an app that only reads CF_UNICODETEXT (Notepad, most
// terminals/IDEs) transparently falls back to plain text — the receiving app picks the format it
// wants, SailBoard doesn't need to know or guess which app is focused.
//
// All three blocks are fully prepared before OpenClipboard, per this codebase's established
// lock-minimization rule (see clipboard_files_windows.go/clipboard_image_windows.go).
func writeClipboardRichText(html, rtf, text string) error {
	var htmlFormat, rtfFormat uintptr
	var htmlMem, rtfMem uintptr
	if html != "" {
		htmlFormat = registeredClipboardFormat("HTML Format")
		if htmlFormat != 0 {
			htmlMem = allocAnsiBlock(html)
		}
	}
	if rtf != "" {
		rtfFormat = registeredClipboardFormat("Rich Text Format")
		if rtfFormat != 0 {
			rtfMem = allocAnsiBlock(rtf)
		}
	}
	textMem := allocUnicodeBlock(text)

	freeAll := func() {
		if htmlMem != 0 {
			procGlobalFree.Call(htmlMem)
		}
		if rtfMem != 0 {
			procGlobalFree.Call(rtfMem)
		}
		if textMem != 0 {
			procGlobalFree.Call(textMem)
		}
	}

	if opened, _, _ := procOpenClipboard.Call(0); opened == 0 {
		freeAll()
		return fmt.Errorf("OpenClipboard failed")
	}
	defer procCloseClipboard.Call()
	procEmptyClipboard.Call()

	if textMem != 0 {
		if ret, _, _ := procSetClipboardData.Call(cfUnicodeText, textMem); ret == 0 {
			procGlobalFree.Call(textMem)
		}
	}
	if htmlFormat != 0 && htmlMem != 0 {
		if ret, _, _ := procSetClipboardData.Call(htmlFormat, htmlMem); ret == 0 {
			procGlobalFree.Call(htmlMem)
		}
	}
	if rtfFormat != 0 && rtfMem != 0 {
		if ret, _, _ := procSetClipboardData.Call(rtfFormat, rtfMem); ret == 0 {
			procGlobalFree.Call(rtfMem)
		}
	}
	// Ownership of every successfully-set block now belongs to the OS; a failed SetClipboardData
	// above already freed its own block, so nothing further to release either way.
	return nil
}

func registeredClipboardFormat(name string) uintptr {
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0
	}
	format, _, _ := procRegisterClipboardFormat.Call(uintptr(unsafe.Pointer(namePtr)))
	return format
}

func isClipboardFormatAvailable(format uintptr) bool {
	avail, _, _ := procIsClipboardFormatAvl.Call(format)
	return avail != 0
}

// readClipboardAnsiFormat reads a single-byte-per-char clipboard format (used for both "HTML
// Format" and "Rich Text Format", which are ASCII/UTF-8 text despite not being CF_TEXT) as a Go
// string, trimmed at the first NUL terminator if present. Must be called with the clipboard
// already open.
func readClipboardAnsiFormat(format uintptr) string {
	hMem, _, _ := procGetClipboardData.Call(format)
	if hMem == 0 {
		return ""
	}
	size, _, _ := procGlobalSize.Call(hMem)
	if size == 0 {
		return ""
	}
	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		return ""
	}
	// ptr is a raw Win32 global-memory address (from GlobalLock), not a Go-managed pointer — same
	// known-safe foreign-memory pattern used throughout this package (see e.g.
	// readClipboardImage's doc comment).
	raw := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), int(size))
	if nul := bytes.IndexByte(raw, 0); nul >= 0 {
		raw = raw[:nul]
	}
	s := string(raw)
	procGlobalUnlock.Call(hMem)
	return s
}

// readClipboardUnicodeTextLocked reads CF_UNICODETEXT as a Go string, trimmed at the first NUL
// terminator. Must be called with the clipboard already open.
func readClipboardUnicodeTextLocked() string {
	hMem, _, _ := procGetClipboardData.Call(cfUnicodeText)
	if hMem == 0 {
		return ""
	}
	size, _, _ := procGlobalSize.Call(hMem)
	if size < 2 {
		return ""
	}
	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		return ""
	}
	units := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), int(size)/2)
	s := syscall.UTF16ToString(units)
	procGlobalUnlock.Call(hMem)
	return s
}

// allocAnsiBlock GlobalAlloc's a NUL-terminated single-byte-per-char block from s (used for both
// "HTML Format" and "Rich Text Format" writes), or returns 0 on failure.
func allocAnsiBlock(s string) uintptr {
	data := append([]byte(s), 0)
	hMem, _, _ := procGlobalAlloc.Call(gmemMoveable, uintptr(len(data)))
	if hMem == 0 {
		return 0
	}
	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		procGlobalFree.Call(hMem)
		return 0
	}
	dst := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), len(data))
	copy(dst, data)
	procGlobalUnlock.Call(hMem)
	return hMem
}

// allocUnicodeBlock GlobalAlloc's a NUL-terminated UTF-16 block from s for CF_UNICODETEXT, or
// returns 0 on failure.
func allocUnicodeBlock(s string) uintptr {
	units, err := syscall.UTF16FromString(s)
	if err != nil {
		return 0
	}
	byteLen := len(units) * 2
	hMem, _, _ := procGlobalAlloc.Call(gmemMoveable, uintptr(byteLen))
	if hMem == 0 {
		return 0
	}
	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		procGlobalFree.Call(hMem)
		return 0
	}
	dst := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(units))
	copy(dst, units)
	procGlobalUnlock.Call(hMem)
	return hMem
}
