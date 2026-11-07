package display

import (
	"image/color"
	"testing"

	"github.com/BeatGlow/display/pixel"
)

func BenchmarkDisplayFillCRGB16(b *testing.B) {
	for _, tc := range []struct {
		name string
		fill func(*pixel.CRGB16Image)
	}{
		{"Fill", func(i *pixel.CRGB16Image) { (&baseDisplay{Image: i}).Fill(color.White) }},
		{"PerPixelReference", func(i *pixel.CRGB16Image) {
			for y := 0; y < 128; y++ {
				for x := 0; x < 400; x++ {
					i.Set(x, y, color.White)
				}
			}
		}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			i := pixel.NewCRGB16Image(400, 128)
			b.SetBytes(400 * 128 * 2)
			b.ReportAllocs()
			for n := 0; n < b.N; n++ {
				tc.fill(i)
			}
		})
	}
}
