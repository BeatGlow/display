package pixel

import (
	"image/color"
	"testing"
)

var convertedPixelValue uint16

func BenchmarkPixelColorConversion(b *testing.B) {
	sources := []struct {
		name  string
		color color.Color
	}{
		{"RGBA", color.RGBA{R: 219, G: 121, B: 63, A: 255}},
		{"NRGBA", color.NRGBA{R: 219, G: 121, B: 63, A: 173}},
		{"Gray", color.Gray{Y: 137}},
	}
	converters := []struct {
		name     string
		typed    func(color.Color) uint16
		modelAPI func(color.Color) uint16
	}{
		{"Mono", func(c color.Color) uint16 {
			if monoModel(c).(Mono).On {
				return 1
			}
			return 0
		}, func(c color.Color) uint16 {
			if MonoModel.Convert(c).(Mono).On {
				return 1
			}
			return 0
		}},
		{"Gray2", func(c color.Color) uint16 { return uint16(gray2Model(c).(Gray2).Y) }, func(c color.Color) uint16 { return uint16(Gray2Model.Convert(c).(Gray2).Y) }},
		{"Gray4", func(c color.Color) uint16 { return uint16(gray4Model(c).(Gray4).Y) }, func(c color.Color) uint16 { return uint16(Gray4Model.Convert(c).(Gray4).Y) }},
		{"CBGR15", func(c color.Color) uint16 { return cbgr15Value(c).V }, func(c color.Color) uint16 { return CBGR15Model.Convert(c).(CBGR15).V }},
		{"CBGR16", func(c color.Color) uint16 { return cbgr16Value(c).V }, func(c color.Color) uint16 { return CBGR16Model.Convert(c).(CBGR16).V }},
		{"CRGB15", func(c color.Color) uint16 { return crgb15Value(c).V }, func(c color.Color) uint16 { return CRGB15Model.Convert(c).(CRGB15).V }},
		{"CRGB16", func(c color.Color) uint16 { return crgb16Value(c).V }, func(c color.Color) uint16 { return CRGB16Model.Convert(c).(CRGB16).V }},
	}
	for _, source := range sources {
		for _, converter := range converters {
			for _, path := range []struct {
				name string
				fn   func(color.Color) uint16
			}{{"Typed", converter.typed}, {"ColorModel", converter.modelAPI}} {
				b.Run(converter.name+"/"+source.name+"/"+path.name, func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						convertedPixelValue = path.fn(source.color)
					}
				})
			}
		}
	}
}
