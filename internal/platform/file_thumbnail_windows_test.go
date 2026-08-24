//go:build windows

package platform

import (
	"image"
	"image/color"
	"testing"
)

func TestResizeNearestDownscalesToMaxSide(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 480, 240)) // 2:1 landscape
	got := resizeNearest(src, 240)
	b := got.Bounds()
	if b.Dx() != 240 || b.Dy() != 120 {
		t.Fatalf("resized bounds = %dx%d, want 240x120 (aspect ratio preserved)", b.Dx(), b.Dy())
	}
}

func TestResizeNearestLeavesSmallImagesAlone(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 40, 20))
	got := resizeNearest(src, 240)
	b := got.Bounds()
	if b.Dx() != 40 || b.Dy() != 20 {
		t.Fatalf("resized bounds = %dx%d, want the original 40x20 (already under the max)", b.Dx(), b.Dy())
	}
}

func TestResizeNearestSamplesCorrectQuadrant(t *testing.T) {
	// A 4x4 image downscaled to 2x2 buckets each source pixel into one of four 2x2 quadrants;
	// nearest-neighbour sampling should keep top-left red and bottom-right blue on their own
	// sides rather than blending or transposing them.
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	red, blue := color.RGBA{255, 0, 0, 255}, color.RGBA{0, 0, 255, 255}
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			src.Set(x, y, red)
			src.Set(x+2, y+2, blue)
		}
	}

	got := resizeNearest(src, 2)
	if r, g, b, a := got.At(0, 0).RGBA(); byte(r>>8) != 255 || byte(g>>8) != 0 || byte(b>>8) != 0 || byte(a>>8) != 255 {
		t.Errorf("top-left = (%d,%d,%d,%d), want red", r>>8, g>>8, b>>8, a>>8)
	}
	if r, g, b, a := got.At(1, 1).RGBA(); byte(r>>8) != 0 || byte(g>>8) != 0 || byte(b>>8) != 255 || byte(a>>8) != 255 {
		t.Errorf("bottom-right = (%d,%d,%d,%d), want blue", r>>8, g>>8, b>>8, a>>8)
	}
}
