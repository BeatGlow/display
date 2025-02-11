package display

import (
	"fmt"
	"log"

	"github.com/BeatGlow/display/conn"
	"github.com/BeatGlow/display/pixel"
)

const (
	sh1122DefaultWidth         = 256
	sh1122DefaultHeight        = 64
	sh1122SetDischargeVSLLevel = 0x30
	sh1122SetDisplayStartLine  = 0x40
	sh1122SetRowAddress        = 0xB0
	sh1122SetDCDC              = 0xAD
	sh1122SetVSEGMLevel        = 0xDC
)

type sh1122 struct {
	gray4Display
	pages  int
	width  int
	halted bool
}

func SH1122(c Conn, config *Config) (Display, error) {
	// Update mode and speed
	if spi, ok := c.(SPI); ok {
		if err := spi.SetMode(conn.SPIMode0); err != nil {
			return nil, err
		}
		if err := spi.SetMaxSpeed(8000000); err != nil {
			return nil, err
		}
	}

	d := &sh1122{
		gray4Display: gray4Display{
			baseDisplay: baseDisplay{
				c: c,
			},
		},
	}

	if config.Width == 0 {
		config.Width = sh1122DefaultWidth
	}
	if config.Height == 0 {
		config.Height = sh1122DefaultHeight
	}
	d.pages = config.Height >> 1
	d.width = config.Width

	if err := d.init(config); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *sh1122) Close() error {
	if !d.halted {
		if err := d.Show(false); err != nil {
			_ = d.c.Close()
			return err
		}
		d.halted = true
	}
	return d.c.Close()
}

func (d *sh1122) String() string {
	bounds := d.Bounds()
	return fmt.Sprintf("SH1122 OLED %dx%d", bounds.Dx(), bounds.Dy())
}

func (d *sh1122) init(config *Config) (err error) {
	var (
		multiplexRatio byte = byte(config.Height - 1)
		displayOffset  byte
	)
	switch {
	case config.Width == 128 && config.Height == 32:
		displayOffset = 0x0f
	case config.Width == 128 && config.Height == 64:
		displayOffset = 0x00
	case config.Width == 128 && config.Height == 128:
		displayOffset = 0x02
	case config.Width == 256 && config.Height == 64:
		displayOffset = 0x00
	default:
		return fmt.Errorf("display: SH1122 unsupported size %dx%d", config.Width, config.Height)
	}

	// init base
	log.Printf("init %dx%d", config.Width, config.Height)
	if err = d.gray4Display.init(config); err != nil {
		return
	}

	// init display
	if err = d.commands(
		[]byte{ssd1xxxSetDisplayOff},
		[]byte{sh1122SetDisplayStartLine},
		[]byte{ssd1322SetRemap},
		[]byte{ssd1xxxSetComScanInc},
		[]byte{ssd1xxxSetContrast, 0x80},
		[]byte{ssd1xxxSetMultiplexRatio, multiplexRatio},
		[]byte{sh1122SetDCDC, 0x81},
		[]byte{ssd1xxxSetDisplayClockDiv, 0x50},
		[]byte{ssd1xxxSetDisplayOffset, displayOffset},
		[]byte{ssd1xxxSetPrecharge, 0x21},
		[]byte{ssd1xxxSetVCOMDeselect, 0x35},
		[]byte{sh1122SetVSEGMLevel, 0x35},
		[]byte{sh1122SetDischargeVSLLevel},
		[]byte{ssd1xxxSetLowColumn},
		[]byte{ssd1xxxSetHighColumn},
		[]byte{sh1122SetRowAddress},
		[]byte{ssd1xxxSetNormalDisplay},
		[]byte{ssd1xxxSetDisplayOn},
	); err != nil {
		return fmt.Errorf("init command failed: %w", err)
	}

	if err = d.SetContrast(0x7F); err != nil {
		return fmt.Errorf("init setting contrast failed: %w", err)
	}
	if err = d.Refresh(); err != nil {
		return fmt.Errorf("init refresh failed: %w", err)
	}
	if err = d.Show(true); err != nil {
		return fmt.Errorf("init show failed: %w", err)
	}

	return
}

func (d *sh1122) Show(show bool) error {
	if show {
		return d.command(ssd1xxxSetDisplayOn)
	} else {
		return d.command(ssd1xxxSetDisplayOff)
	}
}

func (d *sh1122) SetContrast(level uint8) error {
	return d.command(ssd1xxxSetContrast, level)
}

func (d *sh1122) SetRotation(rotation Rotation) error {
	d.rotation = rotation
	return nil
}

func (d *sh1122) Refresh() (err error) {
	pix := d.Image.(*pixel.Gray4Image).Pix
	log.Printf("push %d pixels", len(pix))
	if err = d.commands(
		[]byte{ssd1xxxSetLowColumn},
		[]byte{ssd1xxxSetHighColumn},
		[]byte{sh1122SetRowAddress},
	); err != nil {
		return
	}
	for i, l := 0, len(pix); i < l; i += 4096 {
		if err = d.data(pix[i : i+4096]...); err != nil {
			return
		}
	}
	return
}
