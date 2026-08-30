//go:build windows

package platform

import (
	"bytes"
	"encoding/binary"
	"image"
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

func TestDecodeDIBPaletteIsOpaque(t *testing.T) {
	// A classic 2x1 8bpp DIB stores palette entries as B,G,R,0 RGBQUADs. The
	// reserved zero byte must not be interpreted as an alpha channel.
	hdr := bitmapInfoHeader{Size: 40, Width: 2, Height: 1, Planes: 1, BitCount: 8, Compression: biRGB, ClrUsed: 2}
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, hdr); err != nil {
		t.Fatal(err)
	}
	buf.Write([]byte{255, 0, 0, 0, 0, 0, 255, 0}) // blue, red
	buf.Write([]byte{0, 1, 0, 0})                 // pixels + row padding
	img, err := decodeDIB(buf.Bytes())
	if err != nil {
		t.Fatalf("decodeDIB() error = %v", err)
	}
	assertPixel(t, img, 0, 0, rgb{0, 0, 255})
	assertPixel(t, img, 1, 0, rgb{255, 0, 0})
}

func TestDecodeDIB32BitAlpha(t *testing.T) {
	hdr := bitmapInfoHeader{Size: 40, Width: 1, Height: -1, Planes: 1, BitCount: 32, Compression: biRGB}
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, hdr); err != nil {
		t.Fatal(err)
	}
	buf.Write([]byte{0, 255, 0, 128}) // BGRA: straight green at 50% alpha
	img, err := decodeDIB(buf.Bytes())
	if err != nil {
		t.Fatalf("decodeDIB() error = %v", err)
	}
	pixel, ok := img.(*image.NRGBA)
	if !ok {
		t.Fatalf("decodeDIB() type = %T, want *image.NRGBA", img)
	}
	got := pixel.NRGBAAt(0, 0)
	if got != (color.NRGBA{R: 0, G: 255, B: 0, A: 128}) {
		t.Fatalf("pixel = %+v, want straight RGBA %+v", got, color.NRGBA{R: 0, G: 255, B: 0, A: 128})
	}
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
