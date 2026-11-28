package jpegn_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"reflect"
	"testing"

	"github.com/BeatGlow/display/streamdeck/internal/jpegn"
)

func TestOptimizedCodingPreservesDecodedPixels(t *testing.T) {
	for _, size := range []image.Point{image.Pt(72, 72), image.Pt(96, 96), image.Pt(120, 120), image.Pt(800, 100)} {
		img := image.NewRGBA(image.Rectangle{Max: size})
		for y := range size.Y {
			for x := range size.X {
				img.SetRGBA(x, y, color.RGBA{
					R: byte(x*x + y),
					G: byte(x + y*y),
					B: byte(x * y),
					A: 255,
				})
			}
		}
		plain, err := jpeg.Decode(bytes.NewReader(encode(t, img, jpegn.Options{})))
		if err != nil {
			t.Fatal(err)
		}
		optimized, err := jpeg.Decode(bytes.NewReader(encode(t, img, jpegn.Options{OptimizeCoding: true})))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(plain, optimized) {
			t.Fatalf("optimized JPEG changed decoded pixels at %v", size)
		}
	}
}

func TestRotatedCropPreservesPremultipliedColors(t *testing.T) {
	source := image.NewNRGBA(image.Rect(5, 11, 27, 31))
	img := source.SubImage(image.Rect(7, 13, 22, 24)).(*image.NRGBA)
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				R: byte(x * 21), G: byte(y * 17), B: byte(x * y), A: byte(x*9 + y*13),
			})
		}
	}

	// Render the expected orientation through image.Image's premultiplied colors.
	reference := image.NewRGBA(image.Rectangle{Max: b.Size()})
	for y := range b.Dy() {
		for x := range b.Dx() {
			reference.Set(x, y, img.At(b.Max.X-1-x, b.Max.Y-1-y))
		}
	}
	got := encode(t, img, jpegn.Options{Rotate180: true, OptimizeCoding: true})
	want := encode(t, reference, jpegn.Options{OptimizeCoding: true})
	if !bytes.Equal(got, want) {
		t.Fatal("the rotated crop differs from the reference in JPEG output")
	}
}

func encode(t *testing.T, img image.Image, opts jpegn.Options) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpegn.Encode(&buf, img, opts); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
