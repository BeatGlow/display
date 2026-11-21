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
	port := &testProxyUSB{missStart: true}
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
	frame := make([]byte, 3334)
	copy(frame, []byte{0xff, 0x03, 3, 7})
	frame[3+12], frame[3+3327] = 54, 54
	copy(frame[3331:], []byte{111, 114, 2})
	for _, tc := range []struct {
		name string
		got  []byte
		want []byte
	}{
		{"grayscale frame", port.packets[2], frame},
		{"maximum brightness", port.packets[3], []byte{0xff, 0x68, 4, 8, 6, 126, 1}},
		{"power off", port.packets[4], []byte{0xff, 0x50, 5, 0, 59, 66, 2}},
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

func TestGU256X64D7000ProxySendsNativeFrameFormatsOnSPIAndUSB(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		config                  GU256X64D7000ProxyConfig
		model                   color.Model
		size                    int
		op, first, second, last byte
		crc                     [3]byte
	}{
		{"monochrome", GU256X64D7000ProxyConfig{Config: Config{UseMono: true}}, pixel.MonoModel, 2048, 0x02, 255, 0, 1, [3]byte{91, 20, 2}},
		{"three levels", GU256X64D7000ProxyConfig{}, pixel.Gray3LevelModel, 3328, 0x03, 210, 242, 54, [3]byte{5, 105, 1}},
		{"four levels", GU256X64D7000ProxyConfig{GrayLevels: 4}, pixel.Gray2Model, 4096, 0x04, 228, 255, 0, [3]byte{48, 97, 0}},
	} {
		for _, transport := range []string{"SPI", "USB"} {
			t.Run(tc.name+"/"+transport, func(t *testing.T) {
				link, port, packets := testProxyTransport(transport)
				d, err := newGU256X64D7000Proxy(link, port, &tc.config)
				if err != nil {
					t.Fatal(err)
				}
				if d.ColorModel() != tc.model {
					t.Fatalf("display color model = %T, want %T", d.ColorModel(), tc.model)
				}
				switch tc.model {
				case pixel.MonoModel:
					for y := range 8 {
						d.Set(0, y, color.White)
					}
					d.Set(255, 63, color.White)
				case pixel.Gray3LevelModel:
					for y, level := range []uint8{0, 1, 2, 1, 2, 2, 2, 2, 2, 2} {
						d.Set(0, y, pixel.Gray3Level{Y: level})
					}
					d.Set(255, 63, color.White)
				case pixel.Gray2Model:
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
				if len(*packets) != 3 {
					t.Fatalf("sent %d packets, want 3", len(*packets))
				}
				got := (*packets)[2]
				if !bytes.Equal(got, want) {
					index := 0
					for index < min(len(got), len(want)) && got[index] == want[index] {
						index++
					}
					t.Errorf("packet differs at byte %d (got %d bytes, want %d)", index, len(got), len(want))
				}
				if err := d.Close(); err != nil {
					t.Error(err)
				}
			})
		}
	}
}

func TestGU256X64D7000ProxySendsPositionedTextAndRectanglesOnSPIAndUSB(t *testing.T) {
	for _, transport := range []string{"SPI", "USB"} {
		t.Run(transport, func(t *testing.T) {
			link, port, packets := testProxyTransport(transport)
			d, err := newGU256X64D7000Proxy(link, port, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := d.Close(); err != nil {
					t.Error(err)
				}
			})
			drawer := d.(GU256X64D7000ProxyDisplay)
			src := image.NewGray(image.Rect(10, 20, 12, 36))
			for _, pt := range []image.Point{{X: 10, Y: 20}, {X: 10, Y: 35}, {X: 11, Y: 27}, {X: 11, Y: 28}} {
				src.SetGray(pt.X, pt.Y, color.Gray{Y: 255})
			}
			if err := drawer.DrawBitmap(image.Pt(254, 48), src); err != nil {
				t.Fatal(err)
			}
			if err := drawer.DrawTextAt(image.Pt(242, 56), "A~"); err != nil {
				t.Fatal(err)
			}
			if err := drawer.DrawBitmap(image.Point{}, image.NewGray(image.Rect(0, 0, 256, 64))); err != nil {
				t.Fatal(err)
			}
			full := make([]byte, 2061)
			copy(full, []byte{0xff, 0x21, 5, 5, 8, 0, 0, 0, 1, 8})
			copy(full[len(full)-3:], []byte{104, 15, 0})
			want := [][]byte{
				{0xff, 0x21, 3, 9, 0, 254, 6, 2, 0, 2, 0x80, 0x01, 0x01, 0x80, 87, 56, 2},
				{0xff, 0x34, 4, 4, 0, 242, 7, 'A', '~', 71, 49, 3},
				full,
			}
			if got := len(*packets); got != 2+len(want) {
				t.Fatalf("sent %d packets including setup, want %d", got, 2+len(want))
			}
			for i, packet := range want {
				if got := (*packets)[i+2]; !bytes.Equal(got, packet) {
					t.Errorf("drawing request %d differs from the expected wire packet (got %d bytes, want %d)", i, len(got), len(packet))
				}
			}
		})
	}
}

