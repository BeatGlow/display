package display

import (
	"image"
	"image/color"
	"testing"

	"periph.io/x/conn/v3/gpio"
)

func TestConnectionConfigValidationWithoutHardware(t *testing.T) {
	if _, err := OpenUART(nil); err == nil {
		t.Fatal("OpenUART(nil) succeeded")
	}
	for _, baud := range []int{0, -1, 115201} {
		if _, err := OpenUART(&UARTConfig{Baud: baud}); err == nil {
			t.Errorf("OpenUART baud %d succeeded", baud)
		}
	}
	if _, err := OpenSPI(&SPIConfig{}); err != ErrResetPin {
		t.Fatalf("OpenSPI without reset pin error = %v, want %v", err, ErrResetPin)
	}
	if got := (InvalidPin{Name: "DC"}).Error(); got != "display: DC GPIO pin is invalid" {
		t.Errorf("InvalidPin.Error() = %q", got)
	}
	if isValidPin(nil) || isValidPin(gpio.INVALID) {
		t.Fatal("nil or INVALID GPIO pin reported valid")
	}
}

func TestBaseDisplayFillAndClear(t *testing.T) {
	img := image.NewRGBA(image.Rect(2, 3, 5, 5))
	d := &baseDisplay{Image: img}
	d.Fill(color.RGBA{R: 12, G: 34, B: 56, A: 255})
	for y := 3; y < 5; y++ {
		for x := 2; x < 5; x++ {
			if got := img.RGBAAt(x, y); got != (color.RGBA{R: 12, G: 34, B: 56, A: 255}) {
				t.Fatalf("pixel (%d,%d) after Fill = %v", x, y, got)
			}
		}
	}
	d.Clear()
	for y := 3; y < 5; y++ {
		for x := 2; x < 5; x++ {
			if got := img.RGBAAt(x, y); got != (color.RGBA{A: 255}) {
				t.Fatalf("pixel (%d,%d) after Clear = %v, want opaque black", x, y, got)
			}
		}
	}
}
