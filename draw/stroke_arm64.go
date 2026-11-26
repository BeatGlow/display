//go:build arm64 && !purego && !race

package draw

import "image/color"

const maxStrokeQuotientLength = 64

// For each length, store two rows in scan order. The reverse row retains the
// negative zero obtained when its final offset is divided by a negative dx.
var strokeQuotientTable = func() (table [maxStrokeQuotientLength * (maxStrokeQuotientLength + 3)]float64) {
	for length := 1; length <= maxStrokeQuotientLength; length++ {
		row := strokeQuotientRow(length)
		for offset := 0; offset <= length; offset++ {
			table[row+offset] = float64(offset) / float64(length)
			table[row+length+1+offset] = float64(offset-length) / -float64(length)
		}
	}
	return table
}()

func strokeQuotientRow(length int) int {
	// Every smaller length contributes two rows of (length + 1) entries.
	return (length - 1) * (length + 2)
}

func strokeQuotients(dx float64, majorOffset int) *float64 {
	if dx > maxStrokeQuotientLength || dx < -maxStrokeQuotientLength {
		return nil
	}
	if dx > 0 {
		return &strokeQuotientTable[strokeQuotientRow(int(dx))+majorOffset]
	}
	length := int(-dx)
	reverseRow := strokeQuotientRow(length) + length + 1
	return &strokeQuotientTable[reverseRow+length+majorOffset]
}

//go:noescape
func probeStroke(pix []byte, majorStep, minorStep, count, majorOffset int, minorStart, dx, dy, minorMin, minorMax, reach float64, ink color.RGBA, quotients *float64)
