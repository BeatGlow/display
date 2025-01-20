package draw

import (
	"image"
	"image/color"
)

// Line draws a line between two points.
func Line(dst Image, a, b image.Point, c color.Color) {
	bresenham(dst, a.X, a.Y, b.X, b.Y, c)
}

// HorizontalLine draws a line between (x,y) and (x+w,y).
func HorizontalLine(dst Image, x, y, w int, c color.Color) {
	bresenham(dst, x, y, x+w-1, y, c)
}

// VerticalLine draws a line between (x,y) and (x,y+h).
func VerticalLine(dst Image, x, y, h int, c color.Color) {
	bresenham(dst, x, y, x, y+h-1, c)
}

// Rectangle draws a rectangle.
func Rectangle(dst Image, rect image.Rectangle, c color.Color) {
	for x := rect.Min.X; x < rect.Max.X; x++ {
		dst.Set(x, rect.Min.X, c)
		dst.Set(x, rect.Max.X, c)
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		dst.Set(rect.Min.X, y, c)
		dst.Set(rect.Max.X, y, c)
	}
}

// RoundedRectangle draws a rectangle with radius pixels rounded corners.
func RoundedRectangle(dst Image, rect image.Rectangle, radius int, c color.Color) {
	var (
		r = radius
		x = rect.Min.X
		y = rect.Min.Y
		w = rect.Dx()
		h = rect.Dy()
	)
	HorizontalLine(dst, x, y, w, c)
	HorizontalLine(dst, x, y+h-1, w, c)
	VerticalLine(dst, x, y, h, c)
	VerticalLine(dst, x+w-1, y, h, c)
	roundedCorner(dst, x+0+r+0, y+0+r+0, r, 1, c)
	roundedCorner(dst, x+w-r-1, y+0+r+0, r, 2, c)
	roundedCorner(dst, x+w-r-1, y+h-r-1, r, 4, c)
	roundedCorner(dst, x+0+r+0, y+h-r-1, r, 8, c)
}

// Box draws a filled rectangle.
func Box(dst Image, rect image.Rectangle, c color.Color) {
	var (
		x = rect.Min.X
		h = rect.Dy()
	)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		VerticalLine(dst, x, y, h, c)
	}
}

// RoundedBox draws a filled rectangle with radius pixels rounded corners.
func RoundedBox(dst Image, rect image.Rectangle, radius int, c color.Color) {
	var (
		r = radius
		x = rect.Min.X
		y = rect.Min.Y
		w = rect.Dx()
		h = rect.Dy()
	)
	Box(dst, image.Rectangle{
		Min: image.Point{X: x + r, Y: y},
		Max: image.Point{X: x + r + w - 2*r, Y: y + h - 1},
	}, c)
	filledRoundedCorner(dst, x+w-r-1, y+r, r, 1, h-2*r-1, c)
	filledRoundedCorner(dst, x+r, y+r, r, 2, h-2*r-1, c)
}

func roundedCorner(dst Image, x0, y0, radius, quadrant int, c color.Color) {
	var (
		f    = 1 - radius
		ddFx = 1
		ddFy = -2 * radius
		x    = 0
		y    = radius
	)
	for x < y {
		if f >= 0 {
			y--
			ddFy += 2
			f += ddFy
		}

		x++
		ddFx += 2
		f += ddFx

		if quadrant&4 != 0 {
			dst.Set(x0+x, y0+y, c)
			dst.Set(x0+y, y0+x, c)
		}
		if quadrant&2 != 0 {
			dst.Set(x0+x, y0-y, c)
			dst.Set(x0+y, y0-x, c)
		}
		if quadrant&8 != 0 {
			dst.Set(x0-y, y0+x, c)
			dst.Set(x0-x, y0+y, c)
		}
		if quadrant&1 != 0 {
			dst.Set(x0-y, y0-x, c)
			dst.Set(x0-x, y0-y, c)
		}
	}
}

