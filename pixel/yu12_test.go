package pixel

import (
	"image"
	"image/color"
	"testing"
)

func TestWrapYU12StridesOffsetsOddDimensions(t *testing.T) {
	backing := make([]byte, 8*3+4*2*2)
	img, err := WrapYU12(backing, 5, 3, 8, 4)
	if err != nil {
		t.Fatal(err)
	}
	planes := img.Planes()
	if len(planes.Y) != 24 || len(planes.U) != 8 || len(planes.V) != 8 || planes.ChromaWidth != 3 || planes.ChromaHeight != 2 {
		t.Fatalf("unexpected planes: %+v lengths %d/%d/%d", planes, len(planes.Y), len(planes.U), len(planes.V))
	}
	if &planes.Y[0] != &backing[0] || &planes.U[0] != &backing[24] || &planes.V[0] != &backing[32] {
		t.Fatal("plane offsets do not refer to wrapped storage")
	}
}

func TestWrapYU12ValidationAndOperations(t *testing.T) {
	for _, tc := range []struct{ w, h, ys, uv, n int }{{0, 2, 1, 1, 8}, {2, 2, 1, 1, 8}, {2, 2, 2, 0, 8}, {3, 3, 3, 2, 16}} {
		if _, err := WrapYU12(make([]byte, tc.n), tc.w, tc.h, tc.ys, tc.uv); err == nil {
			t.Fatalf("WrapYU12(%d,%d,%d,%d,%d) accepted", tc.w, tc.h, tc.ys, tc.uv, tc.n)
		}
	}
	backing := make([]byte, 6*3+3*2*2)
	img, err := WrapYU12(backing, 5, 3, 6, 3)
	if err != nil {
		t.Fatal(err)
	}
	img.Set(4, 2, color.RGBA{R: 255, A: 255})
	if img.At(4, 2) == color.Transparent {
		t.Fatal("Set did not update wrapped image")
	}
	img.Fill(color.RGBA{G: 255, A: 255})
	if &img.Pix[0] != &backing[0] {
		t.Fatal("Fill detached wrapped storage")
	}
	src := image.NewRGBA(image.Rect(0, 0, 5, 3))
	src.SetRGBA(1, 1, color.RGBA{B: 255, A: 255})
	if err := img.CopyRGBA(src); err != nil {
		t.Fatal(err)
	}
	if &img.Pix[0] != &backing[0] {
		t.Fatal("CopyRGBA detached wrapped storage")
	}
}

func TestWrapYU12NoCopy(t *testing.T) {
	backing := make([]byte, 1280*400*3/2)
	img, err := WrapYU12(backing, 1280, 400, 1280, 640)
	if err != nil {
		t.Fatal(err)
	}
	img.Pix[0] = 91
	if backing[0] != 91 {
		t.Fatal("wrapper copied backing bytes")
	}
}

func BenchmarkYU12WrappedFill400x1280(b *testing.B) {
	const width, height = 1280, 400
	backing := make([]byte, width*height*3/2)
	img, err := WrapYU12(backing, width, height, width, width/2)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(backing)))
	for range b.N {
		img.Fill(color.Black)
	}
}

func BenchmarkYU12StagingCopy400x1280(b *testing.B) {
	const width, height = 1280, 400
	src := NewYU12Image(width, height)
	mapped := make([]byte, len(src.Pix))
	b.ReportAllocs()
	b.SetBytes(int64(len(mapped)))
	for range b.N {
		copy(mapped, src.Pix)
	}
}

func BenchmarkYU12WrapVsStagingCopy400x1280(b *testing.B) {
	const width, height = 1280, 400
	const size = width * height * 3 / 2
	source := make([]byte, size)
	mapped := make([]byte, size)
	b.Run("wrap", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			if _, err := WrapYU12(mapped, width, height, width, width/2); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("staging-copy", func(b *testing.B) {
		b.SetBytes(size)
		b.ReportAllocs()
		for range b.N {
			copy(mapped, source)
		}
	})
}

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

func TestYU12CopyRGBAColorRoundRobinAllowsLossyRoundTrip(t *testing.T) {
	const width, height = 18, 8
	palette := []color.RGBA{
		{R: 0, G: 0, B: 0, A: 255},
		{R: 255, G: 255, B: 255, A: 255},
		{R: 255, G: 0, B: 0, A: 255},
		{R: 0, G: 255, B: 0, A: 255},
		{R: 0, G: 0, B: 255, A: 255},
		{R: 0, G: 255, B: 255, A: 255},
		{R: 255, G: 0, B: 255, A: 255},
		{R: 255, G: 255, B: 0, A: 255},
	}
	const tolerance = 2
	for round := range 4 {
		src := image.NewRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				block := (y/2)*(width/2) + x/2
				src.SetRGBA(x, y, palette[(block+round)%len(palette)])
			}
		}
		got := NewYU12Image(width, height)
		if err := got.CopyRGBA(src); err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				want := src.RGBAAt(x, y)
				r, g, b, _ := got.At(x, y).RGBA()
				for channel, pair := range [][2]uint8{
					{uint8(r >> 8), want.R},
					{uint8(g >> 8), want.G},
					{uint8(b >> 8), want.B},
				} {
					if delta := int(pair[0]) - int(pair[1]); delta < -tolerance || delta > tolerance {
						t.Fatalf("round %d pixel (%d,%d) channel %d differs by %d; want within ±%d", round, x, y, channel, delta, tolerance)
					}
				}
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
