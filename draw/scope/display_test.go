package scope_test

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"testing"

	"github.com/BeatGlow/display"
	"github.com/BeatGlow/display/pixel"
)

type memoryDisplay struct {
	draw.Image
	refreshes int
	failure   error
}

func (d *memoryDisplay) String() string                     { return "memory display" }
func (d *memoryDisplay) Close() error                       { return nil }
func (d *memoryDisplay) Clear()                             {}
func (d *memoryDisplay) Fill(color.Color)                   {}
func (d *memoryDisplay) Show(bool) error                    { return nil }
func (d *memoryDisplay) SetContrast(uint8) error            { return nil }
func (d *memoryDisplay) SetRotation(display.Rotation) error { return nil }
func (d *memoryDisplay) Refresh() error                     { d.refreshes++; return d.failure }

func TestDisplayPresentationUsesLuminanceAndPropagatesTransferFailure(t *testing.T) {
	transferFailure := errors.New("transfer failed")
	dst := &memoryDisplay{Image: pixel.NewGray4Image(128, 64), failure: transferFailure}
	r := compiledRenderer(t, dst.Bounds())
	var renderErr error
	allocs := testing.AllocsPerRun(10, func() {
		r.UpdatePhysics(frameTime)
		renderErr = r.RenderTo(dst)
	})
	if renderErr != nil || allocs != 0 {
		t.Fatalf("render error %v, allocations %g", renderErr, allocs)
	}
	if dst.refreshes != 0 {
		t.Fatal("RenderTo transferred before caller presentation")
	}
	if err := r.BlitToDisplay(dst); !errors.Is(err, transferFailure) {
		t.Fatalf("lost transfer error: %v", err)
	}
	if dst.refreshes != 1 {
		t.Fatal("completed display buffer was not refreshed")
	}
	lit := false
	for y := 0; y < 64; y++ {
		for x := 0; x < 128; x++ {
			red, green, blue, _ := dst.At(x, y).RGBA()
			if red != green || red != blue {
				t.Fatal("VFD grayscale received a color tint")
			}
			lit = lit || green > 0x8000
		}
	}
	if !lit {
		t.Fatal("grayscale display lost its trace")
	}
	if err := r.RenderTo(image.NewRGBA(image.Rect(0, 0, 64, 64))); err == nil {
		t.Fatal("invalid bounds accepted")
	}
}
