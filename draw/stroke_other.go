//go:build !arm64 || purego || race

package draw

import (
	"image/color"
	"math"
)

func strokeQuotients(float64, int) *float64 { return nil }

func probeStroke(pix []byte, majorStep, minorStep, count, majorOffset int, minorStart, dx, dy, minorMin, minorMax, reach float64, ink color.RGBA, _ *float64) {
	base := 0
	for i := 0; i < count; i++ {
		center := minorStart + dy*(float64(majorOffset+i)/dx)
		low := max(minorMin, math.Floor(center-reach)+1)
		high := min(minorMax, math.Ceil(center+reach))
		if low < high {
			for minor := int(low); minor < int(high); minor++ {
				offset := base + (minor-int(minorMin))*minorStep
				src := strokeCoverage(ink, reach-math.Abs(float64(minor)-center))
				if src.A != 0 {
					blendStrokeRGBA(pix[offset:offset+4], src)
				}
			}
		}
		base += majorStep
	}
}
