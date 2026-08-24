//go:build windows

package platform

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

// thumbnailMaxSide bounds the longer edge of a decoded-image thumbnail. Cards render it at well
// under 100px tall (see App.css's .file-thumbnail), so this stays comfortably above any real
// display size while keeping the base64 payload small.
const thumbnailMaxSide = 240

// decodableImageExt are the image formats Go's stdlib can decode without extra dependencies
// (image/png, image/jpeg, image/gif — registered via this file's blank imports). Anything else
// (bmp, webp, tiff, heic, ...) falls back to the OS's generic per-type icon instead of a real
// preview.
var decodableImageExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
}

// fileThumbnail produces a best-effort PNG thumbnail for a file or folder history item's first
// path: a real downscaled preview when it's a decodable image, otherwise the same per-type/
// per-folder icon Explorer itself would show. Returns ok=false if path is gone (a stale
// reference — see app.go's CopyItem for the same existence check on paste) or nothing else
// worked.
func fileThumbnail(path string) ([]byte, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	if !info.IsDir() && decodableImageExt[strings.ToLower(filepath.Ext(path))] {
		if data, ok := decodedImageThumbnail(path); ok {
			return data, true
		}
		// Fall through to the generic icon below if decoding failed (e.g. a corrupt or
		// truncated file) rather than reporting no thumbnail at all.
	}
	return fileIconPNG(path)
}

func decodedImageThumbnail(path string) ([]byte, bool) {
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
	if err := png.Encode(&out, resizeNearest(img, thumbnailMaxSide)); err != nil {
		return nil, false
	}
	return out.Bytes(), true
}

// resizeNearest downscales src so its longer edge is at most maxSide, using nearest-neighbour
// sampling — good enough for a small preview thumbnail and avoids pulling in an image-resize
// dependency for what's otherwise a pure-stdlib project. Pure function, no Win32 involved, so
// it's unit-tested directly (see file_thumbnail_windows_test.go) with hand-built images rather
// than real files.
func resizeNearest(src image.Image, maxSide int) *image.RGBA {
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

// fileIconPNG extracts the OS's large per-type icon for path (any file or folder — SHGetFileInfo
// works generically, not just for executables) and re-encodes it as PNG, reusing the exact same
// HICON->image.RGBA conversion already proven for source-app icons (see activeapp_windows.go's
// hIconToImage/shFileInfo).
func fileIconPNG(path string) ([]byte, bool) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, false
	}
	var shfi shFileInfo
	ret, _, _ := procSHGetFileInfo.Call(
		uintptr(unsafe.Pointer(pathPtr)), 0,
		uintptr(unsafe.Pointer(&shfi)), unsafe.Sizeof(shfi),
		shgfiIcon) // large icon: SHGFI_SMALLICON deliberately omitted
	if ret == 0 || shfi.HIcon == 0 {
		return nil, false
	}
	defer procDestroyIcon.Call(shfi.HIcon)

	img, err := hIconToImage(shfi.HIcon)
	if err != nil {
		return nil, false
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, false
	}
	return out.Bytes(), true
}
