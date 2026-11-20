package display

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"testing"
)

func TestGU256X64D7000ProxySendsRotatedGrayscaleAndControls(t *testing.T) {
	port := &testProxyUSB{}
	d, err := newGU256X64D7000Proxy(proxyUSB{port}, port, &Config{Rotation: Rotate90})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := d.Bounds(), image.Rect(0, 0, 64, 256); got != want {
		t.Fatalf("rotated bounds = %v, want %v", got, want)
	}
	d.Set(63, 0, color.Gray{Y: 64})
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
	frame := make([]byte, 3334)
	copy(frame, []byte{0xff, 1, 3, 7})
	frame[3+12], frame[3+3327] = 54, 54
	copy(frame[3331:], []byte{35, 63, 1})
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
		b.reserved, b.ready = false, false
		b.reply = w[1]
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
