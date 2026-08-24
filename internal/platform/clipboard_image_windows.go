//go:build windows

package platform

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"unsafe"
)

// clipboardSequence exposes GetClipboardSequenceNumber so the watcher can detect any clipboard
// change (text, image, or otherwise) with a single cheap call instead of polling-and-diffing
// clipboard content, per design doc §8.2's suggestion to move off naive polling.
func clipboardSequence() (uint32, bool) {
	seq, _, _ := procGetClipboardSeqNum.Call()
	return uint32(seq), true
}

// readClipboardImage reads CF_DIB (device-independent bitmap) from the clipboard, if present,
// and re-encodes it as PNG. Only 24bpp and 32bpp uncompressed DIBs are supported, which covers
// what browsers, Paint, Snipping Tool and Office produce; anything else is reported as absent
// rather than failing capture outright.
func readClipboardImage() (data []byte, width, height int, ok bool) {
	if avail, _, _ := procIsClipboardFormatAvl.Call(cfDIB); avail == 0 {
		return nil, 0, 0, false
	}
	if opened, _, _ := procOpenClipboard.Call(0); opened == 0 {
		return nil, 0, 0, false
	}

	hMem, _, _ := procGetClipboardData.Call(cfDIB)
	if hMem == 0 {
		procCloseClipboard.Call()
		return nil, 0, 0, false
	}
	size, _, _ := procGlobalSize.Call(hMem)
	if size < unsafe.Sizeof(bitmapInfoHeader{}) {
		procCloseClipboard.Call()
		return nil, 0, 0, false
	}
	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		procCloseClipboard.Call()
		return nil, 0, 0, false
	}

	// ptr is a raw Win32 global-memory address (from GlobalLock), not a Go-managed pointer, so
	// go vet's unsafeptr heuristic flags this as a possible misuse; it is the standard pattern
	// for reading foreign memory returned by a syscall.
	raw := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), int(size))
	buf := make([]byte, len(raw))
	copy(buf, raw) // copy out before unlocking / releasing the clipboard
	procGlobalUnlock.Call(hMem)
	procCloseClipboard.Call()

	// Decode/encode run after the clipboard is released: OpenClipboard blocks every other
	// process's clipboard access (including Explorer writing a CF_HDROP for a file copy)
	// system-wide until CloseClipboard, so nothing beyond the raw memory copy above may run
	// while it's held.
	img, err := decodeDIB(buf)
	if err != nil {
		return nil, 0, 0, false
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, 0, 0, false
	}
	b := img.Bounds()
	return out.Bytes(), b.Dx(), b.Dy(), true
}

func decodeDIB(buf []byte) (image.Image, error) {
	if len(buf) < int(unsafe.Sizeof(bitmapInfoHeader{})) {
		return nil, errIconUnavailable
	}
	hdr := (*bitmapInfoHeader)(unsafe.Pointer(&buf[0]))
	width := int(hdr.Width)
	height := int(hdr.Height)
	topDown := height < 0
	if topDown {
		height = -height
	}
	if width <= 0 || height <= 0 {
		return nil, errIconUnavailable
	}

	headerSize := int(hdr.Size)
	paletteBytes := 0
	if hdr.BitCount <= 8 {
		colors := int(hdr.ClrUsed)
		if colors == 0 {
			colors = 1 << hdr.BitCount
		}
		paletteBytes = colors * 4
	}
	pixelOffset := headerSize + paletteBytes
	bytesPerPixel := int(hdr.BitCount) / 8
	if bytesPerPixel != 3 && bytesPerPixel != 4 {
		return nil, errIconUnavailable
	}
	rowSize := ((width*int(hdr.BitCount) + 31) / 32) * 4
	needed := pixelOffset + rowSize*height
	if len(buf) < needed {
		return nil, errIconUnavailable
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		srcRow := y
		if !topDown {
			srcRow = height - 1 - y
		}
		rowStart := pixelOffset + srcRow*rowSize
		for x := 0; x < width; x++ {
			si := rowStart + x*bytesPerPixel
			b, g, r := buf[si], buf[si+1], buf[si+2]
			a := byte(255)
			if bytesPerPixel == 4 {
				a = buf[si+3]
				if a == 0 {
					a = 255 // most 32bpp clipboard DIBs leave alpha unset; treat as opaque
				}
			}
			di := img.PixOffset(x, y)
			img.Pix[di+0] = r
			img.Pix[di+1] = g
			img.Pix[di+2] = b
			img.Pix[di+3] = a
		}
	}
	return img, nil
}

