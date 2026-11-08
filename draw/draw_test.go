package draw

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
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

func TestFastDrawUsesBoundsAndStrideOffsets(t *testing.T) {
	for _, overlap := range []bool{false, true} {
		name := "separate images"
		if overlap {
			name = "overlapping image"
		}
		t.Run(name, func(t *testing.T) {
			got := newStridedCRGB16(image.Rect(10, 20, 15, 24), 4)
			want := newStridedCRGB16(got.Bounds(), 4)
			for y := got.Bounds().Min.Y; y < got.Bounds().Max.Y; y++ {
				for x := got.Bounds().Min.X; x < got.Bounds().Max.X; x++ {
					c := pixel.CRGB16{V: uint16((y-got.Bounds().Min.Y)*17 + x - got.Bounds().Min.X)}
					got.Set(x, y, c)
					want.Set(x, y, c)
				}
			}
			r := image.Rect(11, 21, 15, 24)
			sp := image.Pt(10, 20)
			var src image.Image = newStridedCRGB16(image.Rect(10, 20, 15, 24), 6)
			if overlap {
				src = got
				sp = image.Pt(10, 20)
			}
			if distinct, ok := src.(*stridedCRGB16); ok && !overlap {
				for y := distinct.Bounds().Min.Y; y < distinct.Bounds().Max.Y; y++ {
					for x := distinct.Bounds().Min.X; x < distinct.Bounds().Max.X; x++ {
						distinct.Set(x, y, pixel.CRGB16{V: uint16(0x400 + (y-distinct.Bounds().Min.Y)*17 + x - distinct.Bounds().Min.X)})
					}
				}
			}
			if overlap {
				wantSrc := newStridedCRGB16(got.Bounds(), 4)
				copy(wantSrc.pix, got.pix)
				Draw(got, r, src, sp, Src)
				stdDraw.Draw(want, r, wantSrc, sp, stdDraw.Src)
			} else {
				Draw(got, r, src, sp, Src)
				stdDraw.Draw(want, r, src, sp, stdDraw.Src)
			}
			if !bytes.Equal(got.pix, want.pix) {
				for y := got.Bounds().Min.Y; y < got.Bounds().Max.Y; y++ {
					for x := got.Bounds().Min.X; x < got.Bounds().Max.X; x++ {
						if got.At(x, y) != want.At(x, y) {
							t.Errorf("pixel (%d,%d): got %v want %v", x, y, got.At(x, y), want.At(x, y))
						}
					}
				}
				t.Fatal("fast draw differs from image/draw with offset bounds and padded stride")
			}
		})
	}
}

func TestFastUniformAndRGBAUseDestinationOffsets(t *testing.T) {
	for _, sourceKind := range []string{"uniform", "rgba"} {
		t.Run(sourceKind, func(t *testing.T) {
			got := newStridedCRGB16(image.Rect(10, 20, 15, 24), 6)
			want := newStridedCRGB16(got.Bounds(), 6)
			for y := got.Bounds().Min.Y; y < got.Bounds().Max.Y; y++ {
				for x := got.Bounds().Min.X; x < got.Bounds().Max.X; x++ {
					c := pixel.CRGB16{V: uint16((y-got.Bounds().Min.Y)*17 + x - got.Bounds().Min.X)}
					got.Set(x, y, c)
					want.Set(x, y, c)
				}
			}
			var src image.Image
			sp := image.Pt(2, 1)
			if sourceKind == "uniform" {
				src = image.NewUniform(color.NRGBA{R: 211, G: 83, B: 149, A: 121})
				sp = image.Pt(0, 0)
			} else {
				src = image.NewRGBA(image.Rect(1, 1, 7, 6))
				rgba := src.(*image.RGBA)
				for y := rgba.Bounds().Min.Y; y < rgba.Bounds().Max.Y; y++ {
					for x := rgba.Bounds().Min.X; x < rgba.Bounds().Max.X; x++ {
						rgba.Set(x, y, color.RGBA{R: uint8(x*31 + y), G: uint8(y*47 + x), B: uint8(x*13 + y*19), A: uint8(70 + x*9)})
					}
				}
			}
			r := image.Rect(9, 21, 15, 25)
			Draw(got, r, src, sp, Src)
			stdDraw.Draw(want, r, src, sp, stdDraw.Src)
			if !bytes.Equal(got.pix, want.pix) {
				t.Fatal("fast draw differs from image/draw with nonzero destination bounds and padded stride")
			}
		})
	}
}

func TestFastDrawHandlesDifferentByteOrders(t *testing.T) {
	dst := newStridedCRGB16Order(image.Rect(0, 0, 4, 3), 2, binary.BigEndian)
	src := newStridedCRGB16Order(image.Rect(0, 0, 4, 3), 4, binary.LittleEndian)
	want := newStridedCRGB16Order(dst.Bounds(), 2, binary.BigEndian)
	for y := 0; y < 3; y++ {
		for x := 0; x < 4; x++ {
			src.Set(x, y, pixel.CRGB16{V: uint16(0x1200 + y*16 + x)})
		}
	}
	Draw(dst, dst.Bounds(), src, image.Point{}, Src)
	stdDraw.Draw(want, want.Bounds(), src, image.Point{}, stdDraw.Src)
	if !bytes.Equal(dst.pix, want.pix) {
		t.Fatal("draw with differing source and destination byte orders differs from image/draw")
	}
}

