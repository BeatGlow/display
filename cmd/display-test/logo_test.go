package main

import (
	"image"
	"image/color"
	"sort"
	"testing"
)

func TestLogoImageSelectsBeatGlowPaletteAndFitsDisplay(t *testing.T) {
	mono := logoImage(image.Pt(256, 50), true)
	if got, want := mono.Bounds().Size(), image.Pt(50, 50); got != want {
		t.Fatalf("monochrome logo size = %v, want %v", got, want)
	}
	if levels := grayLevels(mono); len(levels) != 2 || levels[0] != 0 || levels[1] != 255 {
		t.Fatalf("monochrome logo gray levels = %v, want [0 255]", levels)
	}

	gray4 := logoImage(image.Pt(256, 64), false)
	if got, want := gray4.Bounds().Size(), image.Pt(64, 64); got != want {
		t.Fatalf("gray4 logo size = %v, want %v", got, want)
	}
	levels := grayLevels(gray4)
	if len(levels) <= 2 || len(levels) > 16 {
		t.Fatalf("gray4 logo has %d levels, want 3 to 16: %v", len(levels), levels)
	}
	for _, level := range levels {
		if level%17 != 0 {
			t.Fatalf("gray4 logo level %d is not a 4-bit grayscale value", level)
		}
	}
}

func grayLevels(img image.Image) []uint8 {
	seen := make(map[uint8]bool)
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			seen[color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y] = true
		}
	}
	levels := make([]uint8, 0, len(seen))
	for value := range seen {
		levels = append(levels, value)
	}
	sort.Slice(levels, func(i, j int) bool { return levels[i] < levels[j] })
	return levels
}
