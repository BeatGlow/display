package draw

import (
	"image"
	"image/color"
	"math"
)

// LineAA draws an anti-aliased line between pixel centers a and b, including
// both endpoints. It composites premultiplied ink over dst. Width is measured
// across the minor axis in pixels, with a minimum of one. A zero-length line
// covers its single pixel fully.
func LineAA(dst Image, a, b image.Point, width int, ink color.RGBA) {
	bounds := dst.Bounds()
	if bounds.Empty() || ink.A == 0 {
		return
	}
	rgba, _ := dst.(*image.RGBA)
	if a == b {
		if a.In(bounds) {
			blendStrokePixel(dst, rgba, a.X, a.Y, ink)
		}
		return
	}

	// Walk the longer axis once. On the other axis, coverage drops linearly
	// from one to zero over the outermost pixel on each side of the stroke.
	dx, dy := strokeDelta(a.X, b.X), strokeDelta(a.Y, b.Y)
	steep := math.Abs(dy) > math.Abs(dx)
	if steep {
		a.X, a.Y = a.Y, a.X
		b.X, b.Y = b.Y, b.X
		dx, dy = dy, dx
		bounds = image.Rect(bounds.Min.Y, bounds.Min.X, bounds.Max.Y, bounds.Max.X)
	}
	first := max(bounds.Min.X, min(a.X, b.X))
	last := min(bounds.Max.X-1, max(a.X, b.X))
	if first > last {
		return
	}
	reach := (float64(max(1, width)) + 1) / 2
	// Keep the original pixel-center arithmetic for ordinary images. For
	// origins beyond the half-pixel precision of float64, translate first.
	origin := 0
	if float64(bounds.Min.Y) <= -(1<<52) || float64(bounds.Max.Y) >= 1<<52 {
		origin = bounds.Min.Y
	}
	minorStart := strokeDelta(origin, a.Y)
	minorMin, minorMax := strokeDelta(origin, bounds.Min.Y), strokeDelta(origin, bounds.Max.Y)
	// The native kernel walks a clipped RGBA rectangle by byte stride.
	// Keep extreme major-axis distances on the unsigned-delta path below.
	if rgba != nil && math.Abs(dx) < min(1<<52, float64(math.MaxInt)) && validStrokeRGBA(rgba) {
		x, y := first, bounds.Min.Y
		majorStep, minorStep := 4, rgba.Stride
		if steep {
			x, y = y, x
			majorStep, minorStep = minorStep, majorStep
		}
		probeStroke(rgba.Pix[rgba.PixOffset(x, y):], majorStep, minorStep, last-first+1, first-a.X,
			minorStart, dx, dy, minorMin, minorMax, reach, ink)
		return
	}
	for major := first; major <= last; major++ {
		fraction := strokeDelta(a.X, major) / dx
		center := minorStart + dy*fraction
		low := max(minorMin, math.Floor(center-reach)+1)
		high := min(minorMax, math.Ceil(center+reach))
		if low >= high {
			continue
		}
		for minor := int(low); minor < int(high); minor++ {
			coverage := reach - math.Abs(float64(minor)-center)
			source := strokeCoverage(ink, coverage)
			x, y := major, origin+minor
			if steep {
				x, y = y, x
			}
			blendStrokePixel(dst, rgba, x, y, source)
		}
	}
}

// Subtract before conversion so nearby coordinates retain their distance even
// far from the origin. Unsigned subtraction also spans the full int range.
func strokeDelta(start, end int) float64 {
	if end < start {
		return -float64(uint(start) - uint(end))
	}
	return float64(uint(end) - uint(start))
}

// Validate the scanlines before passing their backing storage to assembly.
func validStrokeRGBA(dst *image.RGBA) bool {
	w, h := dst.Rect.Dx(), dst.Rect.Dy()
	return w > 0 && h > 0 && w <= len(dst.Pix)/4 && dst.Stride >= w*4 &&
		h-1 <= (len(dst.Pix)-w*4)/dst.Stride
}

func strokeCoverage(ink color.RGBA, coverage float64) color.RGBA {
	source := ink
	if coverage < 1 {
		source = color.RGBA{
			R: uint8(float64(ink.R) * coverage),
			G: uint8(float64(ink.G) * coverage),
			B: uint8(float64(ink.B) * coverage),
			A: uint8(float64(ink.A) * coverage),
		}
	}
	return source
}

func blendStrokePixel(dst Image, rgba *image.RGBA, x, y int, src color.RGBA) {
	if src.A == 0 {
		return
	}
	if rgba != nil {
		i := rgba.PixOffset(x, y)
		blendStrokeRGBA(rgba.Pix[i:i+4], src)
		return
	}

	if src.A == 255 {
		dst.Set(x, y, src)
		return
	}
	r, g, b, a := dst.At(x, y).RGBA()
	inverse := uint32(255-src.A) * 257
	over := func(d uint32, s byte) uint16 {
		return uint16(min(uint32(65535), uint32(s)*257+(d*inverse+32767)/65535))
	}
	dst.Set(x, y, color.RGBA64{R: over(r, src.R), G: over(g, src.G), B: over(b, src.B), A: over(a, src.A)})
}

func blendStrokeRGBA(p []byte, src color.RGBA) {
	_ = p[3]
	if src.A == 255 {
		p[0], p[1], p[2], p[3] = src.R, src.G, src.B, src.A
		return
	}
	inverse := uint32(255 - src.A)
	// Compute rounded 16-bit source-over and take its high byte.
	// 257 expands an 8-bit channel to 16 bits, and 65280 = 255 * 256.
	over := func(d, s byte) byte {
		return byte(min(uint32(255), (257*(uint32(s)*255+uint32(d)*inverse)+127)/65280))
	}
	p[0], p[1], p[2], p[3] = over(p[0], src.R), over(p[1], src.G), over(p[2], src.B), over(p[3], src.A)
}
