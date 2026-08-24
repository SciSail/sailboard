//go:build windows

package platform

import (
	"bytes"
	"image"
	"image/png"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

// activeApp implements design doc §12.3: GetForegroundWindow -> GetWindowThreadProcessId ->
// OpenProcess -> QueryFullProcessImageName, then a best-effort small-icon extraction.
func activeApp() (AppInfo, error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return AppInfo{}, nil
	}
	var pid uint32
	procGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return AppInfo{}, nil
	}

	handle, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if handle == 0 {
		return AppInfo{}, nil
	}
	defer procCloseHandle.Call(handle)

	buf := make([]uint16, 1024)
	size := uint32(len(buf))
	ok, _, _ := procQueryFullProcImg.Call(handle, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if ok == 0 {
		return AppInfo{}, nil
	}
	exePath := syscall.UTF16ToString(buf[:size])
	name := strings.TrimSuffix(filepath.Base(exePath), filepath.Ext(exePath))

	info := AppInfo{Name: name, ExecutablePath: exePath}
	if iconPNG, err := extractAppIconPNG(exePath); err == nil {
		info.IconPNG = iconPNG
	}
	return info, nil
}

// extractAppIconPNG pulls the shell's large icon (typically 32x32, vs. SHGFI_SMALLICON's 16x16)
// for exePath and re-encodes it as PNG so the frontend can render it as a plain data URI without
// any native image type on the JS side. The card header displays this at 18 CSS px, which on any
// scaled-DPI display is well past 16 physical px — SHGFI_SMALLICON visibly blurs there, matching
// the same large-icon choice already made for file thumbnails (see fileIconPNG).
func extractAppIconPNG(exePath string) ([]byte, error) {
	pathPtr, err := syscall.UTF16PtrFromString(exePath)
	if err != nil {
		return nil, err
	}
	var shfi shFileInfo
	ret, _, _ := procSHGetFileInfo.Call(
		uintptr(unsafe.Pointer(pathPtr)), 0,
		uintptr(unsafe.Pointer(&shfi)), unsafe.Sizeof(shfi),
		shgfiIcon)
	if ret == 0 || shfi.HIcon == 0 {
		return nil, errIconUnavailable
	}
	defer procDestroyIcon.Call(shfi.HIcon)

	img, err := hIconToImage(shfi.HIcon)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type shFileInfo struct {
	HIcon         uintptr
	IIcon         int32
	DwAttributes  uint32
	SzDisplayName [260]uint16
	SzTypeName    [80]uint16
}

var errIconUnavailable = errString("icon unavailable")

type errString string

func (e errString) Error() string { return string(e) }

// hIconToImage converts a Win32 HICON to an *image.RGBA via GetIconInfo + GetDIBits, reading the
// 32bpp BGRA colour bitmap top-down and swapping channel order into Go's RGBA layout.
func hIconToImage(hIcon uintptr) (image.Image, error) {
	var info iconInfo
	if ok, _, _ := procGetIconInfo.Call(hIcon, uintptr(unsafe.Pointer(&info))); ok == 0 {
		return nil, errIconUnavailable
	}
	defer procDeleteObject.Call(info.HbmMask)
	defer procDeleteObject.Call(info.HbmColor)

	hdc, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, hdc)
	memDC, _, _ := procCreateCompatibleDC.Call(hdc)
	defer procDeleteDC.Call(memDC)

	var bmi bitmapInfoHeader
	bmi.Size = uint32(unsafe.Sizeof(bmi))
	// First call with a nil pixel buffer fills in width/height/bitcount from the bitmap.
	procGetDIBits.Call(memDC, info.HbmColor, 0, 0, 0, uintptr(unsafe.Pointer(&bmi)), 0)
	if bmi.Width == 0 || bmi.Height == 0 {
		return nil, errIconUnavailable
	}
	width, height := int(bmi.Width), int(bmi.Height)
	if height < 0 {
		height = -height
	}

	bmi.Compression = biRGB
	bmi.BitCount = 32
	bmi.Planes = 1
	bmi.Height = int32(height) // request top-down won't be honoured for icons; positive = bottom-up source
	pixels := make([]byte, width*height*4)
	ret, _, _ := procGetDIBits.Call(memDC, info.HbmColor, 0, uintptr(height),
		uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&bmi)), 0)
	if ret == 0 {
		return nil, errIconUnavailable
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	hasAlpha := false
	for y := 0; y < height; y++ {
		srcY := height - 1 - y // GetDIBits with positive Height returns bottom-up rows
		for x := 0; x < width; x++ {
			si := (srcY*width + x) * 4
			di := img.PixOffset(x, y)
			b, g, r, a := pixels[si], pixels[si+1], pixels[si+2], pixels[si+3]
			if a != 0 {
				hasAlpha = true
			}
			img.Pix[di+0] = r
			img.Pix[di+1] = g
			img.Pix[di+2] = b
			img.Pix[di+3] = a
		}
	}
	// Legacy (non-PNG) icons often carry no real alpha channel in the 32bpp colour bitmap and
	// rely on a separate 1bpp mask instead; GetDIBits above ignores that mask, so treat an
	// all-zero alpha plane as fully opaque rather than rendering an invisible icon.
	if !hasAlpha {
		for i := 3; i < len(img.Pix); i += 4 {
			img.Pix[i] = 255
		}
	}
	return img, nil
}
