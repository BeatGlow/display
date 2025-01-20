package display

import (
	"fmt"

	"github.com/BeatGlow/display/pixel"
)

const (
	ssd1305DefaultWidth    = 128
	ssd1305DefaultHeight   = 32
	ssd1305SetPageAddr     = 0x22
	ssd1305SetLUT          = 0x91
	ssd1305SetMasterConfig = 0xAD
	ssd1305setAreaColor    = 0xD8
)

type ssd1305 struct {
	monoDisplay
	pageSize   int
	pageOffset int
}

// SSD1305 is a driver for the Sino Wealth SSD1305 OLED display.
func SSD1305(conn Conn, config *Config) (Display, error) {
	d := &ssd1305{
		monoDisplay: monoDisplay{
			baseDisplay: baseDisplay{
				c: conn,
			},
		},
	}

	if config.Width == 0 {
		config.Width = ssd1305DefaultWidth
	}
	if config.Height == 0 {
		config.Height = ssd1305DefaultHeight
	}
	d.pageSize = config.Height >> 3
	d.width = config.Width

	if err := d.init(config); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *ssd1305) String() string {
	bounds := d.Bounds()
	return fmt.Sprintf("ssd1305 %dx%d", bounds.Dx(), bounds.Dy())
}

func (d *ssd1305) init(config *Config) (err error) {
	var (
		lowColumn  byte
		highColumn byte
	)
	switch {
	case config.Width == 128 && config.Height == 32:
		lowColumn, highColumn = 0, 0
	case config.Width == 128 && config.Height == 64:
		lowColumn, highColumn = 4, 4
	default:
		return fmt.Errorf("display: ssd1305 unsupported size %dx%d", config.Width, config.Height)
	}

	// init base
	if err = d.monoDisplay.init(config); err != nil {
		return
	}

	// init display
	if err = d.command(
		ssd1xxxSetDisplayOff,
		ssd1xxxSetLowColumn|lowColumn,
		ssd1xxxSetHighColumn|highColumn,
		ssd1xxxSetStartLine, 0x00,
		ssd1xxxSetSegmentRemap|0x01,
		ssd1xxxSetNormalDisplay,
		ssd1xxxSetMultiplexRatio, 0x3F,
		ssd1305SetMasterConfig, 0x8E,
		ssd1xxxSetComScanDec,
		ssd1xxxSetDisplayOffset, 0x40,
		ssd1xxxSetDisplayClockDiv, 0xF0,
		ssd1305setAreaColor, 0x05,
		ssd1xxxSetPrecharge, 0xF1,
		ssd1xxxSetComPins, 0x12,
		ssd1305SetLUT, 0x3F, 0x3F, 0x3F, 0x3F,
	); err != nil {
		return err
	}

	if err = d.SetContrast(0x7F); err != nil {
		return
	}
	if err = d.Refresh(); err != nil {
		return
	}
	if err = d.Show(true); err != nil {
		return
	}

	return
}

func (d *ssd1305) Refresh() (err error) {
	//const pageSize = 8
	pix := d.Image.(*pixel.MonoVerticalLSBImage).Pix
	for page := 0; page < d.pageSize; page++ {
		if err = d.command(
			ssd1305SetPageAddr|byte(page&0x7),
			ssd1xxxSetLowColumn|0x2,
			ssd1xxxSetHighColumn|0x0, //nolint:staticcheck
		); err != nil {
			return
		}
		var (
			off = page * d.width
			end = off + d.width
		)
		if err := d.data(pix[off:end]...); err != nil {
			return err
		}
	}
	return nil
}
