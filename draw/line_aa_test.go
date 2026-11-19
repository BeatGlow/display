package draw_test

import (
	"image"
	"image/color"
	stddraw "image/draw"
	"testing"

	"github.com/BeatGlow/display/draw"
)

// Expose only image/draw.Image so the same expected pixels exercise the fallback.
type strokeImage struct{ stddraw.Image }

func TestLineAAClipsAndBlends(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		steep, reverse, generic bool
	}{
		{name: "RGBA"},
		{name: "reversed", reverse: true},
		{name: "steep", steep: true},
		{name: "generic", steep: true, reverse: true, generic: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := image.NewRGBA(image.Rect(10, 20, 17, 27))
			background := color.RGBA{R: 20, G: 40, B: 60, A: 160}
			stddraw.Draw(parent, parent.Bounds(), image.NewUniform(background), image.Point{}, stddraw.Src)
			expected := append([]byte(nil), parent.Pix...)
			rect := image.Rect(12, 22, 16, 26)
			var dst stddraw.Image = parent.SubImage(rect).(*image.RGBA)
			if tc.generic {
				dst = strokeImage{dst}
			}
			point := func(x, y int) image.Point {
				if tc.steep {
					x, y = y, x
				}
				return rect.Min.Add(image.Pt(x, y))
			}
			a, b := point(-1, 0), point(3, 2)
			if tc.reverse {
				a, b = b, a
			}
			ink := color.RGBA{R: 100, G: 60, B: 40, A: 128}
			draw.LineAA(dst, a, b, 1, ink)
			// The line rises half a pixel per major-axis step. At the
			// midpoints it splits coverage equally between adjacent pixels.
			for _, sample := range []struct {
				x, y int
				half bool
			}{
				{0, 0, true}, {0, 1, true}, {1, 1, false}, {2, 1, true}, {2, 2, true}, {3, 2, false},
			} {
				p := point(sample.x, sample.y)
				offset := parent.PixOffset(p.X, p.Y)
				c := color.RGBA{R: 110, G: 80, B: 70, A: 208}
				if sample.half {
					c = color.RGBA{R: 65, G: 60, B: 65, A: 184}
				}
				copy(expected[offset:offset+4], []byte{c.R, c.G, c.B, c.A})
			}
			for i, want := range expected {
				if got := parent.Pix[i]; got != want {
					t.Fatalf("byte %d of pixel buffer = %d, want %d", i, got, want)
				}
			}
		})
	}
}

func TestLineAAStrokeWidthAndEndpoints(t *testing.T) {
	dst := image.NewRGBA(image.Rect(0, 0, 4, 5))
	ink := color.RGBA{R: 255, A: 255}
	draw.LineAA(dst, image.Pt(-200, 2), image.Pt(200, 2), 2, ink)
	draw.LineAA(dst, image.Pt(1, 0), image.Pt(1, 0), 4, ink)
	for y := 0; y < 5; y++ {
		for x := 0; x < 4; x++ {
			var level uint8
			switch {
			case y == 2 || (x == 1 && y == 0):
				level = 255
			case y == 1 || y == 3:
				level = 127
			}
			want := color.RGBA{R: level, A: level}
			if got := dst.RGBAAt(x, y); got != want {
				t.Errorf("(%d,%d) = %v, want %v", x, y, got, want)
			}
		}
	}
}

func BenchmarkLineAAShortSegments(b *testing.B) {
	dst := image.NewRGBA(image.Rect(0, 0, 400, 1280))
	points := [...]image.Point{
		{X: 120, Y: 88}, {X: 120, Y: 88}, {X: 122, Y: 89},
		{X: 121, Y: 95}, {X: 113, Y: 93}, {X: 115, Y: 92},
		{X: 115, Y: 91}, {X: 116, Y: 91}, {X: 120, Y: 88},
	}
	ink := color.RGBA{R: 10, G: 30, B: 20, A: 36}
	b.ReportAllocs()
	for b.Loop() {
		for i := 1; i < len(points); i++ {
			draw.LineAA(dst, points[i-1], points[i], 2, ink)
		}
	}
}
