package pixel

import (
	"image/color"
	"testing"
)

func FuzzImageSetFillClear(f *testing.F) {
	for format := uint8(0); format < 8; format++ {
		f.Add(format, uint8(1), uint8(1), []byte{0x00, 0x55, 0xaa, 0xff})
	}
	f.Add(uint8(0), uint8(8), uint8(9), []byte{0x00, 0x80, 0xff})
	f.Add(uint8(3), uint8(5), uint8(7), []byte{0x00, 0x11, 0x88, 0xff})

	f.Fuzz(func(t *testing.T, format, widthSeed, heightSeed uint8, data []byte) {
		width, height := int(widthSeed%33), int(heightSeed%33)
		img := fuzzPixelImage(format%8, width, height)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				index := (y*width + x) * 4
				c := color.RGBA{
					R: fuzzPixelByte(data, index),
					G: fuzzPixelByte(data, index+1),
					B: fuzzPixelByte(data, index+2),
					A: fuzzPixelByte(data, index+3),
				}
				img.Set(x, y, c)
				if got, want := img.At(x, y), img.ColorModel().Convert(c); got != want {
					t.Fatalf("format %d pixel (%d,%d) = %#v, want %#v", format%8, x, y, got, want)
				}
			}
		}

		fill := color.RGBA{
			R: fuzzPixelByte(data, len(data)),
			G: fuzzPixelByte(data, len(data)+1),
			B: fuzzPixelByte(data, len(data)+2),
			A: fuzzPixelByte(data, len(data)+3),
		}
		img.Fill(fill)
		want := img.ColorModel().Convert(fill)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				if got := img.At(x, y); got != want {
					t.Fatalf("format %d filled pixel (%d,%d) = %#v, want %#v", format%8, x, y, got, want)
				}
			}
		}

		img.Clear()
		for _, value := range img.(FastImage).PixelView().Pix {
			if value != 0 {
				t.Fatalf("format %d Clear left nonzero pixel data %#02x", format%8, value)
			}
		}
	})
}

func fuzzPixelImage(format uint8, width, height int) Image {
	switch format {
	case 0:
		return NewMonoImage(width, height)
	case 1:
		return NewMonoVerticalLSBImage(width, height)
	case 2:
		return NewGray2Image(width, height)
	case 3:
		return NewGray4Image(width, height)
	case 4:
		return NewCBGR15Image(width, height)
	case 5:
		return NewCBGR16Image(width, height)
	case 6:
		return NewCRGB15Image(width, height)
	default:
		return NewCRGB16Image(width, height)
	}
}

func fuzzPixelByte(data []byte, index int) uint8 {
	if len(data) == 0 {
		return 0
	}
	return data[index%len(data)]
}
