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

func TestCopyToXRGBMonochromePackingBoundaries(t *testing.T) {
	tests := []struct {
		name string
		img  draw.Image
		set  []image.Point
	}{
		{
			name: "horizontal bits across bytes",
			img:  pixel.NewMonoImage(9, 10),
			set:  []image.Point{image.Pt(0, 0), image.Pt(7, 0), image.Pt(8, 0), image.Pt(1, 1), image.Pt(8, 9)},
		},
		{
			name: "vertical bits across bands",
			img:  pixel.NewMonoVerticalLSBImage(9, 10),
			set:  []image.Point{image.Pt(0, 0), image.Pt(8, 7), image.Pt(4, 8), image.Pt(8, 9)},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wantOn := make(map[image.Point]bool, len(tc.set))
			for _, p := range tc.set {
				tc.img.Set(p.X, p.Y, pixel.On)
				wantOn[p] = true
			}
			const padding = 8
			bounds := tc.img.Bounds()
			stride := bounds.Dx()*4 + padding
			dst := make([]byte, stride*bounds.Dy())
			for i := range dst {
				dst[i] = 0xa5
			}
			if err := CopyToXRGB(dst, stride, tc.img); err != nil {
				t.Fatal(err)
			}
			for y := 0; y < bounds.Dy(); y++ {
				for x := 0; x < bounds.Dx(); x++ {
					level := uint32(0)
					if wantOn[image.Pt(x, y)] {
						level = 255
					}
					want := make([]byte, 4)
					binary.NativeEndian.PutUint32(want, level<<16|level<<8|level)
					for i, b := range want {
						if got := dst[y*stride+x*4+i]; got != b {
							t.Fatalf("pixel (%d,%d) byte %d = %#02x, want %#02x", x, y, i, got, b)
						}
					}
				}
				for i := bounds.Dx() * 4; i < stride; i++ {
					if dst[y*stride+i] != 0xa5 {
						t.Fatalf("row %d padding byte %d was modified", y, i)
					}
				}
			}
		})
	}
}

func TestCopyToXRGBExpandsAllPackedRGBValues(t *testing.T) {
	tests := []struct {
		name       string
		valueCount int
		make       func() (draw.Image, func(uint16))
		want       func(uint16) (uint8, uint8, uint8)
	}{
		{
			name:       "CRGB15",
			valueCount: 1 << 15,
			make: func() (draw.Image, func(uint16)) {
				img := pixel.NewCRGB15Image(256, 128)
				return img, func(v uint16) { img.Set(int(v%256), int(v/256), pixel.CRGB15{V: v}) }
			},
			want: func(v uint16) (uint8, uint8, uint8) {
				return expandExpected5(uint32(v >> 10 & 31)), expandExpected5(uint32(v >> 5 & 31)), expandExpected5(uint32(v & 31))
			},
		},
		{
			name:       "CBGR15",
			valueCount: 1 << 15,
			make: func() (draw.Image, func(uint16)) {
				img := pixel.NewCBGR15Image(256, 128)
				return img, func(v uint16) { img.Set(int(v%256), int(v/256), pixel.CBGR15{V: v}) }
			},
			want: func(v uint16) (uint8, uint8, uint8) {
				return expandExpected5(uint32(v & 31)), expandExpected5(uint32(v >> 5 & 31)), expandExpected5(uint32(v >> 10 & 31))
			},
		},
		{
			name:       "CRGB16",
			valueCount: 1 << 16,
			make: func() (draw.Image, func(uint16)) {
				img := pixel.NewCRGB16Image(256, 256)
				return img, func(v uint16) { img.Set(int(v%256), int(v/256), pixel.CRGB16{V: v}) }
			},
			want: func(v uint16) (uint8, uint8, uint8) {
				return expandExpected5(uint32(v >> 11 & 31)), expandExpected6(uint32(v >> 5 & 63)), expandExpected5(uint32(v & 31))
			},
		},
		{
			name:       "CBGR16",
			valueCount: 1 << 16,
			make: func() (draw.Image, func(uint16)) {
				img := pixel.NewCBGR16Image(256, 256)
				return img, func(v uint16) { img.Set(int(v%256), int(v/256), pixel.CBGR16{V: v}) }
			},
			want: func(v uint16) (uint8, uint8, uint8) {
				return expandExpected5(uint32(v & 31)), expandExpected6(uint32(v >> 5 & 63)), expandExpected5(uint32(v >> 11 & 31))
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			img, set := tc.make()
			for v := 0; v < tc.valueCount; v++ {
				set(uint16(v))
			}
			const stride = 256*4 + 12
			dst := make([]byte, stride*img.Bounds().Dy())
			if err := CopyToXRGB(dst, stride, img); err != nil {
				t.Fatal(err)
			}
			for v := 0; v < tc.valueCount; v++ {
				r, g, b := tc.want(uint16(v))
				want := make([]byte, 4)
				binary.NativeEndian.PutUint32(want, uint32(r)<<16|uint32(g)<<8|uint32(b))
				x, y := v%256, v/256
				for i, value := range want {
					if got := dst[y*stride+x*4+i]; got != value {
						t.Fatalf("packed value %#04x byte %d = %#02x, want %#02x", v, i, got, value)
					}
				}
			}
		})
	}
}

