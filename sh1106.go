package display

import (
	"fmt"
)

const (
	sh1106DefaultWidth  = 128
	sh1106DefaultHeight = 64
	sh1106SetPageAddr   = 0xB0
)

type sh1106 struct {
	display
	pageSize int
	width    int
}

// SH1106 is a driver for the Sino Wealth SH1106 OLED display.
func SH1106(conn Conn, config *Config) (Display, error) {
	d := &sh1106{
		display: display{
			c: conn,
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

func (d *sh1106) String() string {
	return fmt.Sprintf("SH1106 %dx%d", d.buf.Rect.Dx(), d.buf.Rect.Dy())
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
		return fmt.Errorf("oled: SH1106 unsupported size %dx%d", config.Width, config.Height)
	}

	// init base
	if err = d.display.init(config); err != nil {
		return
	}

	// init display
	if err = d.command(
		setDisplayOff,
		setMemoryMode,
		setHighColumn, 0x80, 0xC8,
		setLowColumn, 0x10, 0x40,
		setSegmentRemap,
		setNormalDisplay,
		setMultiplexRatio, multiplexRatio,
		setDisplayAllOnResume,
		setDisplayOffset, displayOffset,
		setDisplayClockDiv, 0xF0,
		setPrecharge, 0x22,
		setComPins, 0x12,
		setVComDetect, 0x20,
		setChargePump, 0x14,
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
	for page := 0; page < d.pageSize; page++ {
		if err = d.command(
			sh1106SetPageAddr|byte(page&0x7),
			setLowColumn|0x2,
			setHighColumn|0x0, //nolint:staticcheck
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
