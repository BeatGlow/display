package display

import (
	"fmt"
	"image"
	"log"
)

const (
	ssd1326DefaultWidth  = 256
	ssd1326DefaultHeight = 64
)

const (
	ssd1326SetColumnAddress       = 0x15
	ssd1326WriteRAM               = 0x5C
	ssd1326SetRowAddress          = 0x75
	ssd1326SetRemap               = 0xA0
	ssd1326SetDisplayStartLine    = 0xA1
	ssd1326SetDisplayOffset       = 0xA2
	ssd1326SetDisplayNormal       = 0xA4
	ssd1326SetDisplayAllOn        = 0xA5
	ssd1326SetDisplayOff          = 0xA6
	ssd1326SetInverseDIsplay      = 0xA7
	ssd1326SetMuxRatio            = 0xA8
	ssd1326SetExitPartialDisplay  = 0xA9
	ssd1326SetFunction            = 0xAB
	ssd1326SetDislpayOff          = 0xAE
	ssd1326SetDisplayOn           = 0xAF
	ssd1326SetPhaseLength         = 0xB1
	ssd1326SetFrontClockDiv       = 0xB3
	ssd1326SetDisplayEnhancementA = 0xB4
	ssd1326SetGPIO                = 0xB5
	ssd1326SetSecondPrecharge     = 0xB6
	ssd1326SetDefaultGrayscale    = 0xB9
	ssd1326SetPrechargeVoltage    = 0xBB
	ssd1326SetVCOMHVoltage        = 0xBE
	ssd1326SetContrast            = 0xC1
	ssd1326SetMasterCurrent       = 0xC7
	ssd1326SetDisplayEnhancementB = 0xD1
	ssd1326SetCommandLock         = 0xFD
)

var (
	ssd1326SupportedSizes = []image.Point{
		image.Pt(256, 64),
		image.Pt(256, 48),
		image.Pt(256, 32),
		image.Pt(128, 64),
		image.Pt(128, 48),
		image.Pt(128, 32),
		image.Pt(64, 64),
		image.Pt(64, 48),
		image.Pt(64, 32),
	}
)

type ssd1326 struct {
	grayDisplay
}

// SSD1326 is a driver for Solomon Systech SSD1326 OLED display.
func SSD1326(conn Conn, config *Config) (Display, error) {
	d := &ssd1326{
		grayDisplay: grayDisplay{
			display: display{
				c: conn,
			},
		},
	}

	if config.Width == 0 {
		config.Width = ssd1326DefaultWidth
	}
	if config.Height == 0 {
		config.Height = ssd1326DefaultHeight
	}

	if err := d.init(config); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *ssd1326) String() string {
	return fmt.Sprintf("SSD1326 %dx%d", d.buf.Rect.Dx(), d.buf.Rect.Dy())
}

func (d *ssd1326) init(config *Config) (err error) {
	var supported bool
	for _, size := range ssd1326SupportedSizes {
		if supported = size.X == config.Width && size.Y == config.Height; supported {
			break
		}
	}
	if !supported {
		return fmt.Errorf("oled: SSD1326 unsupported size %dx%d", config.Width, config.Height)
	}
	// init base
	if err = d.grayDisplay.init(config); err != nil {
		return
	}
	log.Printf("SSD1326 with %d bytes buffer", len(d.buf.Pix))

	// init display
	if err = d.commands(
		[]byte{ssd1326SetCommandLock, 0x12}, // Unlock IC
		[]byte{ssd1326SetDisplayAllOn},
		[]byte{ssd1326SetMuxRatio, byte(config.Height - 1)},
		[]byte{ssd1326SetDisplayOffset, 0x00},
		[]byte{ssd1326SetDisplayStartLine, 0x00},
		[]byte{ssd1326SetDisplayOff},
		[]byte{ssd1326SetFrontClockDiv, 0x80},
		[]byte{ssd1326SetRemap, 0x40},
		[]byte{ssd1326SetDisplayNormal},
		[]byte{ssd1326SetDisplayOn},
	); err != nil {
		return
	}

	d.columnOffset = (480 - d.buf.Rect.Max.X) >> 1
	if err = d.SetWindow(d.buf.Rect); err != nil {
		return
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

func (d *ssd1326) SetContrast(level uint8) error {
	return d.command(ssd1326SetContrast, level)
}

func (d *ssd1326) SetWindow(r image.Rectangle) error {
	if !r.In(d.buf.Rect) {
		return ErrBounds
	}

	var (
		left        = r.Min.X
		right       = r.Max.X
		width       = byte(right - left)
		start       = byte(d.columnOffset + left)
		columnStart = byte(start >> 2)
		columnEnd   = (start + width>>2) - 1
	)
	return d.commands(
		[]byte{ssd1326SetColumnAddress, byte(columnStart), byte(columnEnd)}, // Set column address
		[]byte{ssd1326SetRowAddress, byte(r.Min.Y), byte(r.Max.Y - 1)},      // Set row address
		[]byte{ssd1326WriteRAM}, // Enable MCU to write data into RAM
	)
}

// Refresh needs to be duplicated here, otherwise we can't access the gray buf.
func (d *ssd1326) Refresh() error {
	if err := d.SetWindow(d.buf.Rect); err != nil {
		return err
	}
	return d.c.Send(d.buf.Pix, false)
}

// Interface checks
var (
	_ Display = (*ssd1326)(nil)
)