func TestGU256X64D7000ProxyRejectsInvalidDrawingsBeforeTransmission(t *testing.T) {
	port := &testProxyUSB{}
	d, err := newGU256X64D7000Proxy(proxyUSB{port}, port, nil)
	if err != nil {
		t.Fatal(err)
	}
	drawer := d.(GU256X64D7000ProxyDisplay)
	bitmap := image.NewGray(image.Rect(0, 0, 2, 8))
	for _, tc := range []struct {
		name   string
		draw   func() error
		bounds bool
	}{
		{"bitmap past the right edge", func() error { return drawer.DrawBitmap(image.Pt(255, 0), bitmap) }, true},
		{"bitmap between VFD rows", func() error { return drawer.DrawBitmap(image.Pt(0, 1), bitmap) }, true},
		{"bitmap with partial VFD row", func() error { return drawer.DrawBitmap(image.Point{}, image.NewGray(image.Rect(0, 0, 2, 7))) }, true},
		{"text wraps past the right edge", func() error { return drawer.DrawTextAt(image.Pt(250, 0), "A") }, true},
		{"text contains a VFD control byte", func() error { return drawer.DrawTextAt(image.Point{}, "A\x1f") }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.draw(); err == nil || tc.bounds && !errors.Is(err, ErrBounds) {
				t.Errorf("invalid drawing returned %v", err)
			}
			if got := len(port.packets); got != 2 {
				t.Fatalf("invalid drawing sent a request, got %d packets including setup", got)
			}
		})
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if err := drawer.DrawTextAt(image.Point{}, "A"); !errors.Is(err, io.ErrClosedPipe) {
		t.Errorf("drawing after Close = %v, want io.ErrClosedPipe", err)
	}
}

func testProxyTransport(transport string) (proxyLink, io.Closer, *[][]byte) {
	if transport == "SPI" {
		bus := &testProxySPI{}
		return proxySPI{bus}, bus, &bus.packets
	}
	usb := &testProxyUSB{}
	return proxyUSB{usb}, usb, &usb.packets
}

type testProxyUSB struct {
	packets      [][]byte
	reply        []byte
	closed       bool
	missStart    bool
	awaitingBody bool
	processing   bool
}

func (p *testProxyUSB) ResetInputBuffer() error {
	p.reply = nil
	return nil
}

func (p *testProxyUSB) Write(data []byte) (int, error) {
	if bytes.Equal(data, []byte{0xff}) {
		// Opening CDC can reset the AVR, which loses the first marker.
		if p.missStart {
			p.missStart = false
		} else {
			p.reply = []byte{0xfe}
		}
		return len(data), nil
	}
	if !p.awaitingBody || len(data) < 2 {
		return 0, errors.New("USB request body arrived before the Nano was ready")
	}
	p.awaitingBody = false
	p.packets = append(p.packets, append([]byte{0xff}, data...))
	p.reply = []byte{0, data[1]}
	p.processing = true
	return len(data), nil
}

func (p *testProxyUSB) Read(reply []byte) (int, error) {
	if p.processing {
		// A VFD write may take longer than one USB read timeout.
		p.processing = false
		return 0, nil
	}
	n := copy(reply, p.reply)
	p.reply = p.reply[n:]
	if n != 0 && reply[0] == 0xfe {
		p.awaitingBody = true
	}
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
	packets    [][]byte
	packet     []byte
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
			case 0x50, 0x68:
				b.remaining = 6
			case 0x04:
				b.remaining = 4101
			case 0x03:
				b.remaining = 3333
			case 0x02:
				b.remaining = 2053
			case 0x21, 0x34:
				if len(w) < 4 {
					return errors.New("SPI drawing request has no length")
				}
				b.remaining = 7 + (int(w[2]) | int(w[3])<<8)
			default:
				return fmt.Errorf("unknown proxy opcode %d", w[0])
			}
			b.reply = w[1]
			b.packet = []byte{0xff}
		}
		if len(w) != min(4096, b.remaining) {
			return fmt.Errorf("SPI chunk length %d with %d bytes left in request", len(w), b.remaining)
		}
		b.packet = append(b.packet, w...)
		b.remaining -= len(w)
		if b.remaining != 0 {
			return nil
		}
		b.packets = append(b.packets, b.packet)
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
