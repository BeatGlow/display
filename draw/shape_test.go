package draw

import (
	"bytes"
	"fmt"
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

func TestLineUsesExpectedPixelsForShallowAndSteepSlopes(t *testing.T) {
	tests := []struct {
		name string
		from image.Point
		to   image.Point
		want []image.Point
	}{
		{
			name: "shallow tie steps",
			from: image.Pt(2, 2),
			to:   image.Pt(8, 5),
			want: []image.Point{
				image.Pt(2, 2), image.Pt(3, 2), image.Pt(4, 3),
				image.Pt(5, 3), image.Pt(6, 4), image.Pt(7, 4), image.Pt(8, 5),
			},
		},
		{
			name: "steep tie steps",
			from: image.Pt(2, 2),
			to:   image.Pt(5, 8),
			want: []image.Point{
				image.Pt(2, 2), image.Pt(2, 3), image.Pt(3, 4),
				image.Pt(3, 5), image.Pt(4, 6), image.Pt(4, 7), image.Pt(5, 8),
			},
		},
		{
			name: "shallow upward tie steps",
			from: image.Pt(2, 8),
			to:   image.Pt(8, 5),
			want: []image.Point{
				image.Pt(2, 8), image.Pt(3, 8), image.Pt(4, 7),
				image.Pt(5, 7), image.Pt(6, 6), image.Pt(7, 6), image.Pt(8, 5),
			},
		},
		{
			name: "steep upward tie steps",
			from: image.Pt(2, 8),
			to:   image.Pt(5, 2),
			want: []image.Point{
				image.Pt(2, 8), image.Pt(2, 7), image.Pt(3, 6),
				image.Pt(3, 5), image.Pt(4, 4), image.Pt(4, 3), image.Pt(5, 2),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			img := image.NewRGBA(image.Rect(0, 0, 12, 12))
			Line(img, tc.from, tc.to, color.White)
			want := make(map[image.Point]bool, len(tc.want))
			for _, p := range tc.want {
				want[p] = true
			}
			for y := 0; y < 12; y++ {
				for x := 0; x < 12; x++ {
					p := image.Pt(x, y)
					if got := img.RGBAAt(x, y).A != 0; got != want[p] {
						t.Errorf("pixel %v drawn=%v, want %v", p, got, want[p])
					}
				}
			}
		})
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

func TestRoundedRectangleDrawsExpectedCornerPixels(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	RoundedRectangle(img, image.Rect(3, 3, 13, 13), 3, color.White)
	want := map[image.Point]bool{}
	for x := 5; x <= 10; x++ {
		want[image.Pt(x, 3)] = true
		want[image.Pt(x, 12)] = true
	}
	for y := 5; y <= 10; y++ {
		want[image.Pt(3, y)] = true
		want[image.Pt(12, y)] = true
	}
	for _, p := range []image.Point{
		image.Pt(4, 4), image.Pt(11, 4), image.Pt(4, 11), image.Pt(11, 11),
	} {
		want[p] = true
	}
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			p := image.Pt(x, y)
			if got := img.RGBAAt(x, y).A != 0; got != want[p] {
				t.Errorf("pixel %v drawn=%v, want %v", p, got, want[p])
			}
		}
	}
}

func TestRoundedBoxDrawsExpectedRows(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	RoundedBox(img, image.Rect(3, 3, 13, 13), 3, color.White)
	rowBounds := map[int][2]int{
		3:  {5, 10},
		4:  {4, 11},
		5:  {3, 12},
		6:  {3, 12},
		7:  {3, 12},
		8:  {3, 12},
		9:  {3, 12},
		10: {3, 12},
		11: {4, 11},
		12: {5, 10},
	}
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			bounds, inRow := rowBounds[y]
			want := inRow && x >= bounds[0] && x <= bounds[1]
			if got := img.RGBAAt(x, y).A != 0; got != want {
				t.Errorf("pixel (%d,%d) drawn=%v, want %v", x, y, got, want)
			}
		}
	}
}

func TestRoundedShapesStayInBoundsAtRadiusBoundaries(t *testing.T) {
	for _, size := range []image.Point{
		image.Pt(1, 5), image.Pt(2, 1), image.Pt(3, 6), image.Pt(6, 3), image.Pt(5, 5),
	} {
		for _, radius := range []int{0, 1, 2, 4} {
			t.Run(fmt.Sprintf("%dx%d/radius-%d", size.X, size.Y, radius), func(t *testing.T) {
				rect := image.Rectangle{Min: image.Pt(4, 5), Max: image.Pt(4+size.X, 5+size.Y)}
				for _, tc := range []struct {
					name string
					draw func(Image, image.Rectangle, int, color.Color)
				}{
					{"outline", RoundedRectangle},
					{"filled", RoundedBox},
				} {
					t.Run(tc.name, func(t *testing.T) {
						img := image.NewRGBA(image.Rect(0, 0, 16, 16))
						tc.draw(img, rect, radius, color.White)
						if countNonzero(img) == 0 {
							t.Fatal("shape drew no pixels")
						}
						for y := 0; y < 16; y++ {
							for x := 0; x < 16; x++ {
								if img.RGBAAt(x, y).A != 0 && !image.Pt(x, y).In(rect) {
									t.Errorf("radius %d drew outside %v at (%d,%d)", radius, rect, x, y)
								}
							}
						}
					})
				}
			})
		}
	}
}

func TestZeroRadiusMatchesSquareShapes(t *testing.T) {
	for _, rect := range []image.Rectangle{
		image.Rect(2, 3, 3, 8), image.Rect(2, 3, 7, 4), image.Rect(2, 3, 8, 9),
	} {
		t.Run(rect.String(), func(t *testing.T) {
			roundedOutline, outline := image.NewRGBA(image.Rect(0, 0, 12, 12)), image.NewRGBA(image.Rect(0, 0, 12, 12))
			RoundedRectangle(roundedOutline, rect, 0, color.White)
			Rectangle(outline, rect, color.White)
			if !bytes.Equal(roundedOutline.Pix, outline.Pix) {
				t.Fatal("zero-radius rounded rectangle differs from Rectangle")
			}

			roundedBox, box := image.NewRGBA(image.Rect(0, 0, 12, 12)), image.NewRGBA(image.Rect(0, 0, 12, 12))
			RoundedBox(roundedBox, rect, 0, color.White)
			Box(box, rect, color.White)
			if !bytes.Equal(roundedBox.Pix, box.Pix) {
				t.Fatal("zero-radius rounded box differs from Box")
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
