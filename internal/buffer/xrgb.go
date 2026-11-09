package buffer

import (
	"encoding/binary"
	"errors"
	"image"

	"github.com/BeatGlow/display"
	"github.com/BeatGlow/display/internal/rotation"
	"github.com/BeatGlow/display/pixel"
)

const xrgbBytesPerPixel = 4

// CopyToXRGB copies src into a native-endian XRGB8888 scanout buffer. The
// destination stride is measured in bytes and may include row padding.
func CopyToXRGB(dst []byte, stride int, src image.Image) error {
	if src == nil {
		return errors.New("buffer: source image is nil")
	}
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w < 0 || h < 0 || stride < w*xrgbBytesPerPixel {
		return errors.New("buffer: invalid scanout dimensions or stride")
	}
	if h > 0 && stride > len(dst)/h {
		return errors.New("buffer: destination is too small")
	}
	if w == 0 || h == 0 {
		return nil
	}

	switch src := src.(type) {
	case *image.RGBA:
		return copyRGBA(dst, stride, src)
	case *pixel.MonoImage:
		return copyMono(dst, stride, src.Pix, src.Stride, bounds, false)
	case *pixel.MonoVerticalLSBImage:
		return copyMono(dst, stride, src.Pix, src.Stride, bounds, true)
	case *pixel.Gray2Image:
		return copyGray2(dst, stride, src)
	case *pixel.Gray4Image:
		return copyGray4(dst, stride, src)
	case *pixel.CRGB15Image:
		return copyRGB15(dst, stride, src.Pix, src.Stride, bounds, byteOrder(src.Order), false)
	case *pixel.CBGR15Image:
		return copyRGB15(dst, stride, src.Pix, src.Stride, bounds, byteOrder(src.Order), true)
	case *pixel.CRGB16Image:
		return copyRGB16(dst, stride, src.Pix, src.Stride, bounds, byteOrder(src.Order), false)
	case *pixel.CBGR16Image:
		return copyRGB16(dst, stride, src.Pix, src.Stride, bounds, byteOrder(src.Order), true)
	default:
		return copyGeneric(dst, stride, src, bounds)
	}
}

// CopyToXRGBRotated copies an RGBA image in logical orientation into a native
// XRGB8888 scanout buffer. The destination stride is measured in bytes.
func CopyToXRGBRotated(dst []byte, stride int, src *image.RGBA, nativeBounds image.Rectangle, r display.Rotation) error {
	if src == nil {
		return errors.New("buffer: source image is nil")
	}
	scratch := make([]byte, src.Bounds().Dx()*src.Bounds().Dy()*4)
	row := make([]byte, nativeBounds.Dx()*4)
	return CopyToXRGBRotatedInto(dst, stride, src, nativeBounds, r, scratch, row)
}

// CopyToXRGBRotatedInto copies a rotated RGBA image using caller-owned scratch
// storage so repeated scanout conversions do not allocate.
func CopyToXRGBRotatedInto(dst []byte, stride int, src *image.RGBA, nativeBounds image.Rectangle, r display.Rotation, scratch, row []byte) error {
	if src == nil {
		return errors.New("buffer: source image is nil")
	}
	if r == display.NoRotation {
		return CopyToXRGB(dst, stride, src)
	}
	logicalBounds := rotation.Bounds(nativeBounds, r)
	w, h := nativeBounds.Dx(), nativeBounds.Dy()
	if w <= 0 || h <= 0 || stride < w*xrgbBytesPerPixel || src.Bounds().Size() != logicalBounds.Size() {
		return errors.New("buffer: invalid rotated scanout dimensions")
	}
	sourceBounds := src.Bounds()
	sourceEnd := src.PixOffset(sourceBounds.Max.X-1, sourceBounds.Max.Y-1) + 4
	if stride > len(dst)/h || src.Stride < sourceBounds.Dx()*4 || sourceEnd > len(src.Pix) || len(scratch) < sourceBounds.Dx()*sourceBounds.Dy()*4 || len(row) < w*4 {
		return errors.New("buffer: rotated source or destination buffer is too small")
	}
	if err := CopyToXRGB(scratch, sourceBounds.Dx()*4, src); err != nil {
		return err
	}
	logicalStride := sourceBounds.Dx() * 4
	if r%4 == display.Rotate90 {
		rotateXRGB90(dst, stride, scratch, logicalStride, w, h)
		return nil
	}
	if r%4 == display.Rotate270 {
		rotateXRGB270(dst, stride, scratch, logicalStride, w, h)
		return nil
	}
	if r%4 == display.Rotate180 {
		rotateXRGB180(dst, stride, scratch, logicalStride, w, h)
		return nil
	}
	for y := 0; y < h; y++ {
		destination := dst[y*stride:]
		switch r % 4 {
		case display.Rotate90:
			for x := 0; x < w; x++ {
				offset := x*logicalStride + (h-1-y)*4
				destinationOffset := x * 4
				row[destinationOffset], row[destinationOffset+1] = scratch[offset], scratch[offset+1]
				row[destinationOffset+2], row[destinationOffset+3] = scratch[offset+2], scratch[offset+3]
			}
		case display.Rotate180:
			source := scratch[(h-1-y)*logicalStride:]
			for x := 0; x < w; x++ {
				offset := (w - 1 - x) * 4
				destinationOffset := x * 4
				row[destinationOffset], row[destinationOffset+1] = source[offset], source[offset+1]
				row[destinationOffset+2], row[destinationOffset+3] = source[offset+2], source[offset+3]
			}
		case display.Rotate270:
			for x := 0; x < w; x++ {
				offset := (w-1-x)*logicalStride + y*4
				destinationOffset := x * 4
				row[destinationOffset], row[destinationOffset+1] = scratch[offset], scratch[offset+1]
				row[destinationOffset+2], row[destinationOffset+3] = scratch[offset+2], scratch[offset+3]
			}
		default:
			copy(row[:w*4], scratch[y*logicalStride:y*logicalStride+w*4])
		}
		copy(destination[:w*4], row[:w*4])
	}
	return nil
}

