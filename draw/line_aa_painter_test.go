package draw_test

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"math/rand"
	"testing"

	"github.com/BeatGlow/display/draw"
	"github.com/BeatGlow/display/pixel"
)

type aaStroke struct {
	a, b  image.Point
	width int
	ink   color.RGBA
}

func TestLineAAPainterPreservesStrokePixels(t *testing.T) {
	rect := image.Rect(0, 0, 96, 96)
	for _, tc := range []struct {
		name string
		new  func() (draw.Image, []byte)
	}{
		{"RGBA", func() (draw.Image, []byte) {
			dst := image.NewRGBA(rect)
			return dst, dst.Pix
		}},
		{"clipped RGBA subimage", func() (draw.Image, []byte) {
			parent := image.NewRGBA(rect.Add(image.Pt(-48, -48)))
			return parent.SubImage(parent.Bounds().Inset(9)).(*image.RGBA), parent.Pix
		}},
		{"RGBA with large coordinates", func() (draw.Image, []byte) {
			dst := image.NewRGBA(rect.Add(image.Pt(math.MaxInt-256, math.MinInt+256)))
			return dst, dst.Pix
		}},
		{"legacy packed RGB", func() (draw.Image, []byte) {
			dst := pixel.NewCRGB16Image(rect.Dx(), rect.Dy())
			return dst, dst.Pix
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target, actual := tc.new()
			reference, expected := tc.new()
			for i := range actual {
				actual[i] = byte(0x77 + i%16)
				expected[i] = actual[i]
			}
			bounds := target.Bounds()
			center := bounds.Min.Add(image.Pt(16, 16))
			halo := color.RGBA{R: 18, G: 72, B: 48, A: 88}
			core := color.RGBA{R: 240, G: 180, B: 120, A: 255}
			other := color.RGBA{R: 36, G: 14, B: 62, A: 100}
			painter := draw.NewLineAAPainter(target)
			check := func(stroke aaStroke) {
				t.Helper()
				painter.Line(stroke.a, stroke.b, stroke.width, stroke.ink)
				draw.LineAA(reference, stroke.a, stroke.b, stroke.width, stroke.ink)
				if !bytes.Equal(actual, expected) {
					t.Fatalf("stroke %+v changed rendered pixels relative to LineAA", stroke)
				}
			}
			// Repeat a closed path with overlapping translucent and opaque layers.
			path := []image.Point{
				center, center.Add(image.Pt(8, -6)), center.Add(image.Pt(5, 28)),
				center.Add(image.Pt(-22, 10)), center,
			}
			for _, layer := range []struct {
				width int
				ink   color.RGBA
			}{{5, halo}, {1, core}} {
				for i := 1; i < len(path); i++ {
					check(aaStroke{path[i-1], path[i], layer.width, layer.ink})
				}
			}

			// Cover each major-axis distance in both directions and axis
			// orders, moving the starts around pixel and clipping boundaries.
			rng := rand.New(rand.NewSource(26))
			for length := 1; length <= 96; length++ {
				for i := range 4 {
					a := bounds.Min.Add(image.Pt(rng.Intn(106)-13, rng.Intn(106)-13))
					delta := image.Pt(length, rng.Intn(2*length+1)-length)
					if i%2 != 0 {
						delta.X, delta.Y = delta.Y, delta.X
					}
					b := a.Add(delta)
					if i >= 2 {
						a, b = b, a
					}
					ink := [...]color.RGBA{halo, core, other}[i%3]
					check(aaStroke{a, b, length % 6, ink})
				}
			}
			check(aaStroke{center, center, 4, other})
			check(aaStroke{bounds.Max, bounds.Max, 1, core})
			check(aaStroke{bounds.Min, center, 1, color.RGBA{}})
			check(aaStroke{image.Pt(math.MinInt, center.Y), image.Pt(math.MaxInt, center.Y), 2, halo})
		})
	}
}

func TestLineAAPainterKeepsItsPreparedRGBAView(t *testing.T) {
	dst := image.NewRGBA(image.Rect(0, 0, 24, 24))
	pixels := dst.Pix
	painter := draw.NewLineAAPainter(dst)
	want := image.NewRGBA(dst.Bounds())
	*dst = *image.NewRGBA(image.Rect(0, 0, 1, 1))

	a, b := image.Pt(0, 23), image.Pt(23, 12)
	ink := color.RGBA{R: 30, G: 80, B: 140, A: 160}
	draw.LineAA(want, a, b, 3, ink)
	painter.Line(a, b, 3, ink)
	if !bytes.Equal(pixels, want.Pix) || !bytes.Equal(dst.Pix, make([]byte, len(dst.Pix))) {
		t.Fatal("rebinding the original image changed which view the painter rendered")
	}
}

func BenchmarkLineAAPainter(b *testing.B) {
	for _, lengths := range []struct {
		name     string
		min, max int
	}{
		{"short", 1, 3},
		{"medium", 4, 16},
		{"long", 17, 64},
	} {
		b.Run(lengths.name, func(b *testing.B) {
			rng := rand.New(rand.NewSource(7))
			palette := [...]color.RGBA{
				{R: 24, G: 38, B: 18, A: 50},
				{R: 47, G: 102, B: 60, A: 146},
				{R: 110, G: 70, B: 210, A: 255},
				{R: 58, G: 19, B: 42, A: 90},
			}
			var strokes [128]aaStroke
			for i := range strokes {
				a := image.Pt(80+rng.Intn(64), 80+rng.Intn(64))
				length := lengths.min + rng.Intn(lengths.max-lengths.min+1)
				delta := image.Pt(length, rng.Intn(2*length+1)-length)
				if i%2 != 0 {
					delta.X, delta.Y = delta.Y, delta.X
				}
				end := a.Add(delta)
				if i%4 >= 2 {
					a, end = end, a
				}
				strokes[i] = aaStroke{a, end, 1 + i%4, palette[i/16%len(palette)]}
			}
			b.Run("LineAA", func(b *testing.B) {
				dst := image.NewRGBA(image.Rect(0, 0, 256, 256))
				b.ReportAllocs()
				for b.Loop() {
					for _, stroke := range strokes {
						draw.LineAA(dst, stroke.a, stroke.b, stroke.width, stroke.ink)
					}
				}
			})
			b.Run("Painter", func(b *testing.B) {
				dst := image.NewRGBA(image.Rect(0, 0, 256, 256))
				painter := draw.NewLineAAPainter(dst)
				b.ReportAllocs()
				for b.Loop() {
					for _, stroke := range strokes {
						painter.Line(stroke.a, stroke.b, stroke.width, stroke.ink)
					}
				}
			})
		})
	}
}
