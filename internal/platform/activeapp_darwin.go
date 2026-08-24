//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "activeapp_darwin.h"
*/
import "C"

import "unsafe"

func (c *darwinController) ActiveApp() (AppInfo, error) {
	var cName, cPath *C.char
	var cIconData *C.uchar
	var cIconLen C.long
	var cOK C.int
	C.sb_active_app(&cName, &cPath, &cIconData, &cIconLen, &cOK)
	if cOK == 0 {
		return AppInfo{}, nil
	}

	info := AppInfo{}
	if cName != nil {
		info.Name = C.GoString(cName)
		C.free(unsafe.Pointer(cName))
	}
	if cPath != nil {
		info.ExecutablePath = C.GoString(cPath)
		C.free(unsafe.Pointer(cPath))
	}
	if cIconData != nil {
		info.IconPNG = C.GoBytes(unsafe.Pointer(cIconData), C.int(cIconLen))
		C.free(unsafe.Pointer(cIconData))
	}
	return info, nil
}
