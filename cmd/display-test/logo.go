package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/png"
)

//go:embed assets/beatglow-mono.png
var beatGlowMonoLogo []byte

//go:embed assets/beatglow-gray4.png
var beatGlowGray4Logo []byte

func logoImage(size image.Point, monochrome bool) image.Image {
	logoData := beatGlowGray4Logo
	if monochrome {
		logoData = beatGlowMonoLogo
	}
	logo, err := png.Decode(bytes.NewReader(logoData))
	if err != nil {
		panic(err)
	}

	logoSize := min(logo.Bounds().Dx(), size.X, size.Y)
	if logoSize == logo.Bounds().Dx() {
		return logo
	}

	resized := image.NewGray(image.Rect(0, 0, logoSize, logoSize))
	for y := range logoSize {
		for x := range logoSize {
			sourceX := x * logo.Bounds().Dx() / logoSize
			sourceY := y * logo.Bounds().Dy() / logoSize
			resized.SetGray(x, y, color.GrayModel.Convert(logo.At(sourceX, sourceY)).(color.Gray))
		}
	}
	return resized
}
