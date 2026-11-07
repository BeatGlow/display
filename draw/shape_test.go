package draw

import (
	"image"
	"image/color"
	"testing"
)

func TestLineOctantsAndAxisHelpers(t *testing.T) {
	for _, tc := range []struct {
		name string
		a, b image.Point
	}{
		{"point", image.Pt(4, 4), image.Pt(4, 4)},
		{"horizontal-reversed", image.Pt(8, 3), image.Pt(3, 3)},
		{"vertical-reversed", image.Pt(5, 9), image.Pt(5, 4)},
		{"diagonal-down", image.Pt(2, 2), image.Pt(7, 7)},
		{"diagonal-up", image.Pt(2, 8), image.Pt(7, 3)},
		{"shallow-down", image.Pt(2, 2), image.Pt(8, 5)},
		{"shallow-up", image.Pt(2, 8), image.Pt(8, 5)},
		{"steep-down", image.Pt(2, 2), image.Pt(5, 8)},
		{"steep-up", image.Pt(2, 8), image.Pt(5, 2)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			img := image.NewRGBA(image.Rect(0, 0, 12, 12))
			Line(img, tc.a, tc.b, color.White)
			want := max(abs(tc.b.X-tc.a.X), abs(tc.b.Y-tc.a.Y)) + 1
			if got := countNonzero(img); got != want {
				t.Fatalf("line set %d pixels, want %d", got, want)
			}
			for _, p := range []image.Point{tc.a, tc.b} {
				if img.RGBAAt(p.X, p.Y) != (color.RGBA{A: 255, R: 255, G: 255, B: 255}) {
					t.Errorf("line endpoint %v was not drawn", p)
				}
			}
		})
	}

	img := image.NewRGBA(image.Rect(0, 0, 12, 12))
	HorizontalLine(img, 2, 3, 5, color.White)
	VerticalLine(img, 8, 2, 4, color.White)
	if got := countNonzero(img); got != 9 {
		t.Fatalf("axis helpers drew %d pixels, want 9", got)
	}
}

func TestRectangleUsesHalfOpenBounds(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 12, 12))
	rect := image.Rect(2, 3, 8, 9)
	Rectangle(img, rect, color.White)
	for y := 0; y < 12; y++ {
		for x := 0; x < 12; x++ {
			want := x >= rect.Min.X && x < rect.Max.X && y >= rect.Min.Y && y < rect.Max.Y &&
				(x == rect.Min.X || x == rect.Max.X-1 || y == rect.Min.Y || y == rect.Max.Y-1)
			got := img.RGBAAt(x, y).A != 0
			if got != want {
				t.Errorf("pixel (%d,%d) drawn=%v, want %v", x, y, got, want)
			}
		}
	}
}

func TestRectangleDegenerateBounds(t *testing.T) {
	for _, tc := range []struct {
		name string
		rect image.Rectangle
		want int
	}{
		{"empty", image.Rect(4, 4, 4, 7), 0},
		{"point", image.Rect(4, 4, 5, 5), 1},
		{"vertical", image.Rect(4, 3, 5, 8), 5},
		{"horizontal", image.Rect(3, 4, 8, 5), 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			img := image.NewRGBA(image.Rect(0, 0, 12, 12))
			Rectangle(img, tc.rect, color.White)
			if got := countNonzero(img); got != tc.want {
				t.Fatalf("rectangle drew %d pixels, want %d", got, tc.want)
			}
		})
	}
}

func TestBoxAndRoundedShapesStayWithinBounds(t *testing.T) {
	rect := image.Rect(3, 3, 13, 13)
	for _, tc := range []struct {
		name string
		draw func(Image, image.Rectangle, color.Color)
	}{
		{"box", Box},
		{"rounded-box", func(dst Image, r image.Rectangle, c color.Color) { RoundedBox(dst, r, 3, c) }},
		{"rounded-rectangle", func(dst Image, r image.Rectangle, c color.Color) { RoundedRectangle(dst, r, 3, c) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			img := image.NewRGBA(image.Rect(0, 0, 16, 16))
			tc.draw(img, rect, color.White)
			if got := countNonzero(img); got == 0 {
				t.Fatal("shape drew no pixels")
			}
			for y := 0; y < 16; y++ {
				for x := 0; x < 16; x++ {
					if img.RGBAAt(x, y).A != 0 && !image.Pt(x, y).In(rect) {
						t.Errorf("shape drew outside rectangle at (%d,%d)", x, y)
					}
				}
			}
		})
	}
}

func countNonzero(img *image.RGBA) int {
	n := 0
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			if img.RGBAAt(x, y).A != 0 {
				n++
			}
		}
	}
	return n
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
