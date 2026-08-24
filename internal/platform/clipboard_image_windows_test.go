//go:build windows

package platform

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"testing"
)

type rgb struct{ r, g, b byte }

// buildDIB assembles a minimal CF_DIB byte buffer (BITMAPINFOHEADER + 24bpp pixel data, no
// colour table) for a 2x2 image, in either bottom-up (topDown=false, the common case Windows
// actually produces) or top-down row order, so decodeDIB can be exercised without any real
// clipboard or GDI call.
func buildDIB(t *testing.T, topLeft, topRight, bottomLeft, bottomRight rgb, topDown bool) []byte {
	t.Helper()
	const width, height = 2, 2
	rowSize := ((width*24 + 31) / 32) * 4 // = 8

	writeRow := func(buf *bytes.Buffer, left, right rgb) {
		buf.WriteByte(left.b)
		buf.WriteByte(left.g)
		buf.WriteByte(left.r)
		buf.WriteByte(right.b)
		buf.WriteByte(right.g)
		buf.WriteByte(right.r)
		buf.Write(make([]byte, rowSize-6)) // pad to the 4-byte row boundary
	}

	var pixels bytes.Buffer
	if topDown {
		writeRow(&pixels, topLeft, topRight)
		writeRow(&pixels, bottomLeft, bottomRight)
	} else {
		writeRow(&pixels, bottomLeft, bottomRight)
		writeRow(&pixels, topLeft, topRight)
	}

	hdr := bitmapInfoHeader{Size: 40, Width: width, Height: height, Planes: 1, BitCount: 24, Compression: biRGB}
	if topDown {
		hdr.Height = -height
	}

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, hdr); err != nil {
		t.Fatalf("encode header: %v", err)
	}
	buf.Write(pixels.Bytes())
	return buf.Bytes()
}

func TestDecodeDIBBottomUp(t *testing.T) {
	red, green, blue, white := rgb{255, 0, 0}, rgb{0, 255, 0}, rgb{0, 0, 255}, rgb{255, 255, 255}
	buf := buildDIB(t, red, green, blue, white, false)

	img, err := decodeDIB(buf)
	if err != nil {
		t.Fatalf("decodeDIB() error = %v", err)
	}
	assertPixel(t, img, 0, 0, red)
	assertPixel(t, img, 1, 0, green)
	assertPixel(t, img, 0, 1, blue)
	assertPixel(t, img, 1, 1, white)
}

func TestDecodeDIBTopDown(t *testing.T) {
	red, green, blue, white := rgb{255, 0, 0}, rgb{0, 255, 0}, rgb{0, 0, 255}, rgb{255, 255, 255}
	buf := buildDIB(t, red, green, blue, white, true)

	img, err := decodeDIB(buf)
	if err != nil {
		t.Fatalf("decodeDIB() error = %v", err)
	}
	assertPixel(t, img, 0, 0, red)
	assertPixel(t, img, 1, 0, green)
	assertPixel(t, img, 0, 1, blue)
	assertPixel(t, img, 1, 1, white)
}

func assertPixel(t *testing.T, img interface {
	At(x, y int) color.Color
}, x, y int, want rgb) {
	t.Helper()
	r, g, b, a := img.At(x, y).RGBA()
	got := rgb{byte(r >> 8), byte(g >> 8), byte(b >> 8)}
	if got != want || a>>8 != 255 {
		t.Errorf("pixel(%d,%d) = %+v a=%d, want %+v a=255", x, y, got, a>>8, want)
	}
}