// writeClipboardImage decodes a PNG and writes it back to the system clipboard in two formats, so
// picking an image history item pastes a real image into the target app rather than just
// re-selecting it in SailBoard:
//
//   - CF_DIBV5 (32bpp, with an explicit alpha mask) for apps that check it — this is what
//     actually preserves transparency, since plain CF_DIB has no field that says "byte 4 of each
//     pixel is real alpha"; a receiving app has no reliable way to distinguish real alpha from
//     unused padding in a 32bpp CF_DIB (readClipboardImage's decodeDIB hits exactly this ambiguity
//     on the read side, hence its "most 32bpp clipboard DIBs leave alpha unset" fallback).
//   - CF_DIB (24bpp, opaque) as a fallback for the many apps that only ever look at CF_DIB —
//     matches what Paint/Snipping Tool/browsers already put on the clipboard, and is exactly what
//     this function wrote exclusively before CF_DIBV5 was added alongside it.
//
// Both blocks are fully prepared before OpenClipboard, per the lock-minimization rule established
// for this codebase (see internal/platform/clipboard_files_windows.go) — the locked section is
// just Open → Empty → SetClipboardData × 2 → Close.
func writeClipboardImage(data []byte) error {
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}
	b := decoded.Bounds()
	width, height := b.Dx(), b.Dy()
	if width <= 0 || height <= 0 {
		return fmt.Errorf("empty image")
	}
	// Canonicalize to NRGBA (straight, non-premultiplied alpha) once, rather than reading pixels
	// through .At(x,y).RGBA(): that method always returns alpha-premultiplied components per
	// Go's image/color contract, which would silently darken every partially-transparent pixel's
	// RGB (e.g. a green pixel at 50% alpha comes back with G roughly halved) — fine for the fully
	// opaque case this code used to assume, but wrong now that CF_DIBV5 actually carries alpha.
	// image.Draw's NRGBA conversion un-premultiplies correctly; already-NRGBA input (the common
	// case for a PNG with an alpha channel) is used as-is.
	img, ok := decoded.(*image.NRGBA)
	if !ok {
		converted := image.NewNRGBA(b)
		draw.Draw(converted, b, decoded, b.Min, draw.Src)
		img = converted
	}

	dibv5Mem, err := prepareDIBV5Block(img, width, height)
	if err != nil {
		return err
	}
	dibMem, err := prepareDIBBlock(img, width, height)
	if err != nil {
		procGlobalFree.Call(dibv5Mem)
		return err
	}

	if opened, _, _ := procOpenClipboard.Call(0); opened == 0 {
		procGlobalFree.Call(dibv5Mem)
		procGlobalFree.Call(dibMem)
		return fmt.Errorf("OpenClipboard failed")
	}
	defer procCloseClipboard.Call()
	procEmptyClipboard.Call()
	if ret, _, _ := procSetClipboardData.Call(cfDIBV5, dibv5Mem); ret == 0 {
		procGlobalFree.Call(dibv5Mem)
		procGlobalFree.Call(dibMem)
		return fmt.Errorf("SetClipboardData(CF_DIBV5) failed")
	}
	if ret, _, _ := procSetClipboardData.Call(cfDIB, dibMem); ret == 0 {
		procGlobalFree.Call(dibMem)
		return fmt.Errorf("SetClipboardData(CF_DIB) failed")
	}
	// Ownership of both hMem blocks now belongs to the OS; must not GlobalFree either after a
	// successful SetClipboardData.
	return nil
}

