package display

import (
	"fmt"
)

const (
	ssd1306DefaultWidth  = 128
	ssd1306DefaultHeight = 64
	ssd1306SetPageAddr   = 0xB0
)

type ssd1306 struct {
	display
	pageSize int
	width    int
	colStart byte
	colEnd   byte
}

func SSD1306(conn Conn, config *Config) (Display, error) {
	d := &ssd1306{
		display: display{
			c: conn,
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

func (d *ssd1306) String() string {
	return fmt.Sprintf("SSD1306 %dx%d", d.buf.Rect.Dx(), d.buf.Rect.Dy())
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
	if err = d.display.init(config); err != nil {
		return
	}

	// init display
	if err = d.command(
		setDisplayOff,
		setDisplayClockDiv, displayClockDiv,
		setMultiplexRatio, multiplexRatio,
		setDisplayOffset, 0x00,
		setStartLine,
		setChargePump, 0x14,
		setMemoryMode, 0x00,
		setSegmentRemap,
		setComScanDec,
		setComPins, comPins,
		setPrecharge, 0xF1,
		setVComDetect, 0x40,
		setDisplayAllOnResume,
		setNormalDisplay,
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
	for page := 0; page < d.pageSize; page++ {
		if err = d.command(
			setColumnAddr, d.colStart, d.colEnd-1,
			setStartLine|0x0, //nolint:staticcheck
			setPageAddr, 0x00, byte(page),
		); err != nil {
			return
		}
		var (
			off = page * d.width
			end = off + d.width
		)
		if err := d.send(d.buf.Pix[off:end], false); err != nil {
			return err
		}
	}
	return nil
}
