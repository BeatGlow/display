package conn

import (
	"fmt"
	"os"

	"github.com/BeatGlow/display/internal/ioctl"
)

// Definitions from <spi/spidev.h>
const (
	spiCPHA = 0x01
	spiCPOL = 0x02
)

type SPIMode uint8

const (
	SPIMode0 SPIMode = (0 | 0)             //nolint:staticcheck
	SPIMode1 SPIMode = (0 | spiCPHA)       //nolint:staticcheck
	SPIMode2 SPIMode = (spiCPOL | 0)       //nolint:staticcheck
	SPIMode3 SPIMode = (spiCPOL | spiCPHA) //nolint:staticcheck
)

const (
	spiIOCMagic       = 0x6b // 'k'
	spiIOCMode        = 0x6b01
	spiIOCLSBFirst    = 0x6b02
	spiIOCBitsPerWord = 0x6b03
	spiIOCMaxSpeedHz  = 0x6b04
	spiIOCMode32      = 0x6b05
)

// SPI implements the spidev interface.
type SPI struct {
	f           *os.File
	fd          uintptr
	mode        SPIMode
	bitsPerWord uint8
	maxSpeedHz  uint32
}

// OpenSPI opens the numbered spi bus with the numbered device. The device often corresponds to the CS pin for that bus.
func OpenSPI(bus, device int) (*SPI, error) {
	spidev := fmt.Sprintf("%s%d.%d", spiDevPath, bus, device)
	f, err := os.OpenFile(spidev, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	c := &SPI{
		f:  f,
		fd: f.Fd(),
	}
	if err = ioctl.Do(c.fd, ioctl.Pointer(ioctl.Read, &c.mode, spiIOCMode), &c.mode); err != nil {
		_ = f.Close()
		return nil, err
	}
	if err = ioctl.Do(c.fd, ioctl.Pointer(ioctl.Read, &c.bitsPerWord, spiIOCBitsPerWord), &c.bitsPerWord); err != nil {
		return nil, err
	}
	if err = ioctl.Do(c.fd, ioctl.Pointer(ioctl.Read, &c.maxSpeedHz, spiIOCMaxSpeedHz), &c.maxSpeedHz); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *SPI) Close() error {
	return c.f.Close()
}

func (c *SPI) String() string {
	return fmt.Sprintf("SPI mode=%d bits per word=%d max speed=%dHz", c.mode, c.bitsPerWord, c.maxSpeedHz)
}

func (c *SPI) Mode() SPIMode {
	return c.mode
}

func (c *SPI) SetMode(mode SPIMode) error {
	mode &= 0x0f

	if err := ioctl.Do(c.fd, ioctl.Pointer(ioctl.Write, &mode, spiIOCMode), &mode); err != nil {
		return err
	}

	var test SPIMode
	if err := ioctl.Do(c.fd, ioctl.Pointer(ioctl.Read, &test, spiIOCMode), &test); err != nil {
		return err
	}

	if test != mode {
		return fmt.Errorf("conn: SPI attempted to set mode %#02x, but mode %#02x is in use", mode, test)
	}

	c.mode = mode
	return nil
}

func (c *SPI) BitsPerWord() uint8 {
	return c.bitsPerWord
}

func (c *SPI) SetBitsPerWord(bits uint8) error {
	if bits < 8 || bits > 32 {
		return fmt.Errorf("conn: SPI bits per word need to be 8 or more and 32 or less, got %d", bits)
	}

	if c.bitsPerWord != bits {
		if err := ioctl.Do(c.fd, ioctl.Pointer(ioctl.Write, &bits, spiIOCBitsPerWord), &bits); err != nil {
			return err
		}
		c.bitsPerWord = bits
	}

	return nil
}

func (c *SPI) MaxSpeed() int {
	return int(c.maxSpeedHz)
}

func (c *SPI) SetMaxSpeed(v int) error {
	if v < 0 {
		return nil
	}

	u := uint32(v)
	if c.maxSpeedHz != u {
		if err := ioctl.Do(c.fd, ioctl.Pointer(ioctl.Write, &u, spiIOCMaxSpeedHz), &u); err != nil {
			return err
		}
		c.maxSpeedHz = u
	}

	return nil
}

func (c *SPI) Read(b []byte) (n int, err error) {
	return c.f.Read(b)
}

func (c *SPI) Write(b []byte) (n int, err error) {
	return c.f.Write(b)
}
