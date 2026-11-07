package draw

import (
	"encoding/binary"
	"image"
	"image/draw"
	"reflect"

	"github.com/BeatGlow/display/pixel"
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
	if fastDraw(dst, r, src, sp, mask, op) {
		return
	}
	draw.DrawMask(dst, r, src, sp, mask, mp, op)
}

// fastDraw handles same-format, byte-addressable Src copies. Other formats,
// masks, and Porter-Duff composition retain the standard library semantics.
func fastDraw(dst Image, r image.Rectangle, src image.Image, sp image.Point, mask image.Image, op Op) bool {
	if op != Src || mask != nil {
		return false
	}
	dv, dok := dst.(pixel.FastImage)
	sv, sok := src.(pixel.FastImage)
	if !dok || !sok {
		return false
	}
	d, s := dv.PixelView(), sv.PixelView()
	if d.Format != s.Format || d.Format < pixel.FormatCBGR15 || d.Format > pixel.FormatCRGB16 || !sameOrder(d.Order, s.Order) {
		return false
	}
	origin := r.Min
	r = r.Intersect(dst.Bounds()).Intersect(src.Bounds().Sub(sp).Add(origin))
	if r.Empty() {
		return true
	}
	sp = sp.Add(r.Min.Sub(origin))
	w := r.Dx() * 2
	rowOffset := func(v pixel.PixelView, x, y int) int { return (y-v.Bounds.Min.Y)*v.Stride + (x-v.Bounds.Min.X)*2 }
	start, end, step := r.Min.Y, r.Max.Y, 1
	sameImage := reflect.TypeOf(dst).Comparable() && dst == src
	if sameImage && r.Min.Y > sp.Y && r.Min.Y < sp.Y+r.Dy() {
		start, end, step = r.Max.Y-1, r.Min.Y-1, -1
	}
	for y := start; y != end; y += step {
		di, si := rowOffset(d, r.Min.X, y), rowOffset(s, sp.X, sp.Y+y-r.Min.Y)
		copy(d.Pix[di:di+w], s.Pix[si:si+w])
	}
	return true
}

func sameOrder(a, b binary.ByteOrder) bool { return a == b }
