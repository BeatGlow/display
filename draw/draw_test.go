package draw

import (
	"bytes"
	"image"
	"image/color"
	stdDraw "image/draw"
	"testing"

	"github.com/BeatGlow/display/v2/pixel"
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

func TestFastDrawUniformAndRGBA(t *testing.T) {
	formats := []struct {
		name string
		make func() drawImage
	}{
		{"CBGR15", func() drawImage { return pixel.NewCBGR15Image(9, 7) }},
		{"CBGR16", func() drawImage { return pixel.NewCBGR16Image(9, 7) }},
		{"CRGB15", func() drawImage { return pixel.NewCRGB15Image(9, 7) }},
		{"CRGB16", func() drawImage { return pixel.NewCRGB16Image(9, 7) }},
	}
	for _, format := range formats {
		for _, source := range []string{"uniform", "rgba"} {
			t.Run(format.name+"/"+source, func(t *testing.T) {
				got, want := format.make(), format.make()
				gv := got.(pixel.FastImage).PixelView()
				wv := want.(pixel.FastImage).PixelView()
				for i := range gv.Pix {
					gv.Pix[i] = byte(i*29 + 7)
					wv.Pix[i] = gv.Pix[i]
				}
				r := image.Rect(-2, 1, 8, 6)
				var src image.Image
				sp := image.Pt(1, 0)
				if source == "uniform" {
					src = image.NewUniform(color.NRGBA{R: 231, G: 77, B: 159, A: 103})
				} else {
					parent := image.NewRGBA(image.Rect(0, 0, 12, 11))
					rgba := parent.SubImage(image.Rect(3, 4, 11, 10)).(*image.RGBA)
					for y := rgba.Bounds().Min.Y; y < rgba.Bounds().Max.Y; y++ {
						for x := rgba.Bounds().Min.X; x < rgba.Bounds().Max.X; x++ {
							rgba.Set(x, y, color.NRGBA{R: uint8(x*17 + y), G: uint8(y*31 + x), B: uint8(x*7 + y*13), A: uint8(80 + x*9)})
						}
					}
					src, sp = rgba, image.Pt(4, 4)
				}
				Draw(got, r, src, sp, Src)
				stdDraw.Draw(want, r, src, sp, stdDraw.Src)
				if !bytes.Equal(gv.Pix, wv.Pix) {
					t.Fatal("fast path differs from image/draw reference")
				}
			})
		}
	}
}

type drawImage interface {
	stdDraw.Image
	PixelView() pixel.PixelView
}

func BenchmarkUniformSpanCRGB16(b *testing.B) {
	benchmarkDrawPaths(b, image.NewUniform(color.RGBA{R: 200, G: 90, B: 35, A: 255}))
}

func BenchmarkRGBAtoCRGB16(b *testing.B) {
	src := image.NewRGBA(image.Rect(0, 0, 400, 128))
	benchmarkDrawPaths(b, src)
}

func benchmarkDrawPaths(b *testing.B, src image.Image) {
	for _, tc := range []struct {
		name string
		draw func(*pixel.CRGB16Image)
	}{
		{"Fast", func(dst *pixel.CRGB16Image) { Draw(dst, dst.Bounds(), src, src.Bounds().Min, Src) }},
		{"Standard", func(dst *pixel.CRGB16Image) { stdDraw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, stdDraw.Src) }},
	} {
		b.Run(tc.name, func(b *testing.B) {
			dst := pixel.NewCRGB16Image(400, 128)
			b.SetBytes(400 * 128 * 2)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				tc.draw(dst)
			}
		})
	}
}

func TestDrawMaskFallbackMatchesStandard(t *testing.T) {
	for _, tc := range []struct {
		name string
		mask image.Image
		op   Op
	}{
		{"over-alpha", nil, Over},
		{"masked-src", image.NewUniform(color.Alpha{A: 128}), Src},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, want := pixel.NewCRGB16Image(5, 3), pixel.NewCRGB16Image(5, 3)
			src := image.NewRGBA(image.Rect(0, 0, 5, 3))
			for y := 0; y < 3; y++ {
				for x := 0; x < 5; x++ {
					base := color.RGBA{R: uint8(20 + x*20), G: uint8(200 - y*30), B: uint8(x*13 + y*9), A: 255}
					got.Set(x, y, base)
					want.Set(x, y, base)
					src.Set(x, y, color.RGBA{R: uint8(x * 40), G: uint8(y * 70), B: 230, A: uint8(60 + x*35)})
				}
			}
			r := image.Rect(1, 0, 5, 3)
			DrawMask(got, r, src, image.Point{}, tc.mask, image.Point{}, tc.op)
			stdDraw.DrawMask(want, r, src, image.Point{}, tc.mask, image.Point{}, stdDraw.Op(tc.op))
			if !bytes.Equal(got.Pix, want.Pix) {
				t.Fatal("fallback result differs from image/draw reference")
			}
		})
	}
}
