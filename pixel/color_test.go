package pixel

import (
	"image/color"
	"testing"
)

func TestMono(t *testing.T) {
	for y := 0; y < 2; y++ {
		t.Run("", func(it *testing.T) {
			c := Off
			if y > 0 {
				c = On
			}
			r, g, b, _ := c.RGBA()
			y *= 0xF
			want := uint32(y | y<<4 | y<<8 | y<<12)
			if r != want {
				t.Errorf("expected red to be %#04x, got %#04x", want, r)
			}
			if g != want {
				t.Errorf("expected green to be %#04x, got %#04x", want, g)
			}
			if b != want {
				t.Errorf("expected blue to be %#04x, got %#04x", want, b)
			}
		})
	}
}

func TestGray2(t *testing.T) {
	for y := 0; y < 4; y++ {
		t.Run("", func(it *testing.T) {
			c := Gray2{Y: uint8(y)}
			r, g, b, _ := c.RGBA()
			y *= 4
			want := uint32(y | y<<4 | y<<8 | y<<12)
			if r != want {
				t.Errorf("expected red to be %#04x, got %#04x", want, r)
			}
			if g != want {
				t.Errorf("expected green to be %#04x, got %#04x", want, g)
			}
			if b != want {
				t.Errorf("expected blue to be %#04x, got %#04x", want, b)
			}
		})
	}
}

func TestGray4(t *testing.T) {
	for y := range 16 {
		t.Run("", func(it *testing.T) {
			c := Gray4{Y: uint8(y)}
			r, g, b, _ := c.RGBA()
			want := uint32(y | y<<4 | y<<8 | y<<12)
			if r != want {
				t.Errorf("expected red to be %#04x, got %#04x", want, r)
			}
			if g != want {
				t.Errorf("expected green to be %#04x, got %#04x", want, g)
			}
			if b != want {
				t.Errorf("expected blue to be %#04x, got %#04x", want, b)
			}
		})
	}
}

func TestGrayModelsConvertColorChannels(t *testing.T) {
	tests := []struct {
		name  string
		in    color.Color
		gray2 uint8
		gray4 uint8
	}{
		{"black", color.RGBA{A: 255}, 0, 0},
		{"red", color.RGBA{R: 255, A: 255}, 2, 8},
		{"green", color.RGBA{G: 255, A: 255}, 1, 4},
		{"blue", color.RGBA{B: 255, A: 255}, 0, 2},
		{"white", color.RGBA{R: 255, G: 255, B: 255, A: 255}, 3, 15},
		{"mid-gray", color.Gray{Y: 128}, 2, 8},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Gray2Model.Convert(tc.in).(Gray2).Y; got != tc.gray2 {
				t.Errorf("Gray2 conversion got %d, want %d", got, tc.gray2)
			}
			if got := Gray4Model.Convert(tc.in).(Gray4).Y; got != tc.gray4 {
				t.Errorf("Gray4 conversion got %d, want %d", got, tc.gray4)
			}
		})
	}
}

func TestGrayModelsQuantizeAllGrayLevels(t *testing.T) {
	for value := 0; value <= 255; value++ {
		input := color.Gray{Y: uint8(value)}
		if got, want := Gray2Model.Convert(input).(Gray2).Y, uint8(value/64); got != want {
			t.Errorf("Gray2 conversion for %d = %d, want %d", value, got, want)
		}
		if got, want := Gray4Model.Convert(input).(Gray4).Y, uint8(value/16); got != want {
			t.Errorf("Gray4 conversion for %d = %d, want %d", value, got, want)
		}
	}
}
