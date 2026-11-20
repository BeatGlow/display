package display

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"testing"

	"github.com/BeatGlow/display/pixel"
)

func TestGU256X64D7000ProxySendsRotatedGrayscaleAndControls(t *testing.T) {
	port := &testProxyUSB{}
	d, err := newGU256X64D7000Proxy(proxyUSB{port}, port, &GU256X64D7000ProxyConfig{Config: Config{Rotation: Rotate90}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := d.Bounds(), image.Rect(0, 0, 64, 256); got != want {
		t.Fatalf("rotated bounds = %v, want %v", got, want)
	}
	if d.ColorModel() != pixel.Gray3LevelModel {
		t.Fatal("three-level display does not expose the three-level pixel model")
	}
	d.Set(63, 0, color.Gray{Y: 64})
	if got := d.At(63, 0); got != (pixel.Gray3Level{Y: 1}) {
		t.Fatalf("pixel = %v, want half brightness", got)
	}
	d.Set(62, 0, color.Gray{Y: 192})
	d.Set(0, 0, color.White)
	d.Set(0, 255, color.White)
	if err := d.Refresh(); err != nil {
		t.Fatal(err)
	}
	if err := d.SetContrast(255); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}

	if len(port.packets) != 5 {
		t.Fatalf("sent %d USB requests, want 5", len(port.packets))
	}
	// The golden packet includes the first and last columns and CRC-16/XMODEM.
	frame := make([]byte, 4102)
	copy(frame, []byte{0xff, 4, 3, 9})
	frame[3+15], frame[3+4095] = 128, 128
	copy(frame[4099:], []byte{20, 66, 2})
	for _, tc := range []struct {
		name string
		got  []byte
		want []byte
	}{
		{"grayscale frame", port.packets[2], frame},
		{"maximum brightness", port.packets[3], []byte{0xff, 3, 4, 8, 28, 41, 0}},
		{"power off", port.packets[4], []byte{0xff, 2, 5, 0, 21, 35, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !bytes.Equal(tc.got, tc.want) {
				index := 0
				for index < min(len(tc.got), len(tc.want)) && tc.got[index] == tc.want[index] {
					index++
				}
				t.Errorf("packet differs at byte %d (got %d bytes, want %d)", index, len(tc.got), len(tc.want))
			}
		})
	}
	if !port.closed {
		t.Error("closing the display did not release the USB port")
	}
	if err := d.Refresh(); !errors.Is(err, io.ErrClosedPipe) {
		t.Errorf("Refresh after Close = %v, want io.ErrClosedPipe", err)
	}
}

func TestGU256X64D7000ProxySendsDistinctMonochromeAndFourLevelFrames(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		config                  GU256X64D7000ProxyConfig
		size                    int
		op, first, second, last byte
		crc                     [3]byte
	}{
		{"monochrome", GU256X64D7000ProxyConfig{Config: Config{UseMono: true}}, 2048, 5, 255, 0, 1, [3]byte{27, 18, 1}},
		{"four levels", GU256X64D7000ProxyConfig{GrayLevels: 4}, 4096, 6, 228, 255, 0, [3]byte{116, 120, 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			port := &testProxyUSB{}
			d, err := newGU256X64D7000Proxy(proxyUSB{port}, port, &tc.config)
			if err != nil {
				t.Fatal(err)
			}
			if tc.config.UseMono {
				for y := range 8 {
					d.Set(0, y, color.White)
				}
				d.Set(255, 63, color.White)
			} else {
				for y, luma := range []uint8{0, 64, 128, 192, 255, 255, 255, 255} {
					d.Set(0, y, color.Gray{Y: luma})
				}
			}
			if err := d.Refresh(); err != nil {
				t.Fatal(err)
			}
			want := make([]byte, tc.size+6)
			copy(want, []byte{0xff, tc.op, 3, tc.first, tc.second})
			want[3+tc.size-1] = tc.last
			copy(want[3+tc.size:], tc.crc[:])
			got := port.packets[2]
			if !bytes.Equal(got, want) {
				t.Errorf("sent %d bytes with header and first pixels %x, want %d bytes starting %x", len(got), got[:min(5, len(got))], len(want), want[:5])
			}
			if err := d.Close(); err != nil {
				t.Error(err)
			}
		})
	}
}

type testProxyUSB struct {
	packets [][]byte
	reply   []byte
	closed  bool
}

func (p *testProxyUSB) ResetInputBuffer() error {
	p.reply = nil
	return nil
}

func (p *testProxyUSB) Write(packet []byte) (int, error) {
	p.packets = append(p.packets, bytes.Clone(packet))
	p.reply = []byte{0, packet[2]}
	return len(packet), nil
}

func (p *testProxyUSB) Read(reply []byte) (int, error) {
	n := copy(reply, p.reply)
	p.reply = p.reply[n:]
	return n, nil
}

func (p *testProxyUSB) Close() error {
	p.closed = true
	return nil
}

func TestGU256X64D7000ProxySPIReportsRejectionAndCanRefreshAgain(t *testing.T) {
	bus := &testProxySPI{}
	d, err := newGU256X64D7000Proxy(proxySPI{bus}, bus, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
	}()
	bus.rejectNext = true
	if err := d.SetContrast(128); !errors.Is(err, errProxyRejected) {
		t.Errorf("SetContrast = %v, want a proxy rejection", err)
	}
	bus.dropNext = true
	if err := d.Show(false); !errors.Is(err, ErrNotReady) {
		t.Errorf("Show with no acknowledgment = %v, want ErrNotReady", err)
	}
	if err := d.Refresh(); err != nil {
		t.Fatalf("Refresh following a missing acknowledgment: %v", err)
	}
}

type testProxySPI struct {
	reserved   bool
	ready      bool
	reply      byte
	remaining  int
	rejectNext bool
	dropNext   bool
}

func (b *testProxySPI) Tx(w, r []byte) error {
	switch {
	case len(r) != 0:
		if len(w) != 4 || len(r) != 4 || !bytes.Equal(w, []byte{0, 0, 0, 0}) {
			return fmt.Errorf("invalid SPI status poll: %x", w)
		}
		if b.reserved && !b.ready {
			b.ready = true
			r[3] = 0xfe
		} else {
			r[3] = b.reply
		}
	case bytes.Equal(w, []byte{0xff}):
		b.reserved, b.ready = true, false
	case b.ready:
		if b.remaining == 0 {
			switch w[0] {
			case 2, 3:
				b.remaining = 6
			case 4, 6:
				b.remaining = 4101
			case 5:
				b.remaining = 2053
			default:
				return fmt.Errorf("unknown frame opcode %d", w[0])
			}
			b.reply = w[1]
		}
		if len(w) != min(4096, b.remaining) {
			return fmt.Errorf("SPI chunk length %d with %d bytes left in request", len(w), b.remaining)
		}
		b.remaining -= len(w)
		if b.remaining != 0 {
			return nil
		}
		b.reserved, b.ready = false, false
		if b.rejectNext {
			b.reply |= 0x80
			b.rejectNext = false
		}
		if b.dropNext {
			b.reply = 0
			b.dropNext = false
		}
	default:
		return errors.New("SPI request arrived before the proxy signaled ready")
	}
	return nil
}

func (b *testProxySPI) Close() error { return nil }
