package pixel

import (
	"image"
	"image/color"
	"testing"
)

func TestYU12CopyRGBAConvertsSubsampledImage(t *testing.T) {
	const width, height = 19, 5
	src := image.NewRGBA(image.Rect(3, 7, 3+width, 7+height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			src.SetRGBA(3+x, 7+y, color.RGBA{R: uint8(x*17 + y), G: uint8(y*39 + x), B: uint8(x*7 + y*11), A: 255})
		}
	}
	got := NewYU12Image(width, height)
	if err := got.CopyRGBA(src); err != nil {
		t.Fatal(err)
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := src.RGBAAt(3+x, 7+y)
			want, _, _ := color.RGBToYCbCr(c.R, c.G, c.B)
			if actual := got.Pix[y*got.Stride+x]; actual != want {
				t.Fatalf("luma at (%d,%d) = %d, want %d", x, y, actual, want)
			}
		}
	}
	for y := 0; y < height; y += 2 {
		for x := 0; x < width; x += 2 {
			var r, g, b, n int
			for dy := 0; dy < 2 && y+dy < height; dy++ {
				for dx := 0; dx < 2 && x+dx < width; dx++ {
					c := src.RGBAAt(3+x+dx, 7+y+dy)
					r, g, b, n = r+int(c.R), g+int(c.G), b+int(c.B), n+1
				}
			}
			u, v := got.chromaOffsets(x, y)
			_, wantU, wantV := color.RGBToYCbCr(uint8((r+n/2)/n), uint8((g+n/2)/n), uint8((b+n/2)/n))
			if got.Pix[u] != wantU || got.Pix[v] != wantV {
				t.Fatalf("chroma at (%d,%d) = (%d,%d), want (%d,%d)", x, y, got.Pix[u], got.Pix[v], wantU, wantV)
			}
		}
	}
}

func BenchmarkYU12CopyRGBA(b *testing.B) {
	const width, height = 1280, 720
	src := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := range src.Pix {
		src.Pix[i] = byte(i*29 + 17)
	}
	optimized := NewYU12Image(width, height)
	scalar := NewYU12Image(width, height)
	b.Run("chunked", func(b *testing.B) {
		b.SetBytes(int64(width * height * 4))
		b.ReportAllocs()
		for range b.N {
			if err := optimized.CopyRGBA(src); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("scalar", func(b *testing.B) {
		b.SetBytes(int64(width * height * 4))
		b.ReportAllocs()
		for range b.N {
			copyRGBAtoYU12Scalar(scalar, src)
		}
	})
}

func copyRGBAtoYU12Scalar(dst *YU12Image, src *image.RGBA) {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	for y := 0; y < h; y++ {
		row := src.Pix[src.PixOffset(bounds.Min.X, bounds.Min.Y+y):]
		for x := 0; x < w; x++ {
			i := x * 4
			y8, _, _ := color.RGBToYCbCr(row[i], row[i+1], row[i+2])
			dst.Pix[y*dst.Stride+x] = y8
		}
	}
	for y := 0; y < h; y += 2 {
		for x := 0; x < w; x += 2 {
			var r, g, b, n int
			for dy := 0; dy < 2 && y+dy < h; dy++ {
				row := src.Pix[src.PixOffset(bounds.Min.X, bounds.Min.Y+y+dy):]
				for dx := 0; dx < 2 && x+dx < w; dx++ {
					i := (x + dx) * 4
					r, g, b, n = r+int(row[i]), g+int(row[i+1]), b+int(row[i+2]), n+1
				}
			}
			_, cb, cr := color.RGBToYCbCr(uint8((r+n/2)/n), uint8((g+n/2)/n), uint8((b+n/2)/n))
			u, v := dst.chromaOffsets(x, y)
			dst.Pix[u], dst.Pix[v] = cb, cr
		}
	}
}
