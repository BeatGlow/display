// Package rotation contains coordinate transforms for framebuffer images.
package rotation

import (
	"image"

	"github.com/BeatGlow/display"
)

// Bounds returns the logical image bounds after applying r.
func Bounds(bounds image.Rectangle, r display.Rotation) image.Rectangle {
	r %= 4
	if r == display.Rotate90 || r == display.Rotate270 {
		return image.Rect(0, 0, bounds.Dy(), bounds.Dx())
	}
	return bounds
}

// Point maps a point in the rotated image to the native image coordinates.
func Point(bounds image.Rectangle, r display.Rotation, x, y int) (int, int) {
	r %= 4
	switch r {
	case display.Rotate90:
		return bounds.Min.X + y, bounds.Max.Y - 1 - x
	case display.Rotate180:
		return bounds.Max.X - 1 - x, bounds.Max.Y - 1 - y
	case display.Rotate270:
		return bounds.Max.X - 1 - y, bounds.Min.Y + x
	default:
		return bounds.Min.X + x, bounds.Min.Y + y
	}
}

// InversePoint maps a point in the native image to the logical image coordinates.
func InversePoint(bounds image.Rectangle, r display.Rotation, x, y int) (int, int) {
	switch r % 4 {
	case display.Rotate90:
		return bounds.Max.Y - 1 - y, x - bounds.Min.X
	case display.Rotate180:
		return bounds.Max.X - 1 - x, bounds.Max.Y - 1 - y
	case display.Rotate270:
		return y - bounds.Min.Y, bounds.Max.X - 1 - x
	default:
		return x - bounds.Min.X, y - bounds.Min.Y
	}
}
