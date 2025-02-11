package display

import (
	"fmt"

	"github.com/BeatGlow/display/pixel"
)

const (
	sh1106DefaultWidth  = 128
	sh1106DefaultHeight = 64
	sh1106SetPageAddr   = 0xB0
)

type sh1106 struct {
	monoDisplay
	pageSize int
	width    int
}

// SH1106 is a driver for the Sino Wealth SH1106 OLED display.
func SH1106(conn Conn, config *Config) (Display, error) {
	d := &sh1106{
		monoDisplay: monoDisplay{
			baseDisplay: baseDisplay{
				c: conn,
			},
		},
	}

	if config.Width == 0 {
		config.Width = sh1106DefaultWidth
	}
	if config.Height == 0 {
		config.Height = sh1106DefaultHeight
	}
	d.pageSize = config.Height >> 3
	d.width = config.Width

	if err := d.init(config); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *sh1106) Close() error {
	if !d.halted {
		if err := d.Show(false); err != nil {
			_ = d.c.Close()
			return err
		}
		d.halted = true
	}
	return d.c.Close()
}

func (d *sh1106) String() string {
	bounds := d.Bounds()
	return fmt.Sprintf("SH1106 OLED %dx%d", bounds.Dx(), bounds.Dy())
}

func (d *sh1106) init(config *Config) (err error) {
	var (
		multiplexRatio byte
		displayOffset  byte
	)
	switch {
	case config.Width == 128 && config.Height == 32:
		multiplexRatio, displayOffset = 0x20, 0x0f
	case config.Width == 128 && config.Height == 64:
		multiplexRatio, displayOffset = 0x3f, 0x00
	case config.Width == 128 && config.Height == 128:
		multiplexRatio, displayOffset = 0xff, 0x02
	default:
		return fmt.Errorf("display: SH1106 unsupported size %dx%d", config.Width, config.Height)
	}

	// init base
	if err = d.monoDisplay.init(config); err != nil {
		return
	}

	// init display
	if err = d.command(
		ssd1xxxSetDisplayOff,
		ssd1xxxSetMemoryMode,
		ssd1xxxSetHighColumn, 0x80, 0xC8,
		ssd1xxxSetLowColumn, 0x10, 0x40,
		ssd1xxxSetSegmentRemap,
		ssd1xxxSetNormalDisplay,
		ssd1xxxSetMultiplexRatio, multiplexRatio,
		ssd1xxxSetDisplayAllOnResume,
		ssd1xxxSetDisplayOffset, displayOffset,
		ssd1xxxSetDisplayClockDiv, 0xF0,
		ssd1xxxSetPrecharge, 0x22,
		ssd1xxxSetComPins, 0x12,
		ssd1xxxSetVCOMDeselect, 0x20,
		ssd1xxxSetChargePump, 0x14,
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

func (d *sh1106) Refresh() (err error) {
	//const pageSize = 8
	pix := d.Image.(*pixel.MonoVerticalLSBImage).Pix
	for page := 0; page < d.pageSize; page++ {
		if err = d.command(
			sh1106SetPageAddr|byte(page&0x7),
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
