package buffer

import (
	"encoding/binary"
	"image"
	"testing"

	"github.com/BeatGlow/display/pixel"
)

type genericFuzzImage struct{ image.Image }

func FuzzCopyToXRGBMatchesGenericImage(f *testing.F) {
	for format := uint8(0); format < 8; format++ {
		f.Add(format, uint8(1), uint8(1), uint8(0), uint8(0), []byte{0x00, 0x55, 0xaa, 0xff})
	}
	f.Add(uint8(0), uint8(8), uint8(9), uint8(12), uint8(0), []byte{0x81, 0x18})
	f.Add(uint8(3), uint8(5), uint8(7), uint8(3), uint8(1), []byte{0x01, 0x23, 0x45, 0x67})

	f.Fuzz(func(t *testing.T, format, widthSeed, heightSeed, paddingSeed, orderSeed uint8, data []byte) {
		width, height := int(widthSeed%33), int(heightSeed%33)
		src := fuzzBufferImage(format%8, width, height, orderSeed&1 == 1)
		view := src.(pixel.FastImage).PixelView()
		for i := range view.Pix {
			view.Pix[i] = fuzzBufferByte(data, i)
		}

		stride := width*4 + int(paddingSeed%17)
		dst := make([]byte, stride*height)
		generic := make([]byte, len(dst))
		for i := range dst {
			dst[i] = 0xa5
			generic[i] = 0xa5
		}
		if err := CopyToXRGB(dst, stride, src); err != nil {
			t.Fatalf("optimized CopyToXRGB: %v", err)
		}
		if err := CopyToXRGB(generic, stride, genericFuzzImage{Image: src}); err != nil {
			t.Fatalf("generic CopyToXRGB: %v", err)
		}
		for i := range dst {
			if dst[i] != generic[i] {
				t.Fatalf("destination byte %d = %#02x, generic path has %#02x", i, dst[i], generic[i])
			}
		}
	})
}

func fuzzBufferImage(format uint8, width, height int, littleEndian bool) image.Image {
	var order binary.ByteOrder = binary.BigEndian
	if littleEndian {
		order = binary.LittleEndian
	}
	switch format {
	case 0:
		return pixel.NewMonoImage(width, height)
	case 1:
		return pixel.NewMonoVerticalLSBImage(width, height)
	case 2:
		return pixel.NewGray2Image(width, height)
	case 3:
		return pixel.NewGray4Image(width, height)
	case 4:
		img := pixel.NewCBGR15Image(width, height)
		img.Order = order
		return img
	case 5:
		img := pixel.NewCBGR16Image(width, height)
		img.Order = order
		return img
	case 6:
		img := pixel.NewCRGB15Image(width, height)
		img.Order = order
		return img
	default:
		img := pixel.NewCRGB16Image(width, height)
		img.Order = order
		return img
	}
}

func fuzzBufferByte(data []byte, index int) uint8 {
	if len(data) == 0 {
		return 0
	}
	return data[index%len(data)]
}
