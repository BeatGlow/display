package jpegn_test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"math/rand/v2"
	"testing"

	"github.com/BeatGlow/display/streamdeck/internal/jpegn"
)

type tileCase struct {
	width, height int
	rotate        bool
}

var deckTiles = []tileCase{
	{width: 72, height: 72, rotate: true},
	{width: 96, height: 96, rotate: true},
	{width: 120, height: 120},
	{width: 800, height: 100},
}

func BenchmarkTiles(b *testing.B) {
	for _, tile := range deckTiles {
		for _, dense := range []bool{false, true} {
			img := tilePixels(tile.width, tile.height, dense, 0)
			style := "controls"
			if dense {
				style = "dense"
			}
			var reference image.Image = img
			if tile.rotate {
				reference = rotatedView{img}
			}

			for _, quality := range []int{75, 100} {
				codecs := []struct {
					name string
					run  func(io.Writer) error
				}{
					{"stdlib", func(w io.Writer) error {
						return jpeg.Encode(w, reference, &jpeg.Options{Quality: quality})
					}},
					{"fork", func(w io.Writer) error {
						return jpegn.Encode(w, img, jpegn.Options{
							Quality: quality, Rotate180: tile.rotate, OptimizeCoding: true,
						})
					}},
				}
				for _, codec := range codecs {
					name := fmt.Sprintf("%dx%d/%s/Q%d/%s", tile.width, tile.height, style, quality, codec.name)
					b.Run(name, func(b *testing.B) {
						var sample bytes.Buffer
						if err := codec.run(&sample); err != nil {
							b.Fatal(err)
						}
						b.ReportAllocs()
						for b.Loop() {
							if err := codec.run(io.Discard); err != nil {
								b.Fatal(err)
							}
						}
						b.ReportMetric(float64(sample.Len()), "jpeg-B/op")
						b.ReportMetric(float64((sample.Len()+1015)/1016), "reports/op")
					})
				}
			}
		}
	}
}

type rotatedView struct{ image.Image }

func (v rotatedView) At(x, y int) color.Color {
	b := v.Bounds()
	return v.Image.At(b.Max.X-1-(x-b.Min.X), b.Max.Y-1-(y-b.Min.Y))
}

func tilePixels(width, height int, dense bool, variant int) *image.RGBA {
	img := image.NewRGBA(image.Rect(11, 13, width+11, height+13))
	random := rand.New(rand.NewPCG(uint64(4592+variant), uint64(3183+variant)))
	for y := range height {
		for x := range width {
			pixel := color.RGBA{A: 255}
			if dense {
				n := random.Uint32()
				pixel.R, pixel.G, pixel.B = byte(n), byte(n>>8), byte(n>>16)
			} else {
				pixel.R = byte(12 + 24*x/width)
				pixel.G = byte(22 + 34*y/height)
				pixel.B = byte(34 + (x+y)*48/(width+height))
				switch {
				case y >= height*3/4 && y <= height*7/8 && x > width/20 && x < width*(6+variant)/10:
					pixel.R, pixel.G, pixel.B = 40, 210, 167
				case y < height/5 && x > width/7 && x < width*3/4 && (x/5+y/5+variant)%3 != 0:
					pixel.R, pixel.G, pixel.B = 227, 233, 242
				case y > height/4 && y < height*2/3 && (x/9+variant)%11 < (y/8)%9:
					pixel.R, pixel.G, pixel.B = 60, 99, 203
				}
			}
			img.SetRGBA(x+11, y+13, pixel)
		}
	}
	return img
}
