package jpegn

import (
	"image"
)

// rgbToYCbCr converts one pixel to the JPEG YCbCr color space.
func rgbToYCbCr(r, g, b byte) (byte, byte, byte) {
	r1, g1, b1 := int32(r), int32(g), int32(b)

	yy := (19595*r1 + 38470*g1 + 7471*b1 + 1<<15) >> 16
	cb := (-11056*r1 - 21712*g1 + 32768*b1 + 257<<15) >> 16
	cr := (32768*r1 - 27440*g1 - 5328*b1 + 257<<15) >> 16

	if cb < 0 {
		cb = 0
	} else if cb > 255 {
		cb = 255
	}

	if cr < 0 {
		cr = 0
	} else if cr > 255 {
		cr = 255
	}

	return byte(yy), byte(cb), byte(cr)
}

// readRow returns a row in display orientation with a four-byte pixel stride.
func readRow(m image.Image, b image.Rectangle, y int, dst []byte, rotate bool) []byte {
	w := b.Dx()
	x, step := 0, 1
	if rotate {
		y = b.Dy() - 1 - y
		x, step = w-1, -1
	}

	switch src := m.(type) {
	case *image.RGBA:
		p := src.Pix[src.PixOffset(b.Min.X, b.Min.Y+y):]
		if !rotate {
			return p
		}
		for o := 0; o < w*4; o += 4 {
			copy(dst[o:o+4], p[x*4:])
			x--
		}
	case *image.NRGBA:
		p := src.Pix[src.PixOffset(b.Min.X, b.Min.Y+y):]
		scale, shift := uint32(1), uint(0)
		if rotate {
			// Match the RGBA() conversion used for rotated image views.
			scale, shift = 257, 8
		}
		for o := 0; o < w*4; o += 4 {
			in := p[x*4:][:4]
			a := uint32(in[3]) * scale
			dst[o] = byte((uint32(in[0]) * a / 255) >> shift)
			dst[o+1] = byte((uint32(in[1]) * a / 255) >> shift)
			dst[o+2] = byte((uint32(in[2]) * a / 255) >> shift)
			x += step
		}
	default:
		for o := 0; o < w*4; o += 4 {
			r, g, bl, _ := m.At(b.Min.X+x, b.Min.Y+y).RGBA()
			dst[o] = byte(r >> 8)
			dst[o+1] = byte(g >> 8)
			dst[o+2] = byte(bl >> 8)
			x += step
		}
	}

	return dst
}

// fillRow copies src into dst[:w], replicating the last sample if src is short.
func fillRow(dst, src []byte, w int) {
	n := copy(dst[:w], src)
	if n == 0 {
		return
	}

	for i := n; i < w; i++ {
		dst[i] = dst[n-1]
	}
}

// padPlane replicates the right column and bottom row across the MCU padding.
func padPlane(p []byte, stride, w, h, ph int) {
	for y := 0; y < h; y++ {
		row := p[y*stride : (y+1)*stride]
		v := row[w-1]

		for x := w; x < stride; x++ {
			row[x] = v
		}
	}

	last := p[(h-1)*stride : h*stride]
	for y := h; y < ph; y++ {
		copy(p[y*stride:(y+1)*stride], last)
	}
}

// buildPlanes allocates the padded component planes and fills them from m.
func (e *encoder) buildPlanes(m image.Image) {
	for i := 0; i < e.ncomp; i++ {
		c := &e.comp[i]
		c.stride = e.mcusX * c.ssX * 8
		c.width = (e.width*c.ssX + e.hmax - 1) / e.hmax
		c.height = (e.height*c.ssY + e.vmax - 1) / e.vmax

		need := c.stride * e.mcusY * c.ssY * 8
		if cap(c.plane) < need {
			c.plane = make([]byte, need)
		} else {
			c.plane = c.plane[:need]
		}
	}

	switch {
	case e.ncomp == 1:
		e.fillGray(m.(*image.Gray))
	case !e.rotate180 && e.matchesYCbCr(m):
		e.fillFromYCbCr(m.(*image.YCbCr))
	default:
		e.fillColor(m)
	}

	for i := 0; i < e.ncomp; i++ {
		c := &e.comp[i]
		padPlane(c.plane, c.stride, c.width, c.height, e.mcusY*c.ssY*8)
	}
}