func expandExpected5(v uint32) uint8 { return uint8(v<<3 | v>>2) }
func expandExpected6(v uint32) uint8 { return uint8(v<<2 | v>>4) }

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

func TestCopyToXRGBSupportsCroppedRGBASubImage(t *testing.T) {
	parent := image.NewRGBA(image.Rect(0, 0, 8, 6))
	for y := parent.Bounds().Min.Y; y < parent.Bounds().Max.Y; y++ {
		for x := parent.Bounds().Min.X; x < parent.Bounds().Max.X; x++ {
			parent.SetRGBA(x, y, color.RGBA{R: uint8(x * 23), G: uint8(y * 31), B: uint8(x*7 + y*11), A: 255})
		}
	}
	src := parent.SubImage(image.Rect(3, 4, 7, 6)).(*image.RGBA)
	dst := make([]byte, src.Bounds().Dx()*src.Bounds().Dy()*4)
	if err := CopyToXRGB(dst, src.Bounds().Dx()*4, src); err != nil {
		t.Fatalf("CopyToXRGB rejected valid cropped RGBA subimage: %v", err)
	}
	for y := 0; y < src.Bounds().Dy(); y++ {
		for x := 0; x < src.Bounds().Dx(); x++ {
			c := color.RGBAModel.Convert(src.At(src.Bounds().Min.X+x, src.Bounds().Min.Y+y)).(color.RGBA)
			want := make([]byte, 4)
			binary.NativeEndian.PutUint32(want, uint32(c.R)<<16|uint32(c.G)<<8|uint32(c.B))
			for i := range want {
				if got := dst[(y*src.Bounds().Dx()+x)*4+i]; got != want[i] {
					t.Fatalf("pixel (%d,%d) byte %d = %#02x, want %#02x", x, y, i, got, want[i])
				}
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

func TestCopyToXRGBRejectsMalformedPixelStorage(t *testing.T) {
	tests := []struct {
		name string
		src  image.Image
	}{
		{name: "mono zero stride", src: malformedMono(1, 1, 0, nil)},
		{name: "mono truncated data", src: malformedMono(1, 1, 1, nil)},
		{name: "vertical mono truncated band", src: malformedVerticalMono(1, 9, 1, []byte{0xff})},
		{name: "gray2 truncated packed row", src: malformedGray2(5, 1, 2, []byte{0xff})},
		{name: "gray4 truncated packed row", src: malformedGray4(3, 1, 2, []byte{0xff})},
		{name: "rgb15 short stride", src: malformedCRGB15(1, 1, 1, []byte{0, 0})},
		{name: "rgb16 truncated rows", src: malformedCRGB16(1, 2, 2, []byte{0, 0})},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bounds := tc.src.Bounds()
			dst := make([]byte, bounds.Dx()*bounds.Dy()*4)
			if err := CopyToXRGB(dst, bounds.Dx()*4, tc.src); err == nil {
				t.Fatal("CopyToXRGB accepted malformed source storage")
			}
		})
	}
}

func malformedMono(w, h, stride int, pix []byte) *pixel.MonoImage {
	img := pixel.NewMonoImage(w, h)
	img.Stride, img.Pix = stride, pix
	return img
}

func malformedVerticalMono(w, h, stride int, pix []byte) *pixel.MonoVerticalLSBImage {
	img := pixel.NewMonoVerticalLSBImage(w, h)
	img.Stride, img.Pix = stride, pix
	return img
}

func malformedGray2(w, h, stride int, pix []byte) *pixel.Gray2Image {
	img := pixel.NewGray2Image(w, h)
	img.Stride, img.Pix = stride, pix
	return img
}

func malformedGray4(w, h, stride int, pix []byte) *pixel.Gray4Image {
	img := pixel.NewGray4Image(w, h)
	img.Stride, img.Pix = stride, pix
	return img
}

func malformedCRGB15(w, h, stride int, pix []byte) *pixel.CRGB15Image {
	img := pixel.NewCRGB15Image(w, h)
	img.Stride, img.Pix = stride, pix
	return img
}

func malformedCRGB16(w, h, stride int, pix []byte) *pixel.CRGB16Image {
	img := pixel.NewCRGB16Image(w, h)
	img.Stride, img.Pix = stride, pix
	return img
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
