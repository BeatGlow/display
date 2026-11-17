package pixel

import (
	"image"
	"image/color"
	"testing"
)

func TestColorRoundRobinRoundTripsAllPixelFormats(t *testing.T) {
	const width, height, rounds = 18, 8, 4
	colors := []color.RGBA{
		{R: 0, G: 0, B: 0, A: 255},
		{R: 255, G: 255, B: 255, A: 255},
		{R: 255, G: 0, B: 0, A: 255},
		{R: 0, G: 255, B: 0, A: 255},
		{R: 0, G: 0, B: 255, A: 255},
		{R: 0, G: 255, B: 255, A: 255},
		{R: 255, G: 0, B: 255, A: 255},
		{R: 255, G: 255, B: 0, A: 255},
	}
	gray2 := []color.RGBA{
		{R: 0, G: 0, B: 0, A: 255},
		{R: 85, G: 85, B: 85, A: 255},
		{R: 170, G: 170, B: 170, A: 255},
		{R: 255, G: 255, B: 255, A: 255},
	}
	gray4 := make([]color.RGBA, 16)
	for i := range gray4 {
		v := uint8(i * 17)
		gray4[i] = color.RGBA{R: v, G: v, B: v, A: 255}
	}
	mono := []color.RGBA{colors[0], colors[1]}

	tests := []struct {
		name    string
		model   color.Model
		palette []color.RGBA
		new     func() Image
		write   func(Image, *image.RGBA) error
		block   bool
	}{
		{name: "mono-lsb", model: MonoModel, palette: mono, new: func() Image { return NewMonoImage(width, height) }},
		{name: "mono-vertical-lsb", model: MonoModel, palette: mono, new: func() Image { return NewMonoVerticalLSBImage(width, height) }},
		{name: "gray2", model: Gray2Model, palette: gray2, new: func() Image { return NewGray2Image(width, height) }},
		{name: "gray4", model: Gray4Model, palette: gray4, new: func() Image { return NewGray4Image(width, height) }},
		{name: "cbgr15", model: CBGR15Model, palette: colors, new: func() Image { return NewCBGR15Image(width, height) }},
		{name: "cbgr16", model: CBGR16Model, palette: colors, new: func() Image { return NewCBGR16Image(width, height) }},
		{name: "crgb15", model: CRGB15Model, palette: colors, new: func() Image { return NewCRGB15Image(width, height) }},
		{name: "crgb16", model: CRGB16Model, palette: colors, new: func() Image { return NewCRGB16Image(width, height) }},
		{
			name: "yu12", model: color.YCbCrModel, palette: colors, block: true,
			new:   func() Image { return NewYU12Image(width, height) },
			write: func(dst Image, src *image.RGBA) error { return dst.(*YU12Image).CopyRGBA(src) },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for round := range rounds {
				src := image.NewRGBA(image.Rect(0, 0, width, height))
				for y := range height {
					for x := range width {
						index := y*width + x
						if tc.block {
							index = (y/2)*(width/2) + x/2
						}
						src.SetRGBA(x, y, tc.palette[(index+round)%len(tc.palette)])
					}
				}

				got := tc.new()
				if tc.write != nil {
					if err := tc.write(got, src); err != nil {
						t.Fatalf("round %d: %v", round, err)
					}
				} else {
					for y := range height {
						for x := range width {
							got.Set(x, y, src.RGBAAt(x, y))
						}
					}
				}

				for y := range height {
					for x := range width {
						want := tc.model.Convert(src.RGBAAt(x, y))
						assertSameRGBA(t, round, x, y, got.At(x, y), want)
					}
				}
			}
		})
	}
}

func assertSameRGBA(t *testing.T, round, x, y int, got, want color.Color) {
	t.Helper()
	gr, gg, gb, ga := got.RGBA()
	wr, wg, wb, wa := want.RGBA()
	if gr != wr || gg != wg || gb != wb || ga != wa {
		t.Fatalf("round %d pixel (%d,%d) = (%d,%d,%d,%d), want (%d,%d,%d,%d)",
			round, x, y, gr, gg, gb, ga, wr, wg, wb, wa)
	}
}
