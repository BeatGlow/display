package buffer

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"runtime"
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
	for _, r := range []display.Rotation{display.Rotate90, display.Rotate180, display.Rotate270} {
		logicalBounds := rotation.Bounds(native, r).Add(image.Pt(2, 3))
		logical := image.NewRGBA(logicalBounds)
		for y := 0; y < logical.Bounds().Dy(); y++ {
			for x := 0; x < logical.Bounds().Dx(); x++ {
				logical.SetRGBA(logicalBounds.Min.X+x, logicalBounds.Min.Y+y, color.RGBA{R: uint8(x + 1), G: uint8(y + 2), B: 0x63, A: 0xff})
			}
		}
		stride := width*4 + 8
		dst := make([]byte, stride*height)
		for i := range dst {
			dst[i] = 0xee
		}
		scratch := make([]byte, len(dst))
		row := make([]byte, width*4)
		if err := CopyToXRGBRotatedInto(dst, stride, logical, native, r, scratch, row); err != nil {
			t.Fatal(err)
		}
		for y := 0; y < logical.Bounds().Dy(); y++ {
			for x := 0; x < logical.Bounds().Dx(); x++ {
				px, py := rotation.Point(native, r, x, y)
				c := logical.RGBAAt(logicalBounds.Min.X+x, logicalBounds.Min.Y+y)
				got := dst[py*stride+px*4 : py*stride+px*4+4]
				var want [4]byte
				binary.NativeEndian.PutUint32(want[:], uint32(c.R)<<16|uint32(c.G)<<8|uint32(c.B))
				if string(got) != string(want[:]) {
					t.Fatalf("rotation %v pixel (%d,%d) = % x; want % x", r, x, y, got, want)
				}
			}
		}
		for y := 0; y < height; y++ {
			for _, got := range dst[y*stride+width*4 : (y+1)*stride] {
				if got != 0xee {
					t.Fatalf("rotation %v changed row %d padding to %#x", r, y, got)
				}
			}
		}
	}
}

func TestCopyToXRGBRotatedLargeFused(t *testing.T) {
	const width, height = 800, 480
	native := image.Rect(0, 0, width, height)
	for _, r := range []display.Rotation{display.Rotate90, display.Rotate180, display.Rotate270} {
		t.Run(r.String(), func(t *testing.T) {
			bounds := rotation.Bounds(native, r)
			src := image.NewRGBA(bounds)
			for y := 0; y < bounds.Dy(); y++ {
				row := src.Pix[y*src.Stride:]
				for x := 0; x < bounds.Dx(); x++ {
					i := x * 4
					row[i], row[i+1], row[i+2], row[i+3] = uint8(x), uint8(y), uint8(x^y), 0xff
				}
			}
			stride := width*4 + 16
			got := bytes.Repeat([]byte{0xee}, stride*height)
			ref := bytes.Repeat([]byte{0xee}, stride*height)
			scratch := make([]byte, width*height*4)
			row := make([]byte, width*4)
			if err := CopyToXRGBRotatedInto(got, stride, src, native, r, scratch, row); err != nil {
				t.Fatal(err)
			}
			logicalStride := bounds.Dx() * 4
			if err := CopyToXRGB(scratch, logicalStride, src); err != nil {
				t.Fatal(err)
			}
			switch r {
			case display.Rotate90:
				rotateXRGB90Go(ref, stride, scratch, logicalStride, width, height)
			case display.Rotate180:
				rotateXRGB180Go(ref, stride, scratch, logicalStride, width, height)
			default:
				rotateXRGB270Go(ref, stride, scratch, logicalStride, width, height)
			}
			if !bytes.Equal(got, ref) {
				for i := range got {
					if got[i] != ref[i] {
						t.Fatalf("byte %d = %#x; want %#x", i, got[i], ref[i])
					}
				}
			}
		})
	}
}

func TestSIMDRotationHandlersDoNotAllocateAfterWarmup(t *testing.T) {
	if runtime.GOARCH != "arm64" && runtime.GOARCH != "amd64" {
		t.Skip("SIMD rotation is only enabled on arm64 and amd64")
	}
	const width, height = 800, 480
	dst := make([]byte, width*height*4)
	packed := make([]byte, width*height*4)
	for _, r := range []display.Rotation{display.Rotate90, display.Rotate180, display.Rotate270} {
		logical := image.NewRGBA(rotation.Bounds(image.Rect(0, 0, width, height), r))
		scratch := make([]byte, width*height*4)
		row := make([]byte, width*4)
		t.Run(r.String(), func(t *testing.T) {
			runConvenience := func() {
				if err := CopyToXRGBRotated(dst, width*4, logical, image.Rect(0, 0, width, height), r); err != nil {
					panic(err)
				}
			}
			runPacked := func() {
				switch r {
				case display.Rotate90:
					rotateXRGB90(dst, width*4, packed, width*4, width, height)
				case display.Rotate180:
					rotateXRGB180(dst, width*4, packed, width*4, width, height)
				case display.Rotate270:
					rotateXRGB270(dst, width*4, packed, width*4, width, height)
				}
			}
			if err := CopyToXRGBRotatedInto(dst, width*4, logical, image.Rect(0, 0, width, height), r, scratch, row); err != nil {
				t.Fatal(err) // Warm any reusable workers before measuring.
			}
			if got := testing.AllocsPerRun(100, func() {
				if err := CopyToXRGBRotatedInto(dst, width*4, logical, image.Rect(0, 0, width, height), r, scratch, row); err != nil {
					panic(err)
				}
			}); got != 0 {
				t.Fatalf("steady-state fused rotation allocations = %v, want 0", got)
			}
			if got := testing.AllocsPerRun(100, runPacked); got != 0 {
				t.Fatalf("steady-state packed rotation allocations = %v, want 0", got)
			}
			runConvenience()
			if got := testing.AllocsPerRun(100, runConvenience); got != 0 {
				t.Fatalf("steady-state convenience rotation allocations = %v, want 0", got)
			}
		})
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
		if r != display.NoRotation {
			b.Run(r.String()+"-Convenience", func(b *testing.B) {
				b.SetBytes(int64(width * height * 4))
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if err := CopyToXRGBRotated(dst, stride, src, native, r); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
		if r == display.Rotate90 || r == display.Rotate180 || r == display.Rotate270 {
			logicalStride := src.Bounds().Dx() * 4
			b.Run(r.String()+"-PackedXRGB", func(b *testing.B) {
				b.SetBytes(int64(width * height * 4))
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					switch r {
					case display.Rotate90:
						rotateXRGB90(dst, stride, scratch, logicalStride, width, height)
					case display.Rotate180:
						rotateXRGB180(dst, stride, scratch, logicalStride, width, height)
					case display.Rotate270:
						rotateXRGB270(dst, stride, scratch, logicalStride, width, height)
					}
				}
			})
			b.Run(r.String()+"-Go", func(b *testing.B) {
				b.SetBytes(int64(width * height * 4))
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if err := CopyToXRGB(scratch, logicalStride, src); err != nil {
						b.Fatal(err)
					}
					switch r {
					case display.Rotate90:
						rotateXRGB90Go(dst, stride, scratch, logicalStride, width, height)
					case display.Rotate180:
						rotateXRGB180Go(dst, stride, scratch, logicalStride, width, height)
					case display.Rotate270:
						rotateXRGB270Go(dst, stride, scratch, logicalStride, width, height)
					}
				}
			})
		}
	}
}