func rotateXRGB90Go(dst []byte, dstStride int, src []byte, srcStride, width, height int) {
	const tile = 16
	for y0 := 0; y0 < height; y0 += tile {
		for x0 := 0; x0 < width; x0 += tile {
			for y := y0; y < min(y0+tile, height); y++ {
				destination := dst[y*dstStride:]
				for x := x0; x < min(x0+tile, width); x++ {
					sourceOffset, destinationOffset := x*srcStride+(height-1-y)*4, x*4
					destination[destinationOffset], destination[destinationOffset+1] = src[sourceOffset], src[sourceOffset+1]
					destination[destinationOffset+2], destination[destinationOffset+3] = src[sourceOffset+2], src[sourceOffset+3]
				}
			}
		}
	}
}

func rotateXRGB270Go(dst []byte, dstStride int, src []byte, srcStride, width, height int) {
	const tile = 16
	for y0 := 0; y0 < height; y0 += tile {
		for x0 := 0; x0 < width; x0 += tile {
			for y := y0; y < min(y0+tile, height); y++ {
				destination := dst[y*dstStride:]
				for x := x0; x < min(x0+tile, width); x++ {
					sourceOffset, destinationOffset := (width-1-x)*srcStride+y*4, x*4
					destination[destinationOffset], destination[destinationOffset+1] = src[sourceOffset], src[sourceOffset+1]
					destination[destinationOffset+2], destination[destinationOffset+3] = src[sourceOffset+2], src[sourceOffset+3]
				}
			}
		}
	}
}

func rotateXRGB180Go(dst []byte, dstStride int, src []byte, srcStride, width, height int) {
	for y := 0; y < height; y++ {
		destination := dst[y*dstStride:]
		source := src[(height-1-y)*srcStride:]
		for x := 0; x < width; x++ {
			sourceOffset, destinationOffset := (width-1-x)*4, x*4
			copy(destination[destinationOffset:destinationOffset+4], source[sourceOffset:sourceOffset+4])
		}
	}
}

func copyRGBA(dst []byte, stride int, src *image.RGBA) error {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if !validSource(dst, stride, src.Pix, src.Stride, bounds, 4) {
		return errors.New("buffer: invalid RGBA source buffer")
	}
	for y := 0; y < h; y++ {
		srcRow := src.Pix[src.PixOffset(bounds.Min.X, bounds.Min.Y+y):]
		dstRow := dst[y*stride:]
		x := copyRGBAChunk(dstRow[:w*4], srcRow[:w*4])
		for ; x < w; x++ {
			i := x * 4
			putXRGB(dstRow[i:], srcRow[i], srcRow[i+1], srcRow[i+2])
		}
	}
	return nil
}

func copyMono(dst []byte, dstStride int, pix []byte, srcStride int, bounds image.Rectangle, vertical bool) error {
	w, h := bounds.Dx(), bounds.Dy()
	if srcStride <= 0 {
		return errors.New("buffer: invalid monochrome source stride")
	}
	for y := 0; y < h; y++ {
		row := dst[y*dstStride:]
		for x := 0; x < w; x++ {
			var value byte
			if vertical {
				index := (y/8)*srcStride + x
				if index >= len(pix) {
					return errors.New("buffer: monochrome source is too small")
				}
				value = (pix[index] >> uint(y&7)) & 1
			} else {
				index := y*srcStride + x/8
				if index >= len(pix) {
					return errors.New("buffer: monochrome source is too small")
				}
				value = (pix[index] >> uint(x&7)) & 1
			}
			level := value * 255
			putXRGB(row[x*4:], level, level, level)
		}
	}
	return nil
}

