package display

import (
	"fmt"
	"time"

	"periph.io/x/conn/v3/gpio"

	"github.com/BeatGlow/display/conn"
	"github.com/BeatGlow/display/pixel"
)

const (
	ssd1362Width  = 256
	ssd1362Height = 64
)

// SSD1362 drives 256x64 16-level grayscale OLED modules over 4-wire SPI.
// The controller uses rising-edge SPI (mode 0) and stores two pixels per byte.
type ssd1362 struct {
	ssd1xxxDisplay
}

// SSD1362 creates a driver for an SSD1362 256x64 grayscale display.
func SSD1362(c Conn, config *Config) (Display, error) {
	if spi, ok := c.(SPI); ok {
		if err := spi.SetMode(conn.SPIMode0); err != nil {
			return nil, err
		}
		if err := spi.SetMaxSpeed(2500000); err != nil {
			return nil, err
		}
	}

	d := &ssd1362{ssd1xxxDisplay: ssd1xxxDisplay{monoDisplay: monoDisplay{baseDisplay: baseDisplay{c: c}}}}
	if err := d.c.Reset(gpio.Low); err != nil {
		return nil, err
	}
	time.Sleep(150 * time.Millisecond)
	if err := d.c.Reset(gpio.High); err != nil {
		return nil, err
	}
	time.Sleep(250 * time.Millisecond)
	if err := d.init(config); err != nil {
		return nil, err
	}
	if err := d.SetRotation(config.Rotation); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *ssd1362) init(config *Config) error {
	if config.Width == 0 {
		config.Width = ssd1362Width
	}
	if config.Height == 0 {
		config.Height = ssd1362Height
	}
	if config.Width != ssd1362Width || config.Height != ssd1362Height {
		return fmt.Errorf("display: SSD1362 unsupported size %dx%d", config.Width, config.Height)
	}
	if err := d.ssd1xxxDisplay.init(config); err != nil {
		return err
	}
	d.Image = pixel.NewGray4Image(d.width, d.height)

	if err := d.commands(
		[]byte{0xFD, 0x12},       // Unlock commands.
		[]byte{0xAE},             // Display off.
		[]byte{0x15, 0x00, 0x7F}, // 128 byte columns.
		[]byte{0x75, 0x00, 0x3F}, // 64 rows.
		[]byte{0x81, 0x2F},       // Contrast.
		[]byte{0xA0, 0xC3},       // Horizontal addressing and nibble/segment remap.
		[]byte{0xA1, 0x00},       // Display start line.
		[]byte{0xA2, 0x00},       // Display offset.
		[]byte{0xA4},             // Resume from RAM.
		[]byte{0xA8, 0x3F},       // 64-way multiplex.
		[]byte{0xAB, 0x01},       // Enable internal VDD regulator.
		[]byte{0xAD, 0x8E},       // Internal IREF configuration.
		[]byte{0xB1, 0x22},       // Phase length.
		[]byte{0xB3, 0xA0},       // Display clock.
		[]byte{0xB6, 0x04},       // Second precharge period.
		[]byte{0xB9},             // Default linear grayscale table.
		[]byte{0xBC, 0x10},       // Precharge voltage.
		[]byte{0xBD, 0x01},       // Enable precharge voltage regulator.
		[]byte{0xBE, 0x07},       // VCOMH voltage.
		[]byte{0xAF},             // Display on.
	); err != nil {
		return err
	}
	time.Sleep(2 * time.Millisecond)
	if err := d.SetContrast(0x2F); err != nil {
		return err
	}
	if err := d.Refresh(); err != nil {
		return err
	}
	return d.Show(true)
}

func (d *ssd1362) SetRotation(rotation Rotation) error {
	return d.baseDisplay.setHardwareRotation(rotation, func(flip bool) error {
		remap := byte(0xC3) // Existing init remap value.
		if flip {
			remap = (remap ^ 0x01) | 0x10
		}
		return d.command(0xA0, remap)
	})
}

func (d *ssd1362) String() string {
	return fmt.Sprintf("SSD1362 grayscale OLED %dx%d", d.width, d.height)
}

func (d *ssd1362) Close() error {
	if !d.halted {
		if err := d.Show(false); err != nil {
			_ = d.c.Close()
			return err
		}
		d.halted = true
	}
	return d.c.Close()
}

func (d *ssd1362) SetContrast(level uint8) error {
	return d.command(0x81, level)
}

func (d *ssd1362) Refresh() error {
	if err := d.commands(
		[]byte{0x15, 0x00, 0x7F},
		[]byte{0x75, 0x00, 0x3F},
	); err != nil {
		return err
	}
	image, ok := d.Image.(*pixel.Gray4Image)
	if !ok {
		return fmt.Errorf("display: SSD1362 requires a 4-bit grayscale framebuffer")
	}
	// SSD1362 starts writing GDDRAM after the address window is set; unlike
	// SSD1322, it does not use a separate Write RAM command.
	return d.data(image.Pix...)
}

var _ Display = (*ssd1362)(nil)
