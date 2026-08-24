//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "clipboard_files_darwin.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func (c *darwinController) ReadClipboardFiles() (paths []string, ok bool) {
	var cPaths **C.char
	var cCount C.int
	C.sb_read_clipboard_files(&cPaths, &cCount)
	if cCount == 0 {
		return nil, false
	}
	n := int(cCount)
	cSlice := unsafe.Slice(cPaths, n)
	paths = make([]string, n)
	for i, p := range cSlice {
		paths[i] = C.GoString(p)
	}
	C.sb_free_string_array(cPaths, cCount)
	return paths, true
}

func (c *darwinController) WriteClipboardFiles(paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("write clipboard files: no paths")
	}
	cPaths := make([]*C.char, len(paths))
	for i, p := range paths {
		cPaths[i] = C.CString(p)
	}
	defer func() {
		for _, p := range cPaths {
			C.free(unsafe.Pointer(p))
		}
	}()
	C.sb_write_clipboard_files((**C.char)(unsafe.Pointer(&cPaths[0])), C.int(len(cPaths)))
	return nil
}