func copyGray2(dst []byte, dstStride int, src *pixel.Gray2Image) error {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if src.Stride < (w+3)/4 || len(src.Pix) < src.Stride*h {
		return errors.New("buffer: invalid Gray2 source buffer")
	}
	for y := 0; y < h; y++ {
		row := dst[y*dstStride:]
		for x := 0; x < w; x++ {
			packed := src.Pix[y*src.Stride+x/4]
			level := (packed >> uint((3-x&3)*2)) & 3
			gray := level * 68 // Match pixel.Gray2.RGBA().
			putXRGB(row[x*4:], gray, gray, gray)
		}
	}
	return nil
}

func copyGray4(dst []byte, dstStride int, src *pixel.Gray4Image) error {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if src.Stride < (w+1)/2 || len(src.Pix) < src.Stride*h {
		return errors.New("buffer: invalid Gray4 source buffer")
	}
	for y := 0; y < h; y++ {
		row := dst[y*dstStride:]
		for x := 0; x < w; x++ {
			packed := src.Pix[y*src.Stride+x/2]
			level := packed & 0x0f
			if x&1 == 0 {
				level = packed >> 4
			}
			gray := level | level<<4
			putXRGB(row[x*4:], gray, gray, gray)
		}
	}
	return nil
}

func copyRGB15(dst []byte, dstStride int, pix []byte, srcStride int, bounds image.Rectangle, order binary.ByteOrder, bgr bool) error {
	w, h := bounds.Dx(), bounds.Dy()
	if srcStride < w*2 || len(pix) < srcStride*h {
		return errors.New("buffer: invalid 15-bit RGB source buffer")
	}
	for y := 0; y < h; y++ {
		srcRow, dstRow := pix[y*srcStride:], dst[y*dstStride:]
		for x := 0; x < w; x++ {
			value := order.Uint16(srcRow[x*2:])
			red, green, blue := uint8(value>>10&0x1f), uint8(value>>5&0x1f), uint8(value&0x1f)
			if bgr {
				red, blue = blue, red
			}
			putXRGB(dstRow[x*4:], expand5(red), expand5(green), expand5(blue))
		}
	}
	return nil
}

func copyRGB16(dst []byte, dstStride int, pix []byte, srcStride int, bounds image.Rectangle, order binary.ByteOrder, bgr bool) error {
	w, h := bounds.Dx(), bounds.Dy()
	if srcStride < w*2 || len(pix) < srcStride*h {
		return errors.New("buffer: invalid 16-bit RGB source buffer")
	}
	for y := 0; y < h; y++ {
		srcRow, dstRow := pix[y*srcStride:], dst[y*dstStride:]
		for x := 0; x < w; x++ {
			value := order.Uint16(srcRow[x*2:])
			red, green, blue := uint8(value>>11&0x1f), uint8(value>>5&0x3f), uint8(value&0x1f)
			if bgr {
				red, blue = blue, red
			}
			putXRGB(dstRow[x*4:], expand5(red), expand6(green), expand5(blue))
		}
	}
	return nil
}

func copyGeneric(dst []byte, stride int, src image.Image, bounds image.Rectangle) error {
	w, h := bounds.Dx(), bounds.Dy()
	for y := 0; y < h; y++ {
		row := dst[y*stride:]
		for x := 0; x < w; x++ {
			r, g, b, _ := src.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			putXRGB(row[x*4:], uint8(r>>8), uint8(g>>8), uint8(b>>8))
		}
	}
	return nil
}

func validSource(dst []byte, dstStride int, src []byte, srcStride int, bounds image.Rectangle, srcBytesPerPixel int) bool {
	w, h := bounds.Dx(), bounds.Dy()
	if srcStride < w*srcBytesPerPixel || len(dst) < dstStride*h {
		return false
	}
	if h == 0 {
		return true
	}
	return len(src) >= (h-1)*srcStride+w*srcBytesPerPixel
}

func byteOrder(order binary.ByteOrder) binary.ByteOrder {
	if order == nil {
		return binary.BigEndian
	}
	return order
}

func expand5(value uint8) uint8 { return value<<3 | value>>2 }
func expand6(value uint8) uint8 { return value<<2 | value>>4 }
