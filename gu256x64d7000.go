package display

import (
	"fmt"

	"github.com/BeatGlow/display/pixel"
)

const (
	gu256x64d7000Width  = 256
	gu256x64d7000Height = 64

	gu256x64d7000Initialize       = 0x1b
	gu256x64d7000Clear            = 0x0c
	gu256x64d7000ScreenSaver      = 0x1f
	gu256x64d7000Brightness       = 0x58
	gu256x64d7000BitImageGroup    = 0x28
	gu256x64d7000BitImage         = 0x66
	gu256x64d7000RealTimeBitImage = 0x11
)

// GU256X64D7000 is a driver for Noritake GU256X64D-7000 VFD displays using
// either the 8-bit parallel interface or its asynchronous serial interface.
//
// For serial mode, pass a connection from OpenUART configured to match the
// display's baud rate. The display's serial SIN input is RS-232 level.
// For parallel mode, its /RS pin is unused, so command bytes are sent through
// the data interface along with bitmap data.
func GU256X64D7000(conn Conn, config *Config) (Display, error) {
	d := &gu256x64d7000{
		monoDisplay: monoDisplay{
			baseDisplay: baseDisplay{c: conn},
		},
	}

	if config.Width == 0 {
		config.Width = gu256x64d7000Width
	}
	if config.Height == 0 {
		config.Height = gu256x64d7000Height
	}

	if err := d.init(config); err != nil {
		return nil, err
	}
	return d, nil
}

type gu256x64d7000 struct {
	monoDisplay
}

func (d *gu256x64d7000) String() string {
	return fmt.Sprintf("GU256X64D-7000 VFD %dx%d", d.width, d.height)
}

func (d *gu256x64d7000) init(config *Config) error {
	if err := d.monoDisplay.init(config); err != nil {
		return err
	}
	// Initialize the controller and clear its display memory. The GU-7000
	// protocol is carried entirely over the data port on this module.
	if err := d.data(gu256x64d7000Initialize, 0x40, gu256x64d7000Clear); err != nil {
		return err
	}
	return d.Refresh()
}

func (d *gu256x64d7000) data(data ...byte) error {
	return d.c.Data(data...)
}

func (d *gu256x64d7000) Show(show bool) error {
	mode := byte(0)
	if show {
		mode = 1
	}
	if err := d.data(gu256x64d7000ScreenSaver, 0x28, 0x61, 0x40, mode); err != nil {
		return err
	}
	if show {
		return d.Refresh()
	}
	return nil
}

func (d *gu256x64d7000) SetContrast(level uint8) error {
	// The controller accepts brightness levels 1 (12.5%) through 8 (100%).
	value := byte((uint16(level)*7)/255 + 1)
	return d.data(gu256x64d7000ScreenSaver, gu256x64d7000Brightness, value)
}

func (d *gu256x64d7000) Refresh() error {
	image := d.Image.(*pixel.MonoVerticalLSBImage)
	// The GU-7000 bitmap command defines height in 8-dot blocks. The
	// MonoVerticalLSBImage buffer has the matching column-major block layout.
	if err := d.data(
		gu256x64d7000ScreenSaver,
		gu256x64d7000BitImageGroup,
		gu256x64d7000BitImage,
		gu256x64d7000RealTimeBitImage,
		byte(d.width), byte(d.width>>8),
		byte(d.height/8), byte((d.height/8)>>8),
		0x01,
	); err != nil {
		return err
	}
	return d.data(image.Pix...)
}
