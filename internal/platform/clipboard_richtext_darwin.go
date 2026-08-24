//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "clipboard_richtext_darwin.h"
*/
import "C"

import "unsafe"

func (c *darwinController) ReadClipboardRichText() (html, rtf, text string, ok bool) {
	var cHTML, cRTF, cText *C.char
	var cOK C.int
	C.sb_read_clipboard_richtext(&cHTML, &cRTF, &cText, &cOK)
	if cOK == 0 {
		return "", "", "", false
	}
	if cHTML != nil {
		html = C.GoString(cHTML)
		C.free(unsafe.Pointer(cHTML))
	}
	if cRTF != nil {
		rtf = C.GoString(cRTF)
		C.free(unsafe.Pointer(cRTF))
	}
	if cText != nil {
		text = C.GoString(cText)
		C.free(unsafe.Pointer(cText))
	}
	return html, rtf, text, true
}

func (c *darwinController) WriteClipboardRichText(html, rtf, text string) error {
	var cHTML, cRTF *C.char
	if html != "" {
		cHTML = C.CString(html)
		defer C.free(unsafe.Pointer(cHTML))
	}
	if rtf != "" {
		cRTF = C.CString(rtf)
		defer C.free(unsafe.Pointer(cRTF))
	}
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))

	C.sb_write_clipboard_richtext(cHTML, cRTF, cText)
	return nil
}
