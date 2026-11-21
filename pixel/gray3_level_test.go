package pixel_test

import (
	"bytes"
	"image/color"
	"testing"

	"github.com/BeatGlow/display/pixel"
)

func TestGray3LevelImageKeepsHalfBrightnessAndPacksOddRows(t *testing.T) {
	img := pixel.NewGray3LevelImage(5, 2)
	img.Fill(color.Gray{Y: 128})
	img.Set(0, 0, color.Black)
	img.Set(1, 0, color.White)
	img.Set(4, 1, pixel.Gray3Level{Y: 3})

	view := img.PixelView()
	if want := []byte{0x25, 0x55, 0x55, 0x95}; view.Format != pixel.FormatGray3Level || view.Stride != 2 || !bytes.Equal(view.Pix, want) {
		t.Fatalf("packed image has format %d, stride %d, bytes %x; want format %d, stride 2, bytes %x", view.Format, view.Stride, view.Pix, pixel.FormatGray3Level, want)
	}
	for _, tc := range []struct {
		name string
		x, y int
		want uint32
	}{
		{"off", 0, 0, 0},
		{"half", 0, 1, 0x8000},
		{"full", 1, 0, 0xffff},
		{"out-of-range mapped to full", 4, 1, 0xffff},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, g, b, a := img.At(tc.x, tc.y).RGBA()
			if r != tc.want || g != tc.want || b != tc.want || a != 0xffff {
				t.Errorf("RGBA = %x,%x,%x,%x, want opaque grayscale %x", r, g, b, a, tc.want)
			}
		})
	}
}
