package display

import (
	"fmt"

	"github.com/BeatGlow/display/pixel"
)

const (
	ssd1306DefaultWidth  = 128
	ssd1306DefaultHeight = 64
	ssd1306SetPageAddr   = 0xB0
)

type ssd1306 struct {
	monoDisplay
	pageSize int
	width    int
	colStart byte
	colEnd   byte
}

func SSD1306(conn Conn, config *Config) (Display, error) {
	d := &ssd1306{
		monoDisplay: monoDisplay{
			baseDisplay: baseDisplay{
				c: conn,
			},
		},
	}

	if config.Width == 0 {
		config.Width = ssd1306DefaultWidth
	}
	if config.Height == 0 {
		config.Height = ssd1306DefaultHeight
	}

	if err := d.init(config); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *ssd1306) Close() error {
	if !d.halted {
		if err := d.Show(false); err != nil {
			_ = d.c.Close()
			return err
		}
		d.halted = true
	}
	return d.c.Close()
}

func (d *ssd1306) String() string {
	bounds := d.Bounds()
	return fmt.Sprintf("SSD1306 OLED %dx%d", bounds.Dx(), bounds.Dy())
}

func (d *ssd1306) init(config *Config) (err error) {
	var (
		multiplexRatio  byte = byte(config.Width - 1)
		displayClockDiv byte
		comPins         byte
		colStart        byte
	)
	switch {
	case config.Width == 64 && config.Height == 32:
		displayClockDiv, comPins, colStart = 0x80, 0x12, 32
	case config.Width == 64 && config.Height == 48:
		displayClockDiv, comPins, colStart = 0x80, 0x12, 32
	case config.Width == 96 && config.Height == 16:
		displayClockDiv, comPins, colStart = 0x60, 0x02, 0
	case config.Width == 128 && config.Height == 32:
		displayClockDiv, comPins, colStart = 0x80, 0x02, 0
	case config.Width == 128 && config.Height == 64:
		displayClockDiv, comPins, colStart = 0x80, 0x12, 0
	default:
		return fmt.Errorf("display: SSD1306 unsupported size %dx%d", config.Width, config.Height)
	}

	// init paging
	d.pageSize = config.Height >> 3
	d.width = config.Width
	d.colStart = colStart
	d.colEnd = colStart + byte(config.Width)

	// init base
	if err = d.monoDisplay.init(config); err != nil {
		return
	}

	// init display
	if err = d.command(
		ssd1xxxSetDisplayOff,
		ssd1xxxSetDisplayClockDiv, displayClockDiv,
		ssd1xxxSetMultiplexRatio, multiplexRatio,
		ssd1xxxSetDisplayOffset, 0x00,
		ssd1xxxSetStartLine,
		ssd1xxxSetChargePump, 0x14,
		ssd1xxxSetMemoryMode, 0x00,
		ssd1xxxSetSegmentRemap,
		ssd1xxxSetComScanDec,
		ssd1xxxSetComPins, comPins,
		ssd1xxxSetPrecharge, 0xF1,
		ssd1xxxSetVCOMDeselect, 0x40,
		ssd1xxxSetDisplayAllOnResume,
		ssd1xxxSetNormalDisplay,
	); err != nil {
		return err
	}

	if err = d.SetContrast(0xCF); err != nil {
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

func (d *ssd1306) Refresh() (err error) {
	//const pageSize = 8
	pix := d.Image.(*pixel.MonoVerticalLSBImage).Pix
	for page := 0; page < d.pageSize; page++ {
		if err = d.command(
			ssd1xxxSetColumnAddr, d.colStart, d.colEnd-1,
			ssd1xxxSetStartLine|0x0, //nolint:staticcheck
			ssd1xxxSetPageAddr, 0x00, byte(page),
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
