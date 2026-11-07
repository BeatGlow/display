package draw

import (
	"image"
	stdDraw "image/draw"
	"testing"

	"github.com/BeatGlow/display/pixel"
)

func TestFastDrawMatchesStandardAndOverlap(t *testing.T) {
	for _, tc := range []struct {
		name    string
		r       image.Rectangle
		sp      image.Point
		overlap bool
	}{
		{"clip", image.Rect(-2, 1, 8, 6), image.Pt(1, 0), false},
		{"overlap-down", image.Rect(0, 1, 8, 8), image.Pt(0, 0), true},
		{"overlap-up", image.Rect(0, 0, 8, 7), image.Pt(0, 1), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, want := pixel.NewCRGB16Image(8, 8), pixel.NewCRGB16Image(8, 8)
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					c := pixel.CRGB16{V: uint16(y*8 + x)}
					got.Set(x, y, c)
					want.Set(x, y, c)
				}
			}
			if tc.overlap {
				Draw(got, tc.r, got, tc.sp, Src)
				stdDraw.Draw(want, tc.r, want, tc.sp, stdDraw.Src)
			} else {
				src := pixel.NewCRGB16Image(8, 8)
				for y := 0; y < 8; y++ {
					for x := 0; x < 8; x++ {
						src.Set(x, y, pixel.CRGB16{V: uint16(0x100 + y*8 + x)})
					}
				}
				Draw(got, tc.r, src, tc.sp, Src)
				stdDraw.Draw(want, tc.r, src, tc.sp, stdDraw.Src)
			}
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					if got.At(x, y) != want.At(x, y) {
						t.Fatalf("pixel (%d,%d): got %v want %v", x, y, got.At(x, y), want.At(x, y))
					}
				}
			}
		})
	}
}

func BenchmarkBlitCRGB16(b *testing.B) {
	dst, src := pixel.NewCRGB16Image(400, 128), pixel.NewCRGB16Image(400, 128)
	for _, tc := range []struct {
		name string
		draw func()
	}{
		{"Fast", func() { Draw(dst, dst.Bounds(), src, image.Point{}, Src) }},
		{"Standard", func() { stdDraw.Draw(dst, dst.Bounds(), src, image.Point{}, stdDraw.Src) }},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(400 * 128 * 2)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				tc.draw()
			}
		})
	}
}