func filledRoundedCorner(dst Image, x0, y0, radius, quadrant, delta int, c color.Color) {
	var (
		f    = 1 - radius
		ddFx = 1
		ddFy = -2 * radius
		x    = 0
		y    = radius
	)
	for x < y {
		if f >= 0 {
			y--
			ddFy += 2
			f += ddFy
		}

		x++
		ddFx += 2
		f += ddFx

		if quadrant&1 != 0 {
			VerticalLine(dst, x0+x, y0-y, 2*y+1+delta, c)
			VerticalLine(dst, x0+y, y0-x, 2*x+1+delta, c)
		}

		if quadrant&2 != 0 {
			VerticalLine(dst, x0-x, y0-y, 2*y+1+delta, c)
			VerticalLine(dst, x0-y, y0-x, 2*x+1+delta, c)
		}
	}
}

// Generalized with integer
func bresenham(dst Image, x1, y1, x2, y2 int, c color.Color) {
	var dx, dy, e, slope int

	// Because drawing p1 -> p2 is equivalent to draw p2 -> p1,
	// I sort points in x-axis order to handle only half of possible cases.
	if x1 > x2 {
		x1, y1, x2, y2 = x2, y2, x1, y1
	}

	dx, dy = x2-x1, y2-y1
	// Because point is x-axis ordered, dx cannot be negative
	if dy < 0 {
		dy = -dy
	}

	switch {

	// Is line a point ?
	case x1 == x2 && y1 == y2:
		dst.Set(x1, y1, c)

	// Is line an horizontal ?
	case y1 == y2:
		for ; dx != 0; dx-- {
			dst.Set(x1, y1, c)
			x1++
		}
		dst.Set(x1, y1, c)

	// Is line a vertical ?
	case x1 == x2:
		if y1 > y2 {
			y1, y2 = y2, y1
		}
		for ; dy != 0; dy-- {
			dst.Set(x1, y1, c)
			y1++
		}
		dst.Set(x1, y1, c)

	// Is line a diagonal ?
	case dx == dy:
		if y1 < y2 {
			for ; dx != 0; dx-- {
				dst.Set(x1, y1, c)
				x1++
				y1++
			}
		} else {
			for ; dx != 0; dx-- {
				dst.Set(x1, y1, c)
				x1++
				y1--
			}
		}
		dst.Set(x1, y1, c)

	// wider than high ?
	case dx > dy:
		if y1 < y2 {
			// BresenhamDxXRYD(img, x1, y1, x2, y2, col)
			dy, e, slope = 2*dy, dx, 2*dx
			for ; dx != 0; dx-- {
				dst.Set(x1, y1, c)
				x1++
				e -= dy
				if e < 0 {
					y1++
					e += slope
				}
			}
		} else {
			// BresenhamDxXRYU(img, x1, y1, x2, y2, col)
			dy, e, slope = 2*dy, dx, 2*dx
			for ; dx != 0; dx-- {
				dst.Set(x1, y1, c)
				x1++
				e -= dy
				if e < 0 {
					y1--
					e += slope
				}
			}
		}
		dst.Set(x2, y2, c)

	// higher than wide.
	default:
		if y1 < y2 {
			// BresenhamDyXRYD(img, x1, y1, x2, y2, col)
			dx, e, slope = 2*dx, dy, 2*dy
			for ; dy != 0; dy-- {
				dst.Set(x1, y1, c)
				y1++
				e -= dx
				if e < 0 {
					x1++
					e += slope
				}
			}
		} else {
			// BresenhamDyXRYU(img, x1, y1, x2, y2, col)
			dx, e, slope = 2*dx, dy, 2*dy
			for ; dy != 0; dy-- {
				dst.Set(x1, y1, c)
				y1--
				e -= dx
				if e < 0 {
					x1++
					e += slope
				}
			}
		}
		dst.Set(x2, y2, c)
	}
}
