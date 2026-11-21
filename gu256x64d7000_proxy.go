package display

import (
	"errors"
	"fmt"
	"image"
	"io"
	"math/bits"
	"sync"
	"time"

	"go.bug.st/serial"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/host/v3"

	"github.com/BeatGlow/display/pixel"
)

const (
	proxyShow       = 2
	proxyBrightness = 3
	proxyGray3      = 4
	proxyMono       = 5
	proxyGray4      = 6
	proxyReady      = 0xfe
	proxyStart      = 0xff
	proxyMonoColumn = gu256x64d7000Height / 8
	proxyGrayColumn = gu256x64d7000Height / 4
	proxyMonoSize   = gu256x64d7000Width * proxyMonoColumn
	proxyGraySize   = gu256x64d7000Width * proxyGrayColumn
	proxySPIChunk   = 4096 // Default Linux spidev message limit.
	proxyTimeout    = time.Second
	proxyUSBRate    = 1_000_000
	proxyUSBPoll    = 50 * time.Millisecond
)

var errProxyRejected = errors.New("request rejected by Nano Every")

// GU256X64D7000ProxyConfig configures the Nano Every display.
type GU256X64D7000ProxyConfig struct {
	Config
	// GrayLevels selects 3 (the default) or 4 levels. UseMono selects 1-bit frames.
	GrayLevels int
}

// OpenGU256X64D7000ProxySPI opens the Nano Every proxy on a registered SPI port
// (for example, "/dev/spidev0.0") at 2 MHz, mode 0, MSB first.
func OpenGU256X64D7000ProxySPI(portName string, config *GU256X64D7000ProxyConfig) (Display, error) {
	if _, err := host.Init(); err != nil {
		return nil, err
	}
	port, err := spireg.Open(portName)
	if err != nil {
		return nil, err
	}
	bus, err := port.Connect(2*physic.MegaHertz, spi.Mode0, 8)
	if err != nil {
		_ = port.Close()
		return nil, err
	}
	return newGU256X64D7000Proxy(proxySPI{bus}, port, config)
}

// OpenGU256X64D7000ProxyUSB opens the Nano Every proxy's USB CDC serial port
// (for example, "/dev/ttyACM0") at 1 Mbaud.
func OpenGU256X64D7000ProxyUSB(portName string, config *GU256X64D7000ProxyConfig) (Display, error) {
	if portName == "" {
		return nil, errors.New("GU256X64D-7000 proxy requires a USB serial port name")
	}
	port, err := serial.Open(portName, &serial.Mode{BaudRate: proxyUSBRate})
	if err != nil {
		return nil, err
	}
	if err = port.SetReadTimeout(proxyUSBPoll); err != nil {
		_ = port.Close()
		return nil, err
	}
	return newGU256X64D7000Proxy(proxyUSB{port}, port, config)
}

type proxyLink interface {
	exchange(packet []byte, token byte) error
}

type proxySPI struct {
	bus interface{ Tx(w, r []byte) error }
}

func (s proxySPI) exchange(packet []byte, token byte) error {
	if err := s.bus.Tx(packet[:1], nil); err != nil {
		return err
	}
	if err := s.wait(proxyReady); err != nil {
		return fmt.Errorf("waiting for SPI receiver: %w", err)
	}
	for body := packet[1:]; len(body) != 0; {
		n := min(len(body), proxySPIChunk)
		if err := s.bus.Tx(body[:n], nil); err != nil {
			return err
		}
		body = body[n:]
	}
	return s.wait(token)
}

func (s proxySPI) wait(expected byte) error {
	// Clock dummy bytes to allow the slave's transmit buffer to catch up.
	var dummy, reply [4]byte
	deadline := time.Now().Add(proxyTimeout)
	for time.Now().Before(deadline) {
		if err := s.bus.Tx(dummy[:], reply[:]); err != nil {
			return err
		}
		if done, err := proxyReply(reply[len(reply)-1], expected); done {
			return err
		}
		time.Sleep(time.Millisecond)
	}
	return ErrNotReady
}

type proxyUSB struct {
	port interface {
		io.ReadWriter
		ResetInputBuffer() error
	}
}

func (s proxyUSB) exchange(packet []byte, token byte) error {
	if err := s.port.ResetInputBuffer(); err != nil {
		return err
	}
	if err := s.wait(proxyReady, packet[:1]); err != nil {
		return fmt.Errorf("waiting for USB receiver: %w", err)
	}
	if err := s.write(packet[1:]); err != nil {
		return err
	}
	return s.wait(token, nil)
}

func (s proxyUSB) write(data []byte) error {
	n, err := s.port.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return nil
}

func (s proxyUSB) wait(expected byte, marker []byte) error {
	// MuxTO can reset the AVR when CDC opens. Retry the marker after a read
	// timeout while waiting for readiness. Never resend a request body.
	sendMarker := len(marker) != 0
	deadline := time.Now().Add(proxyTimeout)
	var reply [1]byte
	for time.Now().Before(deadline) {
		if sendMarker {
			if err := s.write(marker); err != nil {
				return err
			}
			sendMarker = false
		}
		n, err := s.port.Read(reply[:])
		if err != nil {
			return err
		}
		if n == 0 {
			sendMarker = len(marker) != 0
			continue
		}
		if done, err := proxyReply(reply[0], expected); done {
			return err
		}
	}
	return ErrNotReady
}

