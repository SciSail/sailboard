//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "file_thumbnail_darwin.h"
*/
import "C"

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"unsafe"
)

// darwinThumbnailMaxSide/darwinDecodableImageExt/darwinResizeNearest duplicate file_thumbnail_
// windows.go's identical pure-Go logic (thumbnailMaxSide/decodableImageExt/resizeNearest)
// rather than sharing it from a common file — deliberately, to keep this change from touching any
// _windows.go file at all.
const darwinThumbnailMaxSide = 240

var darwinDecodableImageExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
}

// FileThumbnail produces a best-effort PNG thumbnail for a file or folder history item's first
// path: a real downscaled preview when it's a decodable image, otherwise the same per-type/
// per-folder icon Finder itself would show. Returns ok=false if path is gone.
func (c *darwinController) FileThumbnail(path string) ([]byte, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	if !info.IsDir() && darwinDecodableImageExt[strings.ToLower(filepath.Ext(path))] {
		if data, ok := darwinDecodedImageThumbnail(path); ok {
			return data, true
		}
	}
	return darwinFileIconPNG(path)
}

func darwinDecodedImageThumbnail(path string) ([]byte, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, false
	}
	var out bytes.Buffer
	if err := png.Encode(&out, darwinResizeNearest(img, darwinThumbnailMaxSide)); err != nil {
		return nil, false
	}
	return out.Bytes(), true
}

func darwinResizeNearest(src image.Image, maxSide int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	scale := 1.0
	if w > maxSide || h > maxSide {
		scale = float64(maxSide) / float64(w)
		if hScale := float64(maxSide) / float64(h); hScale < scale {
			scale = hScale
		}
	}
	dw, dh := max(1, int(float64(w)*scale)), max(1, int(float64(h)*scale))
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		sy := b.Min.Y + int(float64(y)/scale)
		for x := 0; x < dw; x++ {
			sx := b.Min.X + int(float64(x)/scale)
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

func darwinFileIconPNG(path string) ([]byte, bool) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	var cData *C.uchar
	var cLen C.long
	var cOK C.int
	C.sb_file_icon(cPath, 64, &cData, &cLen, &cOK)
	if cOK == 0 {
		return nil, false
	}
	defer C.free(unsafe.Pointer(cData))
	return C.GoBytes(unsafe.Pointer(cData), C.int(cLen)), true
}