func TestFastDrawFallsBackForMalformedPixelViews(t *testing.T) {
	for _, tc := range []struct {
		name     string
		badView  string
		badImage string
	}{
		{"destination bounds mismatch", "bounds", "destination"},
		{"destination short storage", "storage", "destination"},
		{"source bounds mismatch", "bounds", "source"},
		{"source short storage", "storage", "source"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bounds := image.Rect(0, 0, 5, 4)
			gotSrc := newMalformedFastRGBA(bounds, tc.badImage == "source", tc.badView)
			wantSrc := gotSrc.RGBA
			for y := 0; y < bounds.Dy(); y++ {
				for x := 0; x < bounds.Dx(); x++ {
					c := color.RGBA{R: uint8(21 + x*23), G: uint8(11 + y*37), B: uint8(x*9 + y*17), A: 255}
					gotSrc.SetRGBA(x, y, c)
				}
			}

			var got, want Image
			if tc.badImage == "destination" {
				got = newMalformedFastRGBA(bounds, true, tc.badView)
				want = image.NewRGBA(bounds)
			} else {
				got = newStridedCRGB16(bounds, 4)
				want = newStridedCRGB16(bounds, 4)
			}
			for y := 0; y < bounds.Dy(); y++ {
				for x := 0; x < bounds.Dx(); x++ {
					c := color.RGBA{R: uint8(x * 19), G: uint8(y * 29), B: uint8(x*7 + y*13), A: 255}
					got.Set(x, y, c)
					want.Set(x, y, c)
				}
			}
			r, sp := image.Rect(-1, 1, 5, 4), image.Pt(0, 0)
			Draw(got, r, gotSrc, sp, Src)
			stdDraw.Draw(want, r, wantSrc, sp, stdDraw.Src)
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					if got.At(x, y) != want.At(x, y) {
						t.Fatalf("pixel (%d,%d): got %v want %v", x, y, got.At(x, y), want.At(x, y))
					}
				}
			}
		})
	}
}

type malformedFastRGBA struct {
	*image.RGBA
	view pixel.PixelView
}

func newMalformedFastRGBA(bounds image.Rectangle, malformed bool, fault string) *malformedFastRGBA {
	img := image.NewRGBA(bounds)
	viewBounds := bounds
	stride := bounds.Dx() * 2
	pix := make([]byte, stride*bounds.Dy())
	if malformed && fault == "bounds" {
		viewBounds = viewBounds.Add(image.Pt(1, 0))
	}
	if malformed && fault == "storage" {
		pix = pix[:1]
	}
	return &malformedFastRGBA{
		RGBA: img,
		view: pixel.PixelView{Pix: pix, Stride: stride, Bounds: viewBounds, Format: pixel.FormatCRGB16, Order: binary.BigEndian},
	}
}

func (img *malformedFastRGBA) PixelView() pixel.PixelView { return img.view }

type stridedCRGB16 struct {
	bounds image.Rectangle
	stride int
	pix    []byte
	order  binary.ByteOrder
}

func newStridedCRGB16(bounds image.Rectangle, padding int) *stridedCRGB16 {
	return newStridedCRGB16Order(bounds, padding, binary.BigEndian)
}

func newStridedCRGB16Order(bounds image.Rectangle, padding int, order binary.ByteOrder) *stridedCRGB16 {
	stride := bounds.Dx()*2 + padding
	return &stridedCRGB16{bounds: bounds, stride: stride, pix: make([]byte, stride*bounds.Dy()), order: order}
}

func (img *stridedCRGB16) ColorModel() color.Model { return pixel.CRGB16Model }
func (img *stridedCRGB16) Bounds() image.Rectangle { return img.bounds }
func (img *stridedCRGB16) PixelView() pixel.PixelView {
	return pixel.PixelView{Pix: img.pix, Stride: img.stride, Bounds: img.bounds, Format: pixel.FormatCRGB16, Order: img.order}
}
func (img *stridedCRGB16) At(x, y int) color.Color {
	if !image.Pt(x, y).In(img.bounds) {
		return color.Transparent
	}
	offset := (y-img.bounds.Min.Y)*img.stride + (x-img.bounds.Min.X)*2
	return pixel.CRGB16{V: img.order.Uint16(img.pix[offset:])}
}
func (img *stridedCRGB16) Set(x, y int, c color.Color) {
	if !image.Pt(x, y).In(img.bounds) {
		return
	}
	offset := (y-img.bounds.Min.Y)*img.stride + (x-img.bounds.Min.X)*2
	img.order.PutUint16(img.pix[offset:], pixel.CRGB16Model.Convert(c).(pixel.CRGB16).V)
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
