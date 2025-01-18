package display

import (
	"fmt"
	"image"
)

const (
	ssd1322DefaultWidth  = 256
	ssd1322DefaultHeight = 64
)

const (
	ssd1322SetColumnAddress       = 0x15
	ssd1322WriteRAM               = 0x5C
	ssd1322SetRowAddress          = 0x75
	ssd1322SetRemap               = 0xA0
	ssd1322SetDisplayStartLine    = 0xA1
	ssd1322SetDisplayOffset       = 0xA2
	ssd1322SetDisplayNormal       = 0xA4
	ssd1322SetDisplayAllOn        = 0xA5
	ssd1322SetDisplayOff          = 0xA6
	ssd1322SetInverseDIsplay      = 0xA7
	ssd1322SetExitPartialDisplay  = 0xA9
	ssd1322SetFunction            = 0xAB
	ssd1322SetDislpayOff          = 0xAE
	ssd1322SetDisplayOn           = 0xAF
	ssd1322SetPhaseLength         = 0xB1
	ssd1322SetFrontClockDiv       = 0xB3
	ssd1322SetDisplayEnhancementA = 0xB4
	ssd1322SetGPIO                = 0xB5
	ssd1322SetSecondPrecharge     = 0xB6
	ssd1322SetDefaultGrayscale    = 0xB9
	ssd1322SetPrechargeVoltage    = 0xBB
	ssd1322SetVCOMHVoltage        = 0xBE
	ssd1322SetContrast            = 0xC1
	ssd1322SetMasterCurrent       = 0xC7
	ssd1322SetMultiplexRatio      = 0xCA
	ssd1322SetDisplayEnhancementB = 0xD1
	ssd1322SetCommandLock         = 0xFD
)

var (
	ssd1322SupportedSizes = []image.Point{
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

type ssd1322 struct {
	grayDisplay
}

// SSD1322 is a driver for Solomon Systech SSD1322 OLED display.
func SSD1322(conn Conn, config *Config) (Display, error) {
	d := &ssd1322{
		grayDisplay: grayDisplay{
			display: display{
				c: conn,
			},
		},
	}

	if config.Width == 0 {
		config.Width = ssd1322DefaultWidth
	}
	if config.Height == 0 {
		config.Height = ssd1322DefaultHeight
	}

	if err := d.init(config); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *ssd1322) String() string {
	return fmt.Sprintf("SSD1322 %dx%d", d.buf.Rect.Dx(), d.buf.Rect.Dy())
}

func (d *ssd1322) init(config *Config) (err error) {
	var supported bool
	for _, size := range ssd1322SupportedSizes {
		if supported = size.X == config.Width && size.Y == config.Height; supported {
			break
		}
	}
	if !supported {
		return fmt.Errorf("oled: SSD1322 unsupported size %dx%d", config.Width, config.Height)
	}

	// init base
	if err = d.grayDisplay.init(config); err != nil {
		return
	}

	// init display
	if err = d.commands(
		[]byte{ssd1322SetCommandLock, 0x12},               // Unlock IC
		[]byte{ssd1322SetDisplayNormal},                   // Display normal
		[]byte{ssd1322SetFrontClockDiv, 0xF2},             // Display divide clockratio/freq
		[]byte{ssd1322SetMultiplexRatio, 0x3F},            // Set MUX ratio
		[]byte{ssd1322SetDisplayOffset, 0x00},             // Display offset
		[]byte{ssd1322SetDisplayStartLine, 0x00},          // Display start Line
		[]byte{ssd1322SetRemap, 0x14, 0x11},               // Set remap & dual COM Line
		[]byte{ssd1322SetGPIO, 0x00},                      // Set GPIO (disabled)
		[]byte{ssd1322SetFunction, 0x01},                  // Function select (internal Vdd)
		[]byte{ssd1322SetDisplayEnhancementA, 0xA0, 0xFD}, // Display enhancement A (External VSL)
		[]byte{ssd1322SetMasterCurrent, 0x0F},             // Master contrast (reset)
		[]byte{ssd1322SetDefaultGrayscale},                // Set default greyscale table
		[]byte{ssd1322SetPhaseLength, 0xF0},               // Phase length
		[]byte{ssd1322SetDisplayEnhancementB, 0x82, 0x20}, // Display enhancement B (reset)
		[]byte{ssd1322SetPrechargeVoltage, 0x0D},          // Pre-charge voltage
		[]byte{ssd1322SetSecondPrecharge, 0x08},           // 2nd precharge period
		[]byte{ssd1322SetVCOMHVoltage, 0x00},              // Set VcomH
		[]byte{ssd1322SetDisplayOff},                      // Display off (all pixels off)
		[]byte{ssd1322SetExitPartialDisplay},              // Exit partial display
		[]byte{ssd1322SetDisplayOn},                       // Display on
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

func (d *ssd1322) SetContrast(level uint8) error {
	return d.command(ssd1322SetContrast, level)
}

func (d *ssd1322) SetWindow(r image.Rectangle) error {
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
		[]byte{ssd1322SetColumnAddress, byte(columnStart), byte(columnEnd)}, // Set column address
		[]byte{ssd1322SetRowAddress, byte(r.Min.Y), byte(r.Max.Y - 1)},      // Set row address
		[]byte{ssd1322WriteRAM}, // Enable MCU to write data into RAM
	)
}

// Refresh needs to be duplicated here, otherwise we can't access the gray buf.
func (d *ssd1322) Refresh() error {
	if err := d.SetWindow(d.buf.Rect); err != nil {
		return err
	}
	return d.c.Send(d.buf.Pix, false)
}

// Interface checks
var (
	_ Display = (*ssd1322)(nil)
)
