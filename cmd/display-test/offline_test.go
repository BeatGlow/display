package main

import (
	"image"
	"image/color"
	"testing"
)

func TestBrightnessPreviewAppliesGlobalScale(t *testing.T) {
	preview := brightnessPreview{
		Image: image.NewUniform(color.White),
		scale: 128,
	}
	got := color.RGBAModel.Convert(preview.At(0, 0)).(color.RGBA)
	want := color.RGBA{R: 128, G: 128, B: 128, A: 255}
	if got != want {
		t.Fatalf("brightness preview pixel = %#v, want %#v", got, want)
	}
}