func proxyReply(reply, expected byte) (bool, error) {
	switch {
	case reply == expected:
		return true, nil
	case expected != proxyReady && reply == expected|0x80:
		return true, errProxyRejected
	default:
		return false, nil
	}
}

type gu256x64d7000Proxy struct {
	baseDisplay
	link    proxyLink
	port    io.Closer
	mu      sync.Mutex
	token   byte
	closed  bool
	frameOp byte
	packet  [proxyGraySize + 6]byte
}

func newGU256X64D7000Proxy(link proxyLink, port io.Closer, config *GU256X64D7000ProxyConfig) (Display, error) {
	if config == nil {
		config = &GU256X64D7000ProxyConfig{}
	}
	w, h := gu256x64d7000Width, gu256x64d7000Height
	if (config.Width != 0 && config.Width != w) || (config.Height != 0 && config.Height != h) {
		_ = port.Close()
		return nil, fmt.Errorf("GU256X64D-7000 proxy requires %dx%d: %w", w, h, ErrBounds)
	}
	mode := byte(proxyGray3)
	switch config.GrayLevels {
	case 0, 3:
	case 4:
		mode = proxyGray4
	default:
		_ = port.Close()
		return nil, fmt.Errorf("GU256X64D-7000 proxy supports 3 or 4 grayscale levels, got %d", config.GrayLevels)
	}
	d := &gu256x64d7000Proxy{
		baseDisplay: baseDisplay{width: w, height: h},
		link:        link,
		port:        port,
		frameOp:     mode,
	}
	if config.UseMono {
		d.frameOp = proxyMono
		d.Image = pixel.NewMonoVerticalLSBImage(w, h)
	} else if mode == proxyGray3 {
		d.Image = pixel.NewGray3LevelImage(w, h)
	} else {
		d.Image = image.NewGray(image.Rect(0, 0, w, h))
	}
	_ = d.SetRotation(config.Rotation)
	if err := d.Refresh(); err != nil {
		_ = port.Close()
		return nil, err
	}
	if err := d.Show(true); err != nil {
		_ = port.Close()
		return nil, err
	}
	return d, nil
}

func (d *gu256x64d7000Proxy) String() string {
	return "GU256X64D-7000 256x64 VFD via Nano Every"
}

func (d *gu256x64d7000Proxy) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil
	}
	err := d.control(proxyShow, 0)
	d.closed = true
	return errors.Join(err, d.port.Close())
}

func (d *gu256x64d7000Proxy) Show(show bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	var value byte
	if show {
		value = 1
	}
	return d.control(proxyShow, value)
}

func (d *gu256x64d7000Proxy) SetContrast(level uint8) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.control(proxyBrightness, level/32+1)
}

func (d *gu256x64d7000Proxy) Refresh() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return io.ErrClosedPipe
	}
	if d.frameOp == proxyMono {
		buffer := d.Image.(*pixel.MonoVerticalLSBImage)
		for x := range d.width {
			for band := range proxyMonoColumn {
				d.packet[3+x*proxyMonoColumn+band] = bits.Reverse8(buffer.Pix[band*buffer.Stride+x])
			}
		}
		return d.send(proxyMono, proxyMonoSize)
	}

	gray3, _ := d.Image.(*pixel.Gray3LevelImage)
	gray4, _ := d.Image.(*image.Gray)
	payload := d.packet[3 : 3+proxyGraySize]
	for x := range d.width {
		for group := range proxyGrayColumn {
			var packed byte
			for dy := range 4 {
				y := 4*group + dy
				var level byte
				if gray3 != nil {
					level = gray3.Gray3LevelAt(x, y).Y
				} else {
					level = gray4.GrayAt(x, y).Y >> 6
				}
				packed |= level << (2 * dy)
			}
			payload[x*proxyGrayColumn+group] = packed
		}
	}
	return d.send(d.frameOp, proxyGraySize)
}

func (d *gu256x64d7000Proxy) control(op, value byte) error {
	if d.closed {
		return io.ErrClosedPipe
	}
	d.packet[3] = value
	return d.send(op, 1)
}

func (d *gu256x64d7000Proxy) send(op byte, payloadLen int) error {
	d.token = d.token%125 + 1
	d.packet[0], d.packet[1], d.packet[2] = proxyStart, op, d.token
	end := 3 + payloadLen
	// CRC-16/XMODEM, split into seven-bit bytes to reserve the delimiters.
	var crc uint16
	for _, b := range d.packet[1:end] {
		crc ^= uint16(b) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	d.packet[end], d.packet[end+1], d.packet[end+2] = byte(crc&127), byte(crc>>7&127), byte(crc>>14)
	if err := d.link.exchange(d.packet[:end+3], d.token); err != nil {
		return fmt.Errorf("GU256X64D-7000 proxy operation %d: %w", op, err)
	}
	return nil
}
