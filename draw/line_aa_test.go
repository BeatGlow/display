package draw_test

import (
	"bytes"
	"image"
	"image/color"
	stddraw "image/draw"
	"math"
	"math/rand"
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

// Overlapping translucent strokes must produce the same pixels on a direct
// RGBA surface and through the generic draw.Image path, including at clip edges.
func TestLineAAOverlappingStrokesMatchGenericSurface(t *testing.T) {
	rect := image.Rect(-30, 17, 31, 86)
	direct, ref := image.NewRGBA(rect), image.NewRGBA(rect)
	bounds := rect.Inset(3)
	target := direct.SubImage(bounds).(*image.RGBA)
	reference := strokeImage{ref.SubImage(bounds).(*image.RGBA)}
	rng := rand.New(rand.NewSource(123))
	point := func() image.Point { return image.Pt(rng.Intn(105)-52, rng.Intn(100)) }
	for i := 0; i < 500; i++ {
		a, b := point(), point()
		width, alpha := rng.Intn(10), rng.Intn(256)
		channel := func() uint8 { return uint8(rng.Intn(alpha + 1)) }
		ink := color.RGBA{R: channel(), G: channel(), B: channel(), A: uint8(alpha)}
		draw.LineAA(target, a, b, width, ink)
		draw.LineAA(reference, a, b, width, ink)
		if !bytes.Equal(direct.Pix, ref.Pix) {
			t.Fatalf("stroke %d from %v to %v, width %d, ink %v differs from generic surface", i, a, b, width, ink)
		}
	}
}

func TestLineAANothingVisible(t *testing.T) {
	rect := image.Rect(0, 0, 4, 4)
	white := color.RGBA{255, 255, 255, 255}
	for _, tc := range []struct {
		name   string
		bounds image.Rectangle
		a, b   image.Point
		ink    color.RGBA
	}{
		{"empty", image.Rectangle{}, image.Pt(0, 0), image.Pt(1, 1), white},
		{"transparent", rect, image.Pt(0, 0), image.Pt(3, 3), color.RGBA{}},
		{"point outside", rect, image.Pt(-1, 0), image.Pt(-1, 0), white},
		{"major axis outside", rect, image.Pt(4, 0), image.Pt(9, 1), white},
		{"minor axis outside", rect, image.Pt(-2, 6), image.Pt(9, 7), white},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := image.NewRGBA(rect)
			for i := range parent.Pix {
				parent.Pix[i] = 0x77
			}
			want := bytes.Clone(parent.Pix)
			draw.LineAA(parent.SubImage(tc.bounds).(*image.RGBA), tc.a, tc.b, 1, tc.ink)
			if !bytes.Equal(parent.Pix, want) {
				t.Fatal("invisible stroke altered the destination")
			}
		})
	}
}

func TestLineAALargeCoordinatesKeepPixelCoverage(t *testing.T) {
	// A small rectangle near the integer limit still has distinct pixel centers.
	y := math.MaxInt - 4
	dst := image.NewRGBA(image.Rect(-1, y, 2, y+3))
	ink := color.RGBA{R: 96, A: 128}
	draw.LineAA(dst, image.Pt(math.MinInt, y+1), image.Pt(math.MaxInt, y+1), -1, ink)
	for row := 0; row < 3; row++ {
		for x := -1; x <= 1; x++ {
			want := color.RGBA{}
			if row == 1 {
				want = ink
			}
			if got := dst.RGBAAt(x, y+row); got != want {
				t.Fatalf("(%d, row %d) = %v, want %v", x, row, got, want)
			}
		}
	}
	// Recenter the same sloped, clipped stroke for the native rectangle path.
	near := image.NewRGBA(image.Rect(-1, 0, 2, 3))
	copy(near.Pix, dst.Pix)
	draw.LineAA(near, image.Pt(-2, 0), image.Pt(2, 2), 2, ink)
	draw.LineAA(dst, image.Pt(-2, y), image.Pt(2, y+2), 2, ink)
	if !bytes.Equal(near.Pix, dst.Pix) {
		t.Fatal("translation changed stroke coverage")
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
