package main

import (
	"testing"

	"github.com/BeatGlow/display/pixel"
)

type frameFuzzCase struct {
	driver string
	new    func(int, int) pixel.Image
}

var frameFuzzCases = []frameFuzzCase{
	{driver: "ssd1306", new: func(w, h int) pixel.Image { return pixel.NewMonoVerticalLSBImage(w, h) }},
	{driver: "gp1278", new: func(w, h int) pixel.Image { return pixel.NewMonoVerticalLSBImage(w, h) }},
	{driver: "gp1294", new: func(w, h int) pixel.Image { return pixel.NewMonoVerticalLSBImage(w, h) }},
	{driver: "gp1247ai", new: func(w, h int) pixel.Image { return pixel.NewMonoVerticalLSBImage(w, h) }},
	{driver: "gu256x64d7000", new: func(w, h int) pixel.Image { return pixel.NewMonoVerticalLSBImage(w, h) }},
	{driver: "gu3000", new: func(w, h int) pixel.Image { return pixel.NewMonoVerticalLSBImage(w, h) }},
	{driver: "ssd1306", new: func(w, h int) pixel.Image { return pixel.NewGray2Image(w, h) }},
	{driver: "sh1122", new: func(w, h int) pixel.Image { return pixel.NewGray4Image(w, h) }},
	{driver: "st7735", new: func(w, h int) pixel.Image { return pixel.NewCRGB15Image(w, h) }},
	{driver: "st7789", new: func(w, h int) pixel.Image { return pixel.NewCRGB16Image(w, h) }},
}

func FuzzOfflineFrameCodecRoundTrip(f *testing.F) {
	for format := range frameFuzzCases {
		f.Add(uint8(format), uint8(1), uint8(1), []byte{0x00, 0x55, 0xaa, 0xff})
		f.Add(uint8(format), uint8(8), uint8(9), []byte{0x81, 0x18, 0x7e})
	}
	f.Add(uint8(6), uint8(5), uint8(7), []byte{0x00, 0x55, 0xaa, 0xff})
	f.Add(uint8(7), uint8(3), uint8(9), []byte{0x01, 0x23, 0x45, 0x67})

	f.Fuzz(func(t *testing.T, format, widthSeed, heightSeed uint8, data []byte) {
		codec := frameFuzzCases[int(format)%len(frameFuzzCases)]
		width, height := 1+int(widthSeed%64), 1+int(heightSeed%32)
		frame := codec.new(width, height)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				index := (y*width + x) * 2
				first, second := fuzzCodecByte(data, index), fuzzCodecByte(data, index+1)
				switch img := frame.(type) {
				case *pixel.MonoVerticalLSBImage:
					img.Set(x, y, pixel.Mono{On: first&1 != 0})
				case *pixel.Gray2Image:
					img.Set(x, y, pixel.Gray2{Y: first & 3})
				case *pixel.Gray4Image:
					img.Set(x, y, pixel.Gray4{Y: first & 0x0f})
				case *pixel.CRGB15Image:
					img.Set(x, y, pixel.CRGB15{V: (uint16(first)<<8 | uint16(second)) & 0x7fff})
				case *pixel.CRGB16Image:
					img.Set(x, y, pixel.CRGB16{V: uint16(first)<<8 | uint16(second)})
				default:
					t.Fatalf("unsupported frame type %T", frame)
				}
			}
		}

		encoded, err := encodeFrame(codec.driver, frame)
		if err != nil {
			t.Fatalf("encodeFrame(%s): %v", codec.driver, err)
		}
		decoded, err := decodeFrame(codec.driver, frame, encoded)
		if err != nil {
			t.Fatalf("decodeFrame(%s): %v", codec.driver, err)
		}
		if !sameImage(frame, decoded) {
			t.Fatalf("%s codec did not round-trip %dx%d frame", codec.driver, width, height)
		}
	})
}

func fuzzCodecByte(data []byte, index int) uint8 {
	if len(data) == 0 {
		return 0
	}
	return data[index%len(data)]
}
