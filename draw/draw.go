package draw

import (
	"encoding/binary"
	"image"
	"image/color"
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
	dv, ok := dst.(pixel.FastImage)
	if !ok {
		return false
	}
	d := dv.PixelView()
	if d.Bounds != dst.Bounds() || !supported16Bit(d) {
		return false
	}
	if uniform, ok := src.(*image.Uniform); ok && uniform.C != nil {
		return fastUniform(d, dst.Bounds(), r, src.Bounds(), sp, uniform.C)
	}
	if rgba, ok := src.(*image.RGBA); ok {
		return fastRGBA(d, dst.Bounds(), r, rgba, sp)
	}
	sv, ok := src.(pixel.FastImage)
	if !ok {
		return false
	}
	s := sv.PixelView()
	if s.Bounds != src.Bounds() || !supported16Bit(s) || d.Format != s.Format || !sameOrder(d.Order, s.Order) {
		return false
	}
	r, sp = clipDraw(dst.Bounds(), r, s.Bounds, sp)
	if r.Empty() {
		return true
	}
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

func supported16Bit(v pixel.PixelView) bool {
	if v.Format < pixel.FormatCBGR15 || v.Format > pixel.FormatCRGB16 || v.Order == nil {
		return false
	}
	w, h := v.Bounds.Dx(), v.Bounds.Dy()
	if w < 0 || h < 0 || v.Stride < w*2 {
		return false
	}
	return h == 0 || len(v.Pix) >= (h-1)*v.Stride+w*2
}

func clipDraw(dst, r, src image.Rectangle, sp image.Point) (image.Rectangle, image.Point) {
	origin := r.Min
	r = r.Intersect(dst).Intersect(src.Sub(sp).Add(origin))
	sp = sp.Add(r.Min.Sub(origin))
	return r, sp
}

func fastUniform(dst pixel.PixelView, bounds image.Rectangle, r image.Rectangle, srcBounds image.Rectangle, sp image.Point, c color.Color) bool {
	word, ok := uniformWord(dst.Format, c)
	if !ok {
		return false
	}
	r, _ = clipDraw(bounds, r, srcBounds, sp)
	if r.Empty() {
		return true
	}
	var value [2]byte
	dst.Order.PutUint16(value[:], word)
	var block [64]byte
	for i := 0; i < len(block); i += 2 {
		block[i], block[i+1] = value[0], value[1]
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		start := (y-dst.Bounds.Min.Y)*dst.Stride + (r.Min.X-dst.Bounds.Min.X)*2
		row := dst.Pix[start : start+r.Dx()*2]
		for len(row) >= len(block) {
			copy(row, block[:])
			row = row[len(block):]
		}
		if len(row) != 0 {
			copy(row, block[:len(row)])
		}
	}
	return true
}

func uniformWord(format pixel.PixelFormat, c color.Color) (uint16, bool) {
	switch format {
	case pixel.FormatCBGR15:
		return pixel.CBGR15Model.Convert(c).(pixel.CBGR15).V, true
	case pixel.FormatCBGR16:
		return pixel.CBGR16Model.Convert(c).(pixel.CBGR16).V, true
	case pixel.FormatCRGB15:
		return pixel.CRGB15Model.Convert(c).(pixel.CRGB15).V, true
	case pixel.FormatCRGB16:
		return pixel.CRGB16Model.Convert(c).(pixel.CRGB16).V, true
	default:
		return 0, false
	}
}

func fastRGBA(dst pixel.PixelView, bounds image.Rectangle, r image.Rectangle, src *image.RGBA, sp image.Point) bool {
	srcBounds := src.Bounds()
	if srcBounds != src.Bounds() || src.Stride < srcBounds.Dx()*4 || srcBounds.Dy() < 0 {
		return false
	}
	if srcBounds.Dy() > 0 && len(src.Pix) < (srcBounds.Dy()-1)*src.Stride+srcBounds.Dx()*4 {
		return false
	}
	r, sp = clipDraw(bounds, r, srcBounds, sp)
	if r.Empty() {
		return true
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		source := src.Pix[src.PixOffset(sp.X, sp.Y+y-r.Min.Y):]
		start := (y-dst.Bounds.Min.Y)*dst.Stride + (r.Min.X-dst.Bounds.Min.X)*2
		row := dst.Pix[start : start+r.Dx()*2]
		for x := 0; x < r.Dx(); x++ {
			i := x * 4
			word := rgbaWord(dst.Format, source[i], source[i+1], source[i+2])
			dst.Order.PutUint16(row[x*2:], word)
		}
	}
	return true
}

func rgbaWord(format pixel.PixelFormat, red, green, blue byte) uint16 {
	r5, g5, b5 := uint16(red>>3), uint16(green>>3), uint16(blue>>3)
	switch format {
	case pixel.FormatCBGR15:
		return b5<<10 | g5<<5 | r5
	case pixel.FormatCBGR16:
		return uint16(blue>>3)<<11 | uint16(green>>2)<<5 | r5
	case pixel.FormatCRGB15:
		return r5<<10 | g5<<5 | b5
	case pixel.FormatCRGB16:
		return r5<<11 | uint16(green>>2)<<5 | b5
	default:
		return 0
	}
}

func sameOrder(a, b binary.ByteOrder) bool { return a == b }
