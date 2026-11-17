package pixel

import (
	"errors"
	"fmt"
	"image"
	"image/color"
)

var errInvalidYU12Source = errors.New("pixel: invalid RGBA source for YU12 image")

// YU12Image stores an 8-bit Y plane followed by 8-bit U and V planes, each
// subsampled by two in both dimensions. This is the planar YUV420 (YU12)
// layout used by V4L2. The image dimensions should be even for direct use as
// a V4L2 YUV420Frame; odd dimensions use rounded-up chroma plane sizes.
type YU12Image struct {
	Buffer
	chromaStride int
	chromaHeight int
	ySize        int
	uOffset      int
	vOffset      int
}

// YU12Planes is a borrowed view of a YU12Image's planes. Slices include row
// padding. The view is valid only while the image's backing memory is valid;
// callers must not retain it beyond that lifetime or mutate it concurrently.
type YU12Planes struct {
	Y, U, V                   []byte
	Width, Height             int
	ChromaWidth, ChromaHeight int
	YStride, UVStride         int
}

// NewYU12Image allocates a planar YUV 4:2:0 image with the given dimensions.
func NewYU12Image(w, h int) *YU12Image {
	chromaStride := w/2 + w%2
	chromaHeight := h/2 + h%2
	ySize := w * h
	chromaSize := chromaStride * chromaHeight
	return &YU12Image{
		Buffer:       makeBuffer(w, h, w, ySize+2*chromaSize),
		chromaStride: chromaStride,
		chromaHeight: chromaHeight,
		ySize:        ySize,
		uOffset:      ySize,
		vOffset:      ySize + chromaSize,
	}
}

// WrapYU12 wraps caller-owned contiguous Y, U, and V storage without copying.
// Plane offsets are YStride*height, then UVStride*ceil(height/2), with chroma
// dimensions ceil(width/2) by ceil(height/2). The caller retains ownership,
// must keep storage valid while the image or its plane views are used, and
// must not reuse or mutate it concurrently. Padding bytes are included in the
// required buffer length.
func WrapYU12(pix []byte, width, height, yStride, uvStride int) (*YU12Image, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("pixel: invalid YU12 dimensions %dx%d", width, height)
	}
	cw, ch := width/2+width%2, height/2+height%2
	if yStride < width {
		return nil, fmt.Errorf("pixel: YU12 Y stride %d is less than width %d", yStride, width)
	}
	if uvStride < cw {
		return nil, fmt.Errorf("pixel: YU12 UV stride %d is less than chroma width %d", uvStride, cw)
	}
	if yStride > int(^uint(0)>>1)/height {
		return nil, fmt.Errorf("pixel: YU12 plane size overflows int")
	}
	ySize := yStride * height
	maxInt := int(^uint(0) >> 1)
	if ch > maxInt/2 || uvStride > (maxInt-ySize)/(2*ch) {
		return nil, fmt.Errorf("pixel: YU12 buffer size overflows int")
	}
	uSize := uvStride * ch
	need := ySize + 2*uSize
	if len(pix) < need {
		return nil, fmt.Errorf("pixel: YU12 buffer too short: got %d bytes, need %d", len(pix), need)
	}
	return &YU12Image{Buffer: Buffer{Rect: image.Rect(0, 0, width, height), Pix: pix[:need], Stride: yStride}, chromaStride: uvStride, chromaHeight: ch, ySize: ySize, uOffset: ySize, vOffset: ySize + uSize}, nil
}

// Planes returns a lightweight borrowed view of the image storage.
func (p *YU12Image) Planes() YU12Planes {
	uSize := p.chromaStride * p.chromaHeight
	w := p.Rect.Dx()
	return YU12Planes{Y: p.Pix[:p.ySize], U: p.Pix[p.uOffset:p.vOffset], V: p.Pix[p.vOffset : p.vOffset+uSize], Width: w, Height: p.Rect.Dy(), ChromaWidth: w/2 + w%2, ChromaHeight: p.chromaHeight, YStride: p.Stride, UVStride: p.chromaStride}
}

func (p *YU12Image) ColorModel() color.Model {
	return color.YCbCrModel
}

