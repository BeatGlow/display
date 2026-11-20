package display

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
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
	proxyFrame      = 1
	proxyShow       = 2
	proxyBrightness = 3
	proxyReady      = 0xfe
	proxyStart      = 0xff
	proxyColumnSize = (gu256x64d7000Height + 4) / 5
	proxyFrameSize  = gu256x64d7000Width * proxyColumnSize
	proxyTimeout    = time.Second
)

var errProxyRejected = errors.New("Nano Every rejected the request")

// OpenGU256X64D7000ProxySPI opens the Nano Every proxy on a registered SPI port
// (for example, "/dev/spidev0.0") at 2 MHz, mode 0, MSB first.
func OpenGU256X64D7000ProxySPI(portName string, config *Config) (Display, error) {
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
// (for example, "/dev/ttyACM0") at 115200 baud.
func OpenGU256X64D7000ProxyUSB(portName string, config *Config) (Display, error) {
	if portName == "" {
		return nil, errors.New("GU256X64D-7000 proxy requires a USB serial port name")
	}
	port, err := serial.Open(portName, &serial.Mode{BaudRate: 115200})
	if err != nil {
		return nil, err
	}
	if err = port.SetReadTimeout(proxyTimeout); err != nil {
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
	if err := s.bus.Tx(packet[1:], nil); err != nil {
		return err
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
	n, err := s.port.Write(packet)
	if err != nil {
		return err
	}
	if n != len(packet) {
		return io.ErrShortWrite
	}
	deadline := time.Now().Add(proxyTimeout)
	var reply [1]byte
	for time.Now().Before(deadline) {
		n, err := s.port.Read(reply[:])
		if err != nil {
			return err
		}
		if n == 0 {
			break
		}
		if done, err := proxyReply(reply[0], token); done {
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
	link   proxyLink
	port   io.Closer
	mu     sync.Mutex
	token  byte
	closed bool
	packet [proxyFrameSize + 6]byte
}

func newGU256X64D7000Proxy(link proxyLink, port io.Closer, config *Config) (Display, error) {
	if config == nil {
		config = &Config{}
	}
	w, h := gu256x64d7000Width, gu256x64d7000Height
	if (config.Width != 0 && config.Width != w) || (config.Height != 0 && config.Height != h) {
		_ = port.Close()
		return nil, fmt.Errorf("GU256X64D-7000 proxy requires %dx%d: %w", w, h, ErrBounds)
	}
	d := &gu256x64d7000Proxy{
		baseDisplay: baseDisplay{width: w, height: h},
		link:        link,
		port:        port,
	}
	if config.UseMono {
		d.Image = pixel.NewMonoVerticalLSBImage(w, h)
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
	payload := d.packet[3 : 3+proxyFrameSize]
	for x := range d.width {
		for group := range proxyColumnSize {
			var packed, place byte = 0, 1
			for y := 5 * group; y < min(5*(group+1), d.height); y++ {
				luma := color.GrayModel.Convert(d.Image.At(x, y)).(color.Gray).Y
				switch {
				case luma >= 192:
					packed += 2 * place
				case luma >= 64:
					packed += place
				}
				place *= 3
			}
			payload[x*proxyColumnSize+group] = packed
		}
	}
	return d.send(proxyFrame, proxyFrameSize)
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
