package pixel

import "testing"

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
	for y := 0; y < 16; y++ {
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
