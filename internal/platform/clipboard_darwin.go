//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "clipboard_darwin.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func (c *darwinController) WriteClipboardImage(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("write clipboard image: no data")
	}
	if C.sb_write_clipboard_image((*C.uchar)(unsafe.Pointer(&data[0])), C.long(len(data))) == 0 {
		return fmt.Errorf("write clipboard image: failed")
	}
	return nil
}

func (c *darwinController) ReadClipboardImage() (data []byte, width, height int, ok bool) {
	var cData *C.uchar
	var cLen C.long
	var cWidth, cHeight, cOK C.int
	C.sb_read_clipboard_image(&cData, &cLen, &cWidth, &cHeight, &cOK)
	if cOK == 0 {
		return nil, 0, 0, false
	}
	defer C.sb_free(unsafe.Pointer(cData))
	return C.GoBytes(unsafe.Pointer(cData), C.int(cLen)), int(cWidth), int(cHeight), true
}

func (c *darwinController) ClipboardSequence() (uint32, bool) {
	return uint32(C.sb_clipboard_change_count()), true
}
