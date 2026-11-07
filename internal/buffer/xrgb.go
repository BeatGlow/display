// Package buffer converts display image formats to scanout buffers.
package buffer

import (
	"encoding/binary"
	"errors"
	"image"

	"github.com/BeatGlow/display/v2/pixel"
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
	return srcStride >= w*srcBytesPerPixel && len(src) >= srcStride*h && len(dst) >= dstStride*h
}

func byteOrder(order binary.ByteOrder) binary.ByteOrder {
	if order == nil {
		return binary.BigEndian
	}
	return order
}

func expand5(value uint8) uint8 { return value<<3 | value>>2 }
func expand6(value uint8) uint8 { return value<<2 | value>>4 }