func (p *YU12Image) chromaOffsets(x, y int) (int, int) {
	chroma := (y/2)*p.chromaStride + x/2
	u := p.uOffset + chroma
	v := p.vOffset + chroma
	return u, v
}

func (p *YU12Image) At(x, y int) color.Color {
	if !(image.Point{X: x, Y: y}).In(p.Rect) {
		return color.Transparent
	}
	u, v := p.chromaOffsets(x, y)
	return color.YCbCr{Y: p.Pix[y*p.Stride+x], Cb: p.Pix[u], Cr: p.Pix[v]}
}

// Set stores the pixel's luma and updates the U and V sample shared by its
// surrounding 2x2 pixel block.
func (p *YU12Image) Set(x, y int, c color.Color) {
	if !(image.Point{X: x, Y: y}).In(p.Rect) {
		return
	}
	r, g, b, _ := c.RGBA()
	y8, u8, v8 := color.RGBToYCbCr(uint8(r>>8), uint8(g>>8), uint8(b>>8))
	p.Pix[y*p.Stride+x] = y8
	u, v := p.chromaOffsets(x, y)
	p.Pix[u], p.Pix[v] = u8, v8
}

// CopyRGBA converts src into p. Each 2x2 pixel block shares chroma computed
// from its average RGB value, as expected by planar YUV 4:2:0.
// Rows use an architecture-specific SIMD kernel for luma, with a scalar tail.
func (p *YU12Image) CopyRGBA(src *image.RGBA) error {
	if src == nil {
		return errInvalidYU12Source
	}
	bounds := src.Bounds()
	w, h := p.Rect.Dx(), p.Rect.Dy()
	if bounds.Dx() != w || bounds.Dy() != h || src.Stride < w*4 {
		return errInvalidYU12Source
	}
	if w > 0 && h > 0 {
		start := src.PixOffset(bounds.Min.X, bounds.Min.Y)
		end := src.PixOffset(bounds.Max.X-1, bounds.Max.Y-1) + 4
		if start < 0 || end > len(src.Pix) {
			return errInvalidYU12Source
		}
	}
	for y := 0; y < h; y++ {
		srcRow := src.Pix[src.PixOffset(bounds.Min.X, bounds.Min.Y+y):]
		dstRow := p.Pix[y*p.Stride:]
		x := copyYU12LumaChunk(dstRow[:w], srcRow[:w*4])
		for ; x < w; x++ {
			i := x * 4
			y8, _, _ := color.RGBToYCbCr(srcRow[i], srcRow[i+1], srcRow[i+2])
			dstRow[x] = y8
		}
	}
	for y := 0; y < h; y += 2 {
		row0 := src.Pix[src.PixOffset(bounds.Min.X, bounds.Min.Y+y):]
		var row1 []byte
		if y+1 < h {
			row1 = src.Pix[src.PixOffset(bounds.Min.X, bounds.Min.Y+y+1):]
		}
		for x := 0; x < w; x += 2 {
			var r, g, b, n int
			for dx := 0; dx < 2 && x+dx < w; dx++ {
				i := (x + dx) * 4
				r += int(row0[i])
				g += int(row0[i+1])
				b += int(row0[i+2])
				n++
				if row1 != nil {
					r += int(row1[i])
					g += int(row1[i+1])
					b += int(row1[i+2])
					n++
				}
			}
			_, cb, cr := color.RGBToYCbCr(uint8((r+n/2)/n), uint8((g+n/2)/n), uint8((b+n/2)/n))
			u, v := p.chromaOffsets(x, y)
			p.Pix[u], p.Pix[v] = cb, cr
		}
	}
	return nil
}

// Fill converts c to YUV and fills all three planes.
func (p *YU12Image) Fill(c color.Color) {
	r, g, b, _ := c.RGBA()
	y8, u8, v8 := color.RGBToYCbCr(uint8(r>>8), uint8(g>>8), uint8(b>>8))
	uSize := p.chromaStride * p.chromaHeight
	fillRepeatedByte(p.Pix[:p.ySize], y8)
	fillRepeatedByte(p.Pix[p.uOffset:p.uOffset+uSize], u8)
	fillRepeatedByte(p.Pix[p.vOffset:p.vOffset+uSize], v8)
}