// prepareDIBBlock builds a GlobalAlloc'd 24bpp CF_DIB block (opaque, bottom-up rows) — the
// long-standing fallback format, unchanged in content from before CF_DIBV5 was added.
func prepareDIBBlock(img *image.NRGBA, width, height int) (uintptr, error) {
	rowSize := ((width*24 + 31) / 32) * 4
	pixels := make([]byte, rowSize*height)
	for y := 0; y < height; y++ {
		dstRow := height - 1 - y // CF_DIB rows are bottom-up
		rowOff := dstRow * rowSize
		si := img.PixOffset(img.Rect.Min.X, img.Rect.Min.Y+y)
		for x := 0; x < width; x++ {
			r, g, bl := img.Pix[si], img.Pix[si+1], img.Pix[si+2]
			o := rowOff + x*3
			pixels[o+0] = bl
			pixels[o+1] = g
			pixels[o+2] = r
			si += 4
		}
	}

	var hdr bitmapInfoHeader
	hdr.Size = uint32(unsafe.Sizeof(hdr))
	hdr.Width = int32(width)
	hdr.Height = int32(height)
	hdr.Planes = 1
	hdr.BitCount = 24
	hdr.Compression = biRGB
	hdr.SizeImage = uint32(len(pixels))
	headerBytes := unsafe.Slice((*byte)(unsafe.Pointer(&hdr)), unsafe.Sizeof(hdr))
	return allocGlobalBlock(headerBytes, pixels)
}

// prepareDIBV5Block builds a GlobalAlloc'd 32bpp CF_DIBV5 block with a real alpha channel (BGRA,
// bottom-up rows, BI_BITFIELDS compression with explicit channel masks — see bitmapV5Header's doc
// comment for why CF_DIBV5 specifically, not plain 32bpp CF_DIB).
func prepareDIBV5Block(img *image.NRGBA, width, height int) (uintptr, error) {
	rowSize := width * 4 // 32bpp rows are always a multiple of 4 bytes, no padding needed
	pixels := make([]byte, rowSize*height)
	for y := 0; y < height; y++ {
		dstRow := height - 1 - y // bottom-up, same as plain CF_DIB
		rowOff := dstRow * rowSize
		si := img.PixOffset(img.Rect.Min.X, img.Rect.Min.Y+y)
		for x := 0; x < width; x++ {
			r, g, bl, a := img.Pix[si], img.Pix[si+1], img.Pix[si+2], img.Pix[si+3]
			o := rowOff + x*4
			pixels[o+0] = bl
			pixels[o+1] = g
			pixels[o+2] = r
			pixels[o+3] = a
			si += 4
		}
	}

	var hdr bitmapV5Header
	hdr.Size = uint32(unsafe.Sizeof(hdr))
	hdr.Width = int32(width)
	hdr.Height = int32(height)
	hdr.Planes = 1
	hdr.BitCount = 32
	hdr.Compression = biBitfields
	hdr.SizeImage = uint32(len(pixels))
	hdr.RedMask = 0x00FF0000
	hdr.GreenMask = 0x0000FF00
	hdr.BlueMask = 0x000000FF
	hdr.AlphaMask = 0xFF000000
	hdr.CSType = lcsSRGB
	hdr.Intent = lcsGMImages
	headerBytes := unsafe.Slice((*byte)(unsafe.Pointer(&hdr)), unsafe.Sizeof(hdr))
	return allocGlobalBlock(headerBytes, pixels)
}

// allocGlobalBlock GlobalAlloc's a GMEM_MOVEABLE block sized for header+pixels and copies both
// in, returning the (still GlobalLock'd-then-unlocked) handle ready for SetClipboardData. Shared
// by prepareDIBBlock/prepareDIBV5Block since both build a DIB the same way, just with a
// differently-shaped header.
func allocGlobalBlock(header, pixels []byte) (uintptr, error) {
	total := len(header) + len(pixels)
	hMem, _, _ := procGlobalAlloc.Call(gmemMoveable, uintptr(total))
	if hMem == 0 {
		return 0, fmt.Errorf("GlobalAlloc failed")
	}
	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		procGlobalFree.Call(hMem)
		return 0, fmt.Errorf("GlobalLock failed")
	}
	// Same known-safe foreign-memory pattern as readClipboardImage above: ptr is a Win32 global
	// memory address, not a Go pointer.
	dst := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), total)
	copy(dst, header)
	copy(dst[len(header):], pixels)
	procGlobalUnlock.Call(hMem)
	return hMem, nil
}
