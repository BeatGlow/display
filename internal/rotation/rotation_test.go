package rotation

import (
	"image"
	"image/color"
	"testing"

	"github.com/BeatGlow/display"
)

func TestPoint(t *testing.T) {
	bounds := image.Rect(0, 0, 4, 3)
	tests := []struct {
		rotation display.Rotation
		logical  image.Point
		physical image.Point
	}{
		{display.NoRotation, image.Pt(0, 0), image.Pt(0, 0)},
		{display.Rotate90, image.Pt(0, 0), image.Pt(0, 2)},
		{display.Rotate90, image.Pt(2, 3), image.Pt(3, 0)},
		{display.Rotate180, image.Pt(0, 0), image.Pt(3, 2)},
		{display.Rotate270, image.Pt(0, 0), image.Pt(3, 0)},
		{display.Rotate270, image.Pt(2, 3), image.Pt(0, 2)},
	}
	for _, tt := range tests {
		x, y := Point(bounds, tt.rotation, tt.logical.X, tt.logical.Y)
		if got := image.Pt(x, y); got != tt.physical {
			t.Errorf("Point(%v, %v) = %v; want %v", tt.rotation, tt.logical, got, tt.physical)
		}
		px, py := InversePoint(bounds, tt.rotation, tt.physical.X, tt.physical.Y)
		if got := image.Pt(px, py); got != tt.logical {
			t.Errorf("InversePoint(%v, %v) = %v; want %v", tt.rotation, tt.physical, got, tt.logical)
		}
	}
}

func BenchmarkSet(b *testing.B) {
	const width, height = 800, 480
	bounds := image.Rect(0, 0, width, height)
	img := image.NewRGBA(bounds)
	c := color.RGBA{R: 0x87, G: 0x43, B: 0x21, A: 0xff}
	for _, rotation := range []display.Rotation{display.NoRotation, display.Rotate90, display.Rotate180, display.Rotate270} {
		b.Run(rotation.String(), func(b *testing.B) {
			logical := Bounds(bounds, rotation)
			b.ReportAllocs()
			b.SetBytes(4)
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				x, y := n%logical.Dx(), (n/logical.Dx())%logical.Dy()
				if rotation == display.NoRotation {
					img.SetRGBA(x, y, c)
				} else {
					px, py := Point(bounds, rotation, x, y)
					img.SetRGBA(px, py, c)
				}
			}
		})
	}
}
