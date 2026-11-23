package pixel_test

import (
	"image"
	"image/color"
	"reflect"
	"testing"

	"github.com/BeatGlow/display/pixel"
)

type packedImage interface {
	image.Image
	pixel.FastImage
}

func TestPackedWriterWritesGrayLevelsToPackedImages(t *testing.T) {
	tests := []struct {
		name   string
		new    func() packedImage
		values []uint8
		want   []color.Color
	}{
		{
			name:   "gray2",
			new:    func() packedImage { return pixel.NewGray2Image(4, 1) },
			values: []uint8{0, 64, 128, 255},
			want:   []color.Color{pixel.Gray2{Y: 0}, pixel.Gray2{Y: 1}, pixel.Gray2{Y: 2}, pixel.Gray2{Y: 3}},
		},
		{
			name:   "gray3level",
			new:    func() packedImage { return pixel.NewGray3LevelImage(4, 1) },
			values: []uint8{0, 64, 192, 255},
			want:   []color.Color{pixel.Gray3Level{Y: 0}, pixel.Gray3Level{Y: 1}, pixel.Gray3Level{Y: 2}, pixel.Gray3Level{Y: 2}},
		},
		{
			name:   "gray4",
			new:    func() packedImage { return pixel.NewGray4Image(4, 1) },
			values: []uint8{0, 64, 128, 255},
			want:   []color.Color{pixel.Gray4{Y: 0}, pixel.Gray4{Y: 4}, pixel.Gray4{Y: 8}, pixel.Gray4{Y: 15}},
		},
		{
			name:   "mono",
			new:    func() packedImage { return pixel.NewMonoImage(4, 1) },
			values: []uint8{0, 128, 255, 128},
			want:   []color.Color{pixel.Mono{}, pixel.Mono{}, pixel.Mono{On: true}, pixel.Mono{}},
		},
		{
			name:   "mono-vertical",
			new:    func() packedImage { return pixel.NewMonoVerticalLSBImage(4, 1) },
			values: []uint8{0, 128, 255, 128},
			want:   []color.Color{pixel.Mono{}, pixel.Mono{}, pixel.Mono{On: true}, pixel.Mono{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := tt.new()
			writer, ok := pixel.NewPackedWriter(img)
			if !ok {
				t.Fatal("NewPackedWriter rejected a supported packed image")
			}
			for x, value := range tt.values {
				writer.SetGray(x, 0, value)
			}
			for x, want := range tt.want {
				if got := img.At(x, 0); !reflect.DeepEqual(got, want) {
					t.Errorf("pixel at (%d, 0) = %#v, want %#v", x, got, want)
				}
			}
		})
	}

	if _, ok := pixel.NewPackedWriter(pixel.NewYU12Image(4, 2)); ok {
		t.Error("NewPackedWriter accepted subsampled YU12 storage")
	}

	for _, tt := range []struct {
		value uint8
		x, y  int
		want  bool
	}{
		{value: 7, x: 0, y: 0, want: false},
		{value: 8, x: 0, y: 0, want: true},
		{value: 135, x: 1, y: 0, want: false},
		{value: 136, x: 1, y: 0, want: true},
	} {
		if got := pixel.MonoDither(tt.value, tt.x, tt.y); got != tt.want {
			t.Errorf("MonoDither(%d, %d, %d) = %t, want %t", tt.value, tt.x, tt.y, got, tt.want)
		}
	}
}

func TestPackedWriterWritesRGBAUsingImageColorModels(t *testing.T) {
	c := color.RGBA{R: 240, G: 96, B: 32, A: 255}
	gray := color.GrayModel.Convert(c).(color.Gray).Y
	tests := []struct {
		name string
		new  func() packedImage
		want color.Color
	}{
		{
			name: "gray2",
			new:  func() packedImage { return pixel.NewGray2Image(2, 1) },
			want: pixel.Gray2{Y: gray >> 6},
		},
		{
			name: "cbgr15",
			new: func() packedImage {
				return pixel.NewCBGR15Image(2, 1)
			},
			want: pixel.CBGR15Model.Convert(c),
		},
		{
			name: "cbgr16",
			new: func() packedImage {
				return pixel.NewCBGR16Image(2, 1)
			},
			want: pixel.CBGR16Model.Convert(c),
		},
		{
			name: "crgb15",
			new: func() packedImage {
				return pixel.NewCRGB15Image(2, 1)
			},
			want: pixel.CRGB15Model.Convert(c),
		},
		{
			name: "crgb16",
			new: func() packedImage {
				return pixel.NewCRGB16Image(2, 1)
			},
			want: pixel.CRGB16Model.Convert(c),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := tt.new()
			writer, ok := pixel.NewPackedWriter(img)
			if !ok {
				t.Fatal("NewPackedWriter rejected a supported packed image")
			}
			writer.SetRGBA(1, 0, c)
			if got := img.At(1, 0); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("pixel at (1, 0) = %#v, want %#v", got, tt.want)
			}

			before := append([]byte(nil), img.PixelView().Pix...)
			writer.SetRGBA(2, 0, color.RGBA{R: 255, A: 255})
			if got := img.PixelView().Pix; !reflect.DeepEqual(got, before) {
				t.Fatalf("out-of-bounds write changed packed pixels: got %x, want %x", got, before)
			}
		})
	}

	if _, ok := pixel.NewPackedWriter(nil); ok {
		t.Error("NewPackedWriter accepted a nil image")
	}

	img := pixel.NewCRGB16Image(1, 1)
	writer, ok := pixel.NewPackedWriter(img)
	if !ok {
		t.Fatal("NewPackedWriter rejected a CRGB16 image")
	}
	if allocs := testing.AllocsPerRun(100, func() { writer.SetRGBA(0, 0, c) }); allocs != 0 {
		t.Errorf("SetRGBA allocated %.2f times per CRGB16 pixel, want 0", allocs)
	}
}
