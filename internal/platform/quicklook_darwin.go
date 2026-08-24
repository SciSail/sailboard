//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework Quartz
#include <stdlib.h>
#include "quicklook_darwin.h"
*/
import "C"

import "unsafe"

// PreviewFile toggles the shared QLPreviewPanel for paths — see quicklook_darwin.m's
// sb_quicklook_toggle doc comment for the open/close semantics.
func (c *darwinController) PreviewFile(paths []string) bool {
	if len(paths) == 0 {
		return false
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
	return C.sb_quicklook_toggle((**C.char)(unsafe.Pointer(&cPaths[0])), C.int(len(cPaths))) != 0
}