// matchesYCbCr reports whether m has planes usable without resampling.
func (e *encoder) matchesYCbCr(m image.Image) bool {
	src, ok := m.(*image.YCbCr)
	return ok && src.SubsampleRatio == image.YCbCrSubsampleRatio420
}

// fillFromYCbCr copies the source planes directly into the component planes.
func (e *encoder) fillFromYCbCr(src *image.YCbCr) {
	b := src.Bounds()

	y := &e.comp[0]
	for py := 0; py < y.height; py++ {
		off := src.YOffset(b.Min.X, b.Min.Y+py)
		fillRow(y.plane[py*y.stride:], src.Y[off:], y.width)
	}

	cb := &e.comp[1]
	cr := &e.comp[2]

	for py := 0; py < cb.height; py++ {
		off := src.COffset(b.Min.X, b.Min.Y+py*e.vmax)
		fillRow(cb.plane[py*cb.stride:], src.Cb[off:], cb.width)
		fillRow(cr.plane[py*cr.stride:], src.Cr[off:], cr.width)
	}
}

// fillGray fills the single luma plane of a grayscale image.
func (e *encoder) fillGray(src *image.Gray) {
	b := src.Bounds()
	c := &e.comp[0]

	for y := 0; y < c.height; y++ {
		row := c.plane[y*c.stride : y*c.stride+c.width]
		if e.rotate180 {
			p := src.Pix[src.PixOffset(b.Min.X, b.Max.Y-1-y):][:c.width]
			for x, v := range p {
				row[c.width-1-x] = v
			}
		} else {
			copy(row, src.Pix[src.PixOffset(b.Min.X, b.Min.Y+y):])
		}
	}
}

// fillColor converts m into 4:2:0 component planes, box-filtering chroma.
func (e *encoder) fillColor(m image.Image) {
	b := m.Bounds()
	w, h := e.width, e.height
	row := e.scratch(w * 4)

	y := &e.comp[0]
	cb := &e.comp[1]
	cr := &e.comp[2]
	span := w * 2
	if cap(e.chromaBuf) < span*2 {
		e.chromaBuf = make([]byte, span*2)
	}
	buf := e.chromaBuf[:span*2]

	for cy := 0; cy < cb.height; cy++ {
		rows := min(2, h-2*cy)
		for k := range rows {
			py := cy*2 + k
			p := readRow(m, b, py, row, e.rotate180)
			rgbToYCbCrRow(y.plane[py*y.stride:], buf[k*span:], buf[k*span+w:], p, w)
		}
		downsampleChroma(cb.plane[cy*cb.stride:], cr.plane[cy*cr.stride:], buf, w, rows)
	}
}

// downsampleChroma box-filters 2x2 sample groups. Odd edges use the pixels present.
func downsampleChroma(dstCb, dstCr, buf []byte, w, rows int) {
	span, full := w*2, w/2
	cb0, cr0 := buf[:w], buf[w:span]
	if rows == 2 {
		cb1, cr1 := buf[span:span+w], buf[span+w:span*2]
		downsampleRow2x2(dstCb, cb0, cb1, full)
		downsampleRow2x2(dstCr, cr0, cr1, full)
		if w%2 != 0 {
			dstCb[full] = average(cb0[w-1], cb1[w-1])
			dstCr[full] = average(cr0[w-1], cr1[w-1])
		}
		return
	}

	for x := range full {
		dstCb[x] = average(cb0[x*2], cb0[x*2+1])
		dstCr[x] = average(cr0[x*2], cr0[x*2+1])
	}
	if w%2 != 0 {
		dstCb[full], dstCr[full] = cb0[w-1], cr0[w-1]
	}
}

func average(a, b byte) byte {
	return byte((uint16(a) + uint16(b) + 1) >> 1)
}

// scratch returns a reusable row buffer of at least n bytes.
func (e *encoder) scratch(n int) []byte {
	if cap(e.rowBuf) < n {
		e.rowBuf = make([]byte, n)
	}

	return e.rowBuf[:n]
}
