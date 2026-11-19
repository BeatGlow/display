package scope

import (
	"errors"
	"image"
	"image/color"
	"image/draw"

	"github.com/BeatGlow/display"
	"github.com/BeatGlow/display/pixel"
)

var errBounds = errors.New("scope: destination bounds must match renderer bounds")

var bayer = [16]uint8{0, 8, 2, 10, 12, 4, 14, 6, 3, 11, 1, 9, 15, 7, 13, 5}

func modelMode(model color.Model) int {
	switch model {
	case pixel.MonoModel:
		return 1
	case pixel.Gray2Model:
		return 2
	case pixel.Gray4Model:
		return 4
	case color.GrayModel, color.Gray16Model:
		return 8
	default:
		return 0
	}
}

// RenderTo draws the current phosphor state without transferring it to hardware.
// The destination must retain the bounds supplied to NewRenderer. Calls on a
// Renderer require exclusive ownership. No per-frame heap storage is created.
func (r *Renderer) RenderTo(dst draw.Image) error {
	if dst == nil || dst.Bounds() != r.bounds {
		return errBounds
	}
	if provider, ok := dst.(interface{ DrawBuffer() *image.RGBA }); ok {
		if buffer := provider.DrawBuffer(); buffer != nil && buffer.Bounds() == r.bounds {
			dst = buffer
		}
	}
	r.renderEnergy()
	mode := modelMode(dst.ColorModel())
	if rgba, ok := dst.(*image.RGBA); ok {
		for y := 0; y < r.height; y++ {
			row := rgba.Pix[y*rgba.Stride:]
			for x := 0; x < r.width; x++ {
				c := r.colors[r.lightIndex(y*r.width+x)]
				i := x * 4
				row[i], row[i+1], row[i+2], row[i+3] = c.R, c.G, c.B, 255
			}
		}
		return nil
	}
	if fast, ok := dst.(pixel.FastImage); ok {
		v := fast.PixelView()
		if v.Bounds == r.bounds && r.fastBlit(v) {
			return nil
		}
	}
	for y := 0; y < r.height; y++ {
		for x := 0; x < r.width; x++ {
			index := r.lightIndex(y*r.width + x)
			c := r.lut[index]
			if mode == 1 {
				on := r.grayValues[index] > bayer[(y&3)*4+(x&3)]*16+7
				c = r.monoLUT[0]
				if on {
					c = r.monoLUT[1]
				}
			} else if mode > 0 {
				c = r.grayLUT[index]
			}
			dst.Set(r.bounds.Min.X+x, r.bounds.Min.Y+y, c)
		}
	}
	return nil
}

func (r *Renderer) lightIndex(i int) int {
	uv := r.remap[i]
	var e float32
	for j, index := range uv.index {
		if index >= 0 {
			e += r.bloom[index] * uv.weight[j]
		}
	}
	return toneIndex(e*float32(r.cfg.Exposure) + r.base[i])
}

func (r *Renderer) fastBlit(v pixel.PixelView) bool {
	switch v.Format {
	case pixel.FormatCRGB16, pixel.FormatGray4, pixel.FormatGray2, pixel.FormatMonoLSB, pixel.FormatMonoVerticalLSB:
	default:
		return false
	}
	// FastImage storage is borrowed only for this call. Other layouts use Set.
	if v.Stride < 0 {
		return false
	}
	rowBytes := r.width
	rows := r.height
	switch v.Format {
	case pixel.FormatCRGB16:
		rowBytes *= 2
	case pixel.FormatGray4:
		rowBytes = (r.width + 1) / 2
	case pixel.FormatGray2:
		rowBytes = (r.width + 3) / 4
	case pixel.FormatMonoLSB:
		rowBytes = (r.width + 7) / 8
	case pixel.FormatMonoVerticalLSB:
		rows = (r.height + 7) / 8
	}
	if v.Stride < rowBytes || len(v.Pix) < rowBytes || (rows > 1 && v.Stride > (len(v.Pix)-rowBytes)/(rows-1)) {
		return false
	}
	if v.Format == pixel.FormatCRGB16 && v.Order == nil {
		return false
	}
	for y := 0; y < r.height; y++ {
		for x := 0; x < r.width; x++ {
			index := r.lightIndex(y*r.width + x)
			value := r.grayValues[index]
			switch v.Format {
			case pixel.FormatCRGB16:
				c := r.colors[index]
				word := uint16(c.R>>3)<<11 | uint16(c.G>>2)<<5 | uint16(c.B>>3)
				v.Order.PutUint16(v.Pix[y*v.Stride+x*2:], word)
			case pixel.FormatGray4:
				i, shift := y*v.Stride+x/2, uint(4*(1-x%2))
				v.Pix[i] = (v.Pix[i] &^ (15 << shift)) | (value>>4)<<shift
			case pixel.FormatGray2:
				i, shift := y*v.Stride+x/4, uint(2*(3-x%4))
				v.Pix[i] = (v.Pix[i] &^ (3 << shift)) | (value>>6)<<shift
			default:
				i, bit := y*v.Stride+x/8, uint(x&7)
				if v.Format == pixel.FormatMonoVerticalLSB {
					i, bit = (y/8)*v.Stride+x, uint(y&7)
				}
				mask := byte(1 << bit)
				v.Pix[i] &^= mask
				if value > bayer[(y&3)*4+(x&3)]*16+7 {
					v.Pix[i] |= mask
				}
			}
		}
	}
	return true
}

// BlitToDisplay renders and transfers the completed buffer using Refresh.
// Display transport errors are returned to the caller. Transport allocations
// and transfer time are owned by the selected driver.
func (r *Renderer) BlitToDisplay(dst display.Display) error {
	if err := r.RenderTo(dst); err != nil {
		return err
	}
	return dst.Refresh()
}
