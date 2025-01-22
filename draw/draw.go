package draw

import (
	"image"
	"image/draw"
)

// Drawer is an alias for [image/draw.Drawer].
type Drawer = draw.Drawer

// Image is an alias for [image/draw.Image].
type Image = draw.Image

// Op is an alias for image/draw.Op
type Op = draw.Op

const (
	// Over specifies ``(src in mask) over dst''.
	Over Op = iota

	// Src specifies ``src in mask''.
	Src
)

// Draw calls draw.Draw
// Draw calls [DrawMask] with a nil mask.
func Draw(dst Image, r image.Rectangle, src image.Image, sp image.Point, op Op) {
	DrawMask(dst, r, src, sp, nil, image.Point{}, op)
}

// DrawMask aligns r.Min in dst with sp in src and mp in mask and then replaces the rectangle r
// in dst with the result of a Porter-Duff composition. A nil mask is treated as opaque.
func DrawMask(dst Image, r image.Rectangle, src image.Image, sp image.Point, mask image.Image, mp image.Point, op Op) {
	draw.DrawMask(dst, r, src, sp, mask, mp, op)
}
