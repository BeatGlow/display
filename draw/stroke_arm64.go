//go:build arm64 && !purego && !race

package draw

import "image/color"

//go:noescape
func probeStroke(pix []byte, majorStep, minorStep, count, majorOffset int, minorStart, dx, dy, minorMin, minorMax, reach float64, ink color.RGBA)
