package draw

import (
	"image"
	"image/color"
	stdDraw "image/draw"
	"testing"

	"github.com/BeatGlow/display/pixel"
)

func TestDrawConvertsBetweenAllPixelFormats(t *testing.T) {
	const width, height = 8, 6
	formats := []struct {
		name string
		new  func() image.Image
		dst  func() stdDraw.Image
	}{
		{
			name: "mono-lsb",
			new:  func() image.Image { return pixel.NewMonoImage(width, height) },
			dst:  func() stdDraw.Image { return pixel.NewMonoImage(width, height) },
		},
		{
			name: "mono-vertical-lsb",
			new:  func() image.Image { return pixel.NewMonoVerticalLSBImage(width, height) },
			dst:  func() stdDraw.Image { return pixel.NewMonoVerticalLSBImage(width, height) },
		},
		{
			name: "gray2",
			new:  func() image.Image { return pixel.NewGray2Image(width, height) },
			dst:  func() stdDraw.Image { return pixel.NewGray2Image(width, height) },
		},
		{
			name: "gray4",
			new:  func() image.Image { return pixel.NewGray4Image(width, height) },
			dst:  func() stdDraw.Image { return pixel.NewGray4Image(width, height) },
		},
		{
			name: "cbgr15",
			new:  func() image.Image { return pixel.NewCBGR15Image(width, height) },
			dst:  func() stdDraw.Image { return pixel.NewCBGR15Image(width, height) },
		},
		{
			name: "cbgr16",
			new:  func() image.Image { return pixel.NewCBGR16Image(width, height) },
			dst:  func() stdDraw.Image { return pixel.NewCBGR16Image(width, height) },
		},
		{
			name: "crgb15",
			new:  func() image.Image { return pixel.NewCRGB15Image(width, height) },
			dst:  func() stdDraw.Image { return pixel.NewCRGB15Image(width, height) },
		},
		{
			name: "crgb16",
			new:  func() image.Image { return pixel.NewCRGB16Image(width, height) },
			dst:  func() stdDraw.Image { return pixel.NewCRGB16Image(width, height) },
		},
		{
			name: "yu12",
			new:  func() image.Image { return pixel.NewYU12Image(width, height) },
			dst:  func() stdDraw.Image { return pixel.NewYU12Image(width, height) },
		},
	}
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

	for _, srcFormat := range formats {
		src := srcFormat.new().(stdDraw.Image)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				index := y*width + x
				if srcFormat.name == "yu12" {
					// Keep each 2x2 block uniform so the source has well-defined
					// shared chroma samples.
					index = (y/2)*(width/2) + x/2
				}
				src.Set(x, y, palette[index%len(palette)])
			}
		}

		for _, dstFormat := range formats {
			t.Run(srcFormat.name+"-to-"+dstFormat.name, func(t *testing.T) {
				got, want := dstFormat.dst(), dstFormat.dst()
				Draw(got, got.Bounds(), src, src.Bounds().Min, Src)
				stdDraw.Draw(want, want.Bounds(), src, src.Bounds().Min, stdDraw.Src)

				for y := 0; y < height; y++ {
					for x := 0; x < width; x++ {
						gr, gg, gb, ga := got.At(x, y).RGBA()
						wr, wg, wb, wa := want.At(x, y).RGBA()
						if gr != wr || gg != wg || gb != wb || ga != wa {
							t.Fatalf("pixel (%d,%d): Draw = (%d,%d,%d,%d), standard draw = (%d,%d,%d,%d)",
								x, y, gr, gg, gb, ga, wr, wg, wb, wa)
						}
					}
				}
			})
		}
	}
}
