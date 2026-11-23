package pixel

import (
	"image"
	"image/color"
)

var packedDither4x4 = [16]uint8{0, 8, 2, 10, 12, 4, 14, 6, 3, 11, 1, 9, 15, 7, 13, 5}

// PackedWriter writes pixels directly into a packed [FastImage]. Keep the
// image alive and do not modify its storage concurrently while using the writer.
type PackedWriter struct {
	view PixelView
}

// NewPackedWriter returns a direct writer for supported packed pixel formats.
// It returns false for invalid storage and planar YU12 images.
func NewPackedWriter(img FastImage) (PackedWriter, bool) {
	if img == nil {
		return PackedWriter{}, false
	}
	view := img.PixelView()
	width, height := view.Bounds.Dx(), view.Bounds.Dy()
	if width <= 0 || height <= 0 {
		return PackedWriter{}, false
	}

	rowBytes, rows, ok := packedViewLayout(view.Format, width, height)
	if !ok || view.Stride < rowBytes {
		return PackedWriter{}, false
	}
	if view.Format >= FormatCBGR15 && view.Format <= FormatCRGB16 && view.Order == nil {
		return PackedWriter{}, false
	}

	maxInt := int(^uint(0) >> 1)
	length := rowBytes
	if rows > 1 {
		if view.Stride > (maxInt-rowBytes)/(rows-1) {
			return PackedWriter{}, false
		}
		length += view.Stride * (rows - 1)
	}
	if len(view.Pix) < length {
		return PackedWriter{}, false
	}
	return PackedWriter{view: view}, true
}

// Bounds returns the destination image bounds.
func (w PackedWriter) Bounds() image.Rectangle {
	return w.view.Bounds
}

// SetGray writes an 8-bit luminance value. Monochrome formats use a 4×4 Bayer
// pattern to preserve apparent gray levels.
func (w PackedWriter) SetGray(x, y int, value uint8) {
	if !image.Pt(x, y).In(w.view.Bounds) {
		return
	}
	x -= w.view.Bounds.Min.X
	y -= w.view.Bounds.Min.Y

	pix := w.view.Pix
	stride := w.view.Stride
	switch w.view.Format {
	case FormatMonoLSB:
		index := y*stride + x/8
		mask := byte(1 << uint(x&7))
		pix[index] &^= mask
		if MonoDither(value, x, y) {
			pix[index] |= mask
		}
	case FormatMonoVerticalLSB:
		index := (y/8)*stride + x
		mask := byte(1 << uint(y&7))
		pix[index] &^= mask
		if MonoDither(value, x, y) {
			pix[index] |= mask
		}
	case FormatGray2:
		index, shift := y*stride+x/4, uint((3-x%4)*2)
		pix[index] = (pix[index] &^ (3 << shift)) | ((value>>6)&3)<<shift
	case FormatGray3Level:
		var level uint8
		if value >= 192 {
			level = 2
		} else if value >= 64 {
			level = 1
		}
		index, shift := y*stride+x/4, uint((3-x%4)*2)
		pix[index] = (pix[index] &^ (3 << shift)) | level<<shift
	case FormatGray4:
		index, level := y*stride+x/2, value>>4
		if x&1 == 0 {
			pix[index] = (pix[index] & 0x0f) | level<<4
		} else {
			pix[index] = (pix[index] & 0xf0) | level
		}
	case FormatCBGR15, FormatCRGB15:
		level := uint16(value >> 3)
		w.view.Order.PutUint16(pix[y*stride+x*2:], level<<10|level<<5|level)
	case FormatCBGR16, FormatCRGB16:
		level := uint16(value)
		w.view.Order.PutUint16(pix[y*stride+x*2:], (level>>3)<<11|(level>>2)<<5|level>>3)
	}
}

// SetRGBA writes a color using the destination format's color model. Gray
// formats convert the color to luminance before packing it.
func (w PackedWriter) SetRGBA(x, y int, c color.RGBA) {
	if !image.Pt(x, y).In(w.view.Bounds) {
		return
	}
	x -= w.view.Bounds.Min.X
	y -= w.view.Bounds.Min.Y

	pix := w.view.Pix
	index := y * w.view.Stride
	switch w.view.Format {
	case FormatCBGR15, FormatCBGR16, FormatCRGB15, FormatCRGB16:
		w.view.Order.PutUint16(pix[index+x*2:], packedRGBAValue(w.view.Format, c))
	default:
		gray := grayValue(c)
		w.SetGray(x+w.view.Bounds.Min.X, y+w.view.Bounds.Min.Y, gray)
	}
}

func packedViewLayout(format PixelFormat, width, height int) (rowBytes, rows int, ok bool) {
	maxInt := int(^uint(0) >> 1)
	switch format {
	case FormatMonoLSB:
		rowBytes = (width-1)/8 + 1
		rows = height
	case FormatMonoVerticalLSB:
		rowBytes = width
		rows = (height-1)/8 + 1
	case FormatGray2, FormatGray3Level:
		rowBytes = (width-1)/4 + 1
		rows = height
	case FormatGray4:
		rowBytes = (width-1)/2 + 1
		rows = height
	case FormatCBGR15, FormatCBGR16, FormatCRGB15, FormatCRGB16:
		if width > maxInt/2 {
			return 0, 0, false
		}
		rowBytes = width * 2
		rows = height
	default:
		return 0, 0, false
	}
	return rowBytes, rows, true
}

// MonoDither reports whether an 8-bit luminance value lights a monochrome
// pixel at local coordinates x, y, using a 4×4 Bayer pattern.
func MonoDither(value uint8, x, y int) bool {
	threshold := packedDither4x4[(y&3)*4+(x&3)]*16 + 7
	return value > threshold
}

func grayValue(c color.RGBA) uint8 {
	r, g, b, _ := c.RGBA()
	y := (299*r + 587*g + 114*b + 500) / 1000
	return uint8(y >> 8)
}

func packedRGBAValue(format PixelFormat, c color.RGBA) uint16 {
	r, g, b, _ := c.RGBA()
	switch format {
	case FormatCBGR15:
		return uint16((b&0xf800)>>1 | (g&0xf800)>>6 | (r&0xf800)>>11)
	case FormatCRGB15:
		return uint16((r&0xf800)>>1 | (g&0xf800)>>6 | (b&0xf800)>>11)
	case FormatCBGR16:
		return uint16(b&0xf800 | (g&0xfc00)>>5 | (r&0xf800)>>11)
	default: // FormatCRGB16
		return uint16(r&0xf800 | (g&0xfc00)>>5 | (b&0xf800)>>11)
	}
}
