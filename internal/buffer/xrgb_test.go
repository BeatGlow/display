package buffer

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"testing"

	"github.com/BeatGlow/display/pixel"
)

func TestCopyToXRGBSupportsPixelImages(t *testing.T) {
	images := []struct {
		name  string
		image draw.Image
	}{
		{name: "mono", image: pixel.NewMonoImage(5, 3)},
		{name: "mono-vertical-lsb", image: pixel.NewMonoVerticalLSBImage(5, 3)},
		{name: "gray2", image: pixel.NewGray2Image(5, 3)},
		{name: "gray4", image: pixel.NewGray4Image(5, 3)},
		{name: "crgb15", image: pixel.NewCRGB15Image(5, 3)},
		{name: "cbgr15", image: pixel.NewCBGR15Image(5, 3)},
		{name: "crgb16", image: pixel.NewCRGB16Image(5, 3)},
		{name: "cbgr16", image: pixel.NewCBGR16Image(5, 3)},
	}
	colors := []color.RGBA{
		{R: 255, A: 255}, {G: 255, A: 255}, {B: 255, A: 255},
		{R: 230, G: 120, B: 17, A: 255}, {R: 20, G: 80, B: 240, A: 255},
	}

	for _, tc := range images {
		t.Run(tc.name, func(t *testing.T) {
			bounds := tc.image.Bounds()
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					tc.image.Set(x, y, colors[(x+y)%len(colors)])
				}
			}

			const padding = 12
			stride := bounds.Dx()*4 + padding
			dst := make([]byte, stride*bounds.Dy())
			for i := range dst {
				dst[i] = 0xa5
			}
			if err := CopyToXRGB(dst, stride, tc.image); err != nil {
				t.Fatalf("CopyToXRGB returned error: %v", err)
			}

			for y := 0; y < bounds.Dy(); y++ {
				for x := 0; x < bounds.Dx(); x++ {
					c := color.RGBAModel.Convert(tc.image.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.RGBA)
					want := make([]byte, 4)
					binary.NativeEndian.PutUint32(want, uint32(c.R)<<16|uint32(c.G)<<8|uint32(c.B))
					offset := y*stride + x*4
					for i := range want {
						if dst[offset+i] != want[i] {
							t.Fatalf("pixel (%d,%d) byte %d = %#02x, want %#02x", x, y, i, dst[offset+i], want[i])
						}
					}
				}
				for i := bounds.Dx() * 4; i < stride; i++ {
					if got := dst[y*stride+i]; got != 0xa5 {
						t.Fatalf("row padding byte %d = %#02x, want unchanged %#02x", i, got, 0xa5)
					}
				}
			}
		})
	}
}

func TestCopyToXRGBFallsBackForOtherImages(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 200, G: 100, B: 50, A: 128})
	src.SetNRGBA(1, 0, color.NRGBA{R: 9, G: 80, B: 210, A: 255})
	dst := make([]byte, 8)
	if err := CopyToXRGB(dst, 8, src); err != nil {
		t.Fatalf("CopyToXRGB returned error: %v", err)
	}
	for x := 0; x < 2; x++ {
		c := color.RGBAModel.Convert(src.At(x, 0)).(color.RGBA)
		want := make([]byte, 4)
		binary.NativeEndian.PutUint32(want, uint32(c.R)<<16|uint32(c.G)<<8|uint32(c.B))
		for i := range want {
			if got := dst[x*4+i]; got != want[i] {
				t.Fatalf("pixel %d byte %d = %#02x, want %#02x", x, i, got, want[i])
			}
		}
	}
}

func TestCopyToXRGBRejectsInvalidDestination(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	if err := CopyToXRGB(nil, 7, src); err == nil {
		t.Fatal("CopyToXRGB accepted a stride shorter than one row")
	}
	if err := CopyToXRGB(make([]byte, 15), 8, src); err == nil {
		t.Fatal("CopyToXRGB accepted a destination smaller than two rows")
	}
	if err := CopyToXRGB(nil, 0, nil); err == nil {
		t.Fatal("CopyToXRGB accepted a nil source")
	}
}

func TestCopyRGBAVectorAndScalarTail(t *testing.T) {
	const width, height, stride = 37, 3, 37*4 + 12
	src := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := range src.Pix {
		src.Pix[i] = byte(i*37 + 11)
	}
	dst := make([]byte, stride*height)
	if err := CopyToXRGB(dst, stride, src); err != nil {
		t.Fatal(err)
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := y*stride + x*4
			s := y*src.Stride + x*4
			want := [4]byte{src.Pix[s+2], src.Pix[s+1], src.Pix[s], 0}
			for channel := range want {
				if dst[i+channel] != want[channel] {
					t.Fatalf("pixel (%d,%d) channel %d = %#02x, want %#02x", x, y, channel, dst[i+channel], want[channel])
				}
			}
		}
	}
}

func BenchmarkRGBAConversion(b *testing.B) {
	for _, size := range []image.Point{{X: 400, Y: 1280}, {X: 256, Y: 64}} {
		b.Run(fmt.Sprintf("%dx%d/optimized", size.X, size.Y), func(b *testing.B) {
			benchmarkRGBAConversion(b, size, CopyToXRGB)
		})
		b.Run(fmt.Sprintf("%dx%d/scalar", size.X, size.Y), func(b *testing.B) {
			benchmarkRGBAConversion(b, size, copyToXRGBScalar)
		})
	}
}

func benchmarkRGBAConversion(b *testing.B, size image.Point, convert func([]byte, int, image.Image) error) {
	src := image.NewRGBA(image.Rect(0, 0, size.X, size.Y))
	dst := make([]byte, size.X*size.Y*4)
	b.SetBytes(int64(len(dst)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := convert(dst, size.X*4, src); err != nil {
			b.Fatal(err)
		}
	}
}

func copyToXRGBScalar(dst []byte, stride int, src image.Image) error {
	rgba := src.(*image.RGBA)
	bounds := rgba.Bounds()
	for y := 0; y < bounds.Dy(); y++ {
		source := rgba.Pix[rgba.PixOffset(bounds.Min.X, bounds.Min.Y+y):]
		row := dst[y*stride:]
		for x := 0; x < bounds.Dx(); x++ {
			i := x * 4
			putXRGB(row[i:], source[i], source[i+1], source[i+2])
		}
	}
	return nil
}
