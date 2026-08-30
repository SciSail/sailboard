//go:build windows

package platform

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
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

// readClipboardImage is the legacy single-image entry point. Keep it backed by
// the same one-session snapshot reader used by the event-driven watcher so old
// callers also get PNG/CF_DIBV5/CF_DIB support and the same lock-minimised path.
func readClipboardImage() (data []byte, width, height int, ok bool) {
	snap, err := readClipboardSnapshot()
	if err != nil {
		return nil, 0, 0, false
	}
	return snap.ImagePNG, snap.ImageWidth, snap.ImageHeight, len(snap.ImagePNG) > 0
}

func decodeDIB(buf []byte) (image.Image, error) {
	if len(buf) < 40 {
		return nil, errIconUnavailable
	}
	hdrSize := int(binary.LittleEndian.Uint32(buf[0:4]))
	if hdrSize < 40 || hdrSize > len(buf) {
		return nil, errIconUnavailable
	}
	width := int(int32(binary.LittleEndian.Uint32(buf[4:8])))
	heightRaw := int(int32(binary.LittleEndian.Uint32(buf[8:12])))
	if width <= 0 || heightRaw == 0 || width > 65535 || heightRaw == -2147483648 {
		return nil, errIconUnavailable
	}
	topDown := heightRaw < 0
	height := heightRaw
	if topDown {
		height = -height
	}
	if height > 65535 || width*height > 64*1024*1024 {
		return nil, errIconUnavailable
	}
	planes := binary.LittleEndian.Uint16(buf[12:14])
	bpp := int(binary.LittleEndian.Uint16(buf[14:16]))
	compression := binary.LittleEndian.Uint32(buf[16:20])
	clrUsed := int(binary.LittleEndian.Uint32(buf[32:36]))
	if planes != 0 && planes != 1 {
		return nil, errIconUnavailable
	}
	switch bpp {
	case 1, 4, 8, 16, 24, 32:
	default:
		return nil, errIconUnavailable
	}
	if compression != biRGB && compression != biBitfields && compression != 6 {
		return nil, errIconUnavailable
	}

	var rMask, gMask, bMask, aMask uint32
	maskExtra := 0
	if compression == biBitfields || compression == 6 {
		if hdrSize == 40 {
			if len(buf) < 52 {
				return nil, errIconUnavailable
			}
			rMask = binary.LittleEndian.Uint32(buf[40:44])
			gMask = binary.LittleEndian.Uint32(buf[44:48])
			bMask = binary.LittleEndian.Uint32(buf[48:52])
			maskExtra = 12
			if compression == 6 && len(buf) >= 56 {
				aMask = binary.LittleEndian.Uint32(buf[52:56])
				maskExtra = 16
			}
		} else {
			if hdrSize < 56 {
				return nil, errIconUnavailable
			}
			rMask = binary.LittleEndian.Uint32(buf[40:44])
			gMask = binary.LittleEndian.Uint32(buf[44:48])
			bMask = binary.LittleEndian.Uint32(buf[48:52])
			aMask = binary.LittleEndian.Uint32(buf[52:56])
		}
	} else if hdrSize >= 108 && len(buf) >= 56 {
		aMask = binary.LittleEndian.Uint32(buf[52:56])
	}
	if bpp == 16 && rMask == 0 {
		rMask, gMask, bMask = 0x7C00, 0x03E0, 0x001F
	}
	paletteEntries := 0
	if bpp <= 8 {
		paletteEntries = clrUsed
		if paletteEntries == 0 {
			paletteEntries = 1 << bpp
		}
		if paletteEntries > 256 {
			return nil, errIconUnavailable
		}
	}
	pixelOffset := hdrSize + maskExtra + paletteEntries*4
	stride := ((width*bpp + 31) / 32) * 4
	if pixelOffset < 0 || stride <= 0 || height > (len(buf)-pixelOffset)/stride {
		return nil, errIconUnavailable
	}

	palette := make([]color.NRGBA, paletteEntries)
	for i := range palette {
		off := hdrSize + maskExtra + i*4
		// RGBQUAD's fourth byte is reserved for the classic palette
		// formats (it is normally zero), not an alpha channel. Treating
		// it as alpha would make ordinary 1/4/8bpp clipboard images fully
		// transparent.
		palette[i] = color.NRGBA{R: buf[off+2], G: buf[off+1], B: buf[off], A: 255}
	}
	alphaAsAlpha := false
	if bpp == 32 {
		if aMask != 0 || compression == 6 {
			alphaAsAlpha = true
		} else if compression == biRGB {
			allZero := true
			for y := 0; y < height && allZero; y++ {
				row := buf[pixelOffset+y*stride:]
				for x := 0; x < width; x++ {
					if row[x*4+3] != 0 {
						allZero = false
						break
					}
				}
			}
			alphaAsAlpha = !allZero
		}
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		srcRow := y
		if !topDown {
			srcRow = height - 1 - y
		}
		row := buf[pixelOffset+srcRow*stride:]
		for x := 0; x < width; x++ {
			var c color.NRGBA
			switch bpp {
			case 32:
				v := binary.LittleEndian.Uint32(row[x*4 : x*4+4])
				if rMask != 0 || gMask != 0 || bMask != 0 {
					c = color.NRGBA{R: maskTo8(v, rMask), G: maskTo8(v, gMask), B: maskTo8(v, bMask), A: 255}
					if aMask != 0 {
						c.A = maskTo8(v, aMask)
					}
				} else {
					c = color.NRGBA{R: row[x*4+2], G: row[x*4+1], B: row[x*4], A: 255}
					if alphaAsAlpha {
						c.A = row[x*4+3]
					}
				}
			case 24:
				o := x * 3
				c = color.NRGBA{R: row[o+2], G: row[o+1], B: row[o], A: 255}
			case 16:
				v := uint32(binary.LittleEndian.Uint16(row[x*2 : x*2+2]))
				c = color.NRGBA{R: maskTo8(v, rMask), G: maskTo8(v, gMask), B: maskTo8(v, bMask), A: 255}
			case 8:
				if x < width && int(row[x]) < len(palette) {
					c = palette[row[x]]
				}
			case 4:
				b := row[x/2]
				n := b >> 4
				if x%2 == 1 {
					n = b & 0xF
				}
				if int(n) < len(palette) {
					c = palette[n]
				}
			case 1:
				n := (row[x/8] >> (7 - uint(x%8))) & 1
				if int(n) < len(palette) {
					c = palette[n]
				}
			}
			o := img.PixOffset(x, y)
			img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = c.R, c.G, c.B, c.A
		}
	}
	return img, nil
}

func maskTo8(v, mask uint32) byte {
	if mask == 0 {
		return 0
	}
	shift := uint(0)
	for mask&1 == 0 {
		mask >>= 1
		shift++
	}
	bits := uint(0)
	for mask&1 == 1 {
		mask >>= 1
		bits++
	}
	if bits == 0 {
		return 0
	}
	val := (v >> shift) & ((uint32(1) << bits) - 1)
	if bits >= 8 {
		return byte(val >> (bits - 8))
	}
	return byte(val * 255 / ((uint32(1) << bits) - 1))
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
