package buffer

import (
	"encoding/binary"
	"image"
	"image/color"
	"testing"

	"github.com/BeatGlow/display"
	"github.com/BeatGlow/display/internal/rotation"
)

func TestCopyToXRGBRotated(t *testing.T) {
	const width, height = 5, 3
	native := image.Rect(0, 0, width, height)
	for _, r := range []display.Rotation{display.Rotate90, display.Rotate180, display.Rotate270} {
		t.Run(r.String(), func(t *testing.T) {
			logical := image.NewRGBA(rotation.Bounds(native, r))
			for y := 0; y < logical.Bounds().Dy(); y++ {
				for x := 0; x < logical.Bounds().Dx(); x++ {
					logical.SetRGBA(x, y, color.RGBA{R: uint8(x + 1), G: uint8(y + 2), B: 0x63, A: 0xff})
				}
			}
			stride := width*4 + 12
			dst := make([]byte, stride*height)
			for i := range dst {
				dst[i] = 0xee
			}
			if err := CopyToXRGBRotated(dst, stride, logical, native, r); err != nil {
				t.Fatal(err)
			}
			for y := 0; y < logical.Bounds().Dy(); y++ {
				for x := 0; x < logical.Bounds().Dx(); x++ {
					px, py := rotation.Point(native, r, x, y)
					c := logical.RGBAAt(x, y)
					var want [4]byte
					binary.NativeEndian.PutUint32(want[:], uint32(c.R)<<16|uint32(c.G)<<8|uint32(c.B))
					got := dst[py*stride+px*4 : py*stride+px*4+4]
					if string(got) != string(want[:]) {
						t.Fatalf("logical (%d,%d) at native (%d,%d) = % x; want % x", x, y, px, py, got, want)
					}
				}
			}
			for y := 0; y < height; y++ {
				for _, b := range dst[y*stride+width*4 : (y+1)*stride] {
					if b != 0xee {
						t.Fatalf("row %d padding overwritten with %#x", y, b)
					}
				}
			}
		})
	}
}

func TestCopyToXRGBRotatedTiles(t *testing.T) {
	const width, height = 12, 8
	native := image.Rect(0, 0, width, height)
	for _, r := range []display.Rotation{display.Rotate90, display.Rotate270} {
		logical := image.NewRGBA(rotation.Bounds(native, r))
		for y := 0; y < logical.Bounds().Dy(); y++ {
			for x := 0; x < logical.Bounds().Dx(); x++ {
				logical.SetRGBA(x, y, color.RGBA{R: uint8(x + 1), G: uint8(y + 2), B: 0x63, A: 0xff})
			}
		}
		dst := make([]byte, width*height*4)
		scratch := make([]byte, len(dst))
		row := make([]byte, width*4)
		if err := CopyToXRGBRotatedInto(dst, width*4, logical, native, r, scratch, row); err != nil {
			t.Fatal(err)
		}
		for y := 0; y < logical.Bounds().Dy(); y++ {
			for x := 0; x < logical.Bounds().Dx(); x++ {
				px, py := rotation.Point(native, r, x, y)
				c := logical.RGBAAt(x, y)
				got := dst[py*width*4+px*4 : py*width*4+px*4+4]
				var want [4]byte
				binary.NativeEndian.PutUint32(want[:], uint32(c.R)<<16|uint32(c.G)<<8|uint32(c.B))
				if string(got) != string(want[:]) {
					t.Fatalf("rotation %v pixel (%d,%d) = % x; want % x", r, x, y, got, want)
				}
			}
		}
	}
}

func BenchmarkCopyToXRGBRotation(b *testing.B) {
	const width, height = 800, 480
	native := image.Rect(0, 0, width, height)
	stride := width * 4
	dst := make([]byte, stride*height)
	for _, r := range []display.Rotation{display.NoRotation, display.Rotate90, display.Rotate180, display.Rotate270} {
		src := image.NewRGBA(rotation.Bounds(native, r))
		scratch := make([]byte, src.Bounds().Dx()*src.Bounds().Dy()*4)
		row := make([]byte, width*4)
		b.Run(r.String(), func(b *testing.B) {
			b.SetBytes(int64(width * height * 4))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var err error
				if r == display.NoRotation {
					err = CopyToXRGB(dst, stride, src)
				} else {
					err = CopyToXRGBRotatedInto(dst, stride, src, native, r, scratch, row)
				}
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		if r == display.Rotate90 || r == display.Rotate270 {
			b.Run(r.String()+"-Go", func(b *testing.B) {
				logicalStride := src.Bounds().Dx() * 4
				b.SetBytes(int64(width * height * 4))
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if err := CopyToXRGB(scratch, logicalStride, src); err != nil {
						b.Fatal(err)
					}
					if r == display.Rotate90 {
						rotateXRGB90Go(dst, stride, scratch, logicalStride, width, height)
					} else {
						rotateXRGB270Go(dst, stride, scratch, logicalStride, width, height)
					}
				}
			})
		}
	}
}
