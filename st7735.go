package display

import (
	"encoding/binary"
	"fmt"
	"log"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/physic"

	"github.com/BeatGlow/display/conn"
	"github.com/BeatGlow/display/pixel"
)

const (
	st7735DefaultWidth  = 128
	st7735DefaultHeight = 160
)

// Registers (from st7735.pdf).
const (
	st7735NOP      = 0x00
	st7735SWRESET  = 0x01
	st7735RDDID    = 0x04
	st7735RDDST    = 0x09
	st7735SLPIN    = 0x10
	st7735SLPOUT   = 0x11
	st7735PTLON    = 0x12
	st7735NORON    = 0x13
	st7735INVOFF   = 0x20
	st7735INVON    = 0x21
	st7735DISPOFF  = 0x28
	st7735DISPON   = 0x29
	st7735CASET    = 0x2A
	st7735RASET    = 0x2B
	st7735RAMWR    = 0x2C
	st7735RAMRD    = 0x2E
	st7735PTLAR    = 0x30
	st7735COLMOD   = 0x3A
	st7735MADCTL   = 0x36
	MADCTL_MY      = 0x80
	MADCTL_MX      = 0x40
	MADCTL_MV      = 0x20
	MADCTL_ML      = 0x10
	MADCTL_RGB     = 0x00
	MADCTL_BGR     = 0x08
	MADCTL_MH      = 0x04
	st7735RDID1    = 0xDA
	st7735RDID2    = 0xDB
	st7735RDID3    = 0xDC
	st7735RDID4    = 0xDD
	st7735FRMCTR1  = 0xB1
	st7735FRMCTR2  = 0xB2
	st7735FRMCTR3  = 0xB3
	st7735INVCTR   = 0xB4
	st7735DISSET5  = 0xB6
	st7735PWCTR1   = 0xC0
	st7735PWCTR2   = 0xC1
	st7735PWCTR3   = 0xC2
	st7735PWCTR4   = 0xC3
	st7735PWCTR5   = 0xC4
	st7735VMCTR1   = 0xC5
	st7735PWCTR6   = 0xFC
	st7735GMCTRP1  = 0xE0
	st7735GMCTRN1  = 0xE1
	st7735VSCRDEF  = 0x33
	st7735VSCRSADD = 0x37
)

// Memory Data Access Control (MADCTL) bit fields.
const (
	_                           byte = 1 << iota // D0: reserved
	_                                            // D1: reserved
	st7735DisplayDataLatchOrder                  // D2: MH
	st7735RGBOrder                               // D3: RGB
	st7735LineAddressOrder                       // D4: ML
	st7735PageColumnOrder                        // D5: MV
	st7735ColumnAddressOrder                     // D6: MX
	st7735PageAddressOrder                       // D7: MY
)

type st7735 struct {
	crgb16Display
	backlight gpio.PinOut
}

func ST7735(c Conn, config *Config) (Display, error) {
	// Update mode and speed
	if spi, ok := c.(SPI); ok {
		spi.SetDataLow(false)
		if err := spi.SetMode(conn.SPIMode3); err != nil {
			return nil, err
		}
		if err := spi.SetMaxSpeed(40000000); err != nil {
			return nil, err
		}
	}

	d := &st7735{
		crgb16Display: crgb16Display{
			baseDisplay: baseDisplay{c: c},
		},
		backlight: config.Backlight,
	}

	// Common initialization
	if err := d.init(config); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *st7735) String() string {
	bounds := d.Bounds()
	return fmt.Sprintf("ST7735 %dx%d", bounds.Dx(), bounds.Dy())
}

// command shadows display.command
func (d *st7735) command(cmnd byte, data ...byte) (err error) {
	if err = d.command(cmnd); err != nil {
		return
	}
	for _, data := range data {
		if err = d.data(data); err != nil {
			return
		}
	}
	return
}

// commands shadows display.commands to call our local command implementation.
func (d *st7735) commands(commands [][]byte) (err error) {
	for _, command := range commands {
		if err = d.command(command[0]); err != nil {
			return
		}
		for _, data := range command[1:] {
			if err = d.data(data); err != nil {
				return
			}
		}
	}
	return
}

func (d *st7735) init(config *Config) (err error) {
	if config.Width == 0 {
		if config.Rotation == Rotate90 || config.Rotation == Rotate270 {
			config.Width = st7735DefaultHeight
		} else {
			config.Width = st7735DefaultWidth
		}
	}
	d.width = config.Width

	if config.Height == 0 {
		if config.Rotation == Rotate90 || config.Rotation == Rotate270 {
			config.Width = st7735DefaultWidth
		} else {
			config.Height = st7735DefaultHeight
		}
	}
	d.height = config.Height

	if (config.Rotation == NoRotation || config.Rotation == Rotate180) && (config.Width > 240 || config.Height > 320) {
		return fmt.Errorf("st7735: invalid size %dx%d, maximum size is 240x320 at %s rotation", config.Width, config.Height, config.Rotation)
	} else if (config.Rotation == Rotate90 || config.Rotation == Rotate270) && (config.Width > 320 || config.Height > 240) {
		return fmt.Errorf("st7735: invalid size %dx%d, maximum size is 320x240 at %s rotation", config.Width, config.Height, config.Rotation)
	}

	// init base
	if err = d.crgb16Display.init(config, binary.BigEndian); err != nil {
		return
	}

	if config.Backlight != nil {
		d.backlight = config.Backlight
		if err = d.backlight.PWM(gpio.DutyMax, 2000*physic.Hertz); err != nil {
			return
		}
	} else {
		log.Println("st7735: no backlight control")
	}

	// reset the device.
	if err = d.c.Reset(gpio.High); err != nil {
		return
	}
	time.Sleep(100 * time.Millisecond)
	if err = d.c.Reset(gpio.Low); err != nil {
		return
	}
	time.Sleep(100 * time.Millisecond)
	if err = d.c.Reset(gpio.High); err != nil {
		return
	}

	// init display
	time.Sleep(10 * time.Millisecond)
	if err = d.command(st7735SWRESET); err != nil { // Sleep Out
		return
	}
	time.Sleep(150 * time.Millisecond)
	if err = d.command(st7735SLPOUT); err != nil { // Sleep Out
		return
	}
	time.Sleep(150 * time.Millisecond)

	if err = d.commands([][]byte{
		{st7735FRMCTR1, 0x01, 0x2C, 0x2D},
		{st7735FRMCTR2, 0x01, 0x2C, 0x2D},
		{st7735FRMCTR3, 0x01, 0x2C, 0x2D, 0x01, 0x2C, 0x2D},
		{st7735INVCTR, 0x07},
		{st7735PWCTR1, 0xA2, 0x02, 0x84},
		{st7735PWCTR2, 0xC5},
		{st7735PWCTR3, 0x0A, 0x00},
		{st7735PWCTR4, 0x8A, 0x2A},
		{st7735PWCTR5, 0x8A, 0xEE},
		{st7735VMCTR1, 0x0E},
		{st7735COLMOD, 0x05}, // 16-bits per pixel
		{st7735GMCTRP1, 0x02, 0x1C, 0x07, 0x12, 0x37, 0x32, 0x29, 0x2D, 0x29, 0x25, 0x2B, 0x39, 0x00, 0x01, 0x03, 0x10},
		{st7735GMCTRN1, 0x03, 0x1D, 0x07, 0x06, 0x2E, 0x2C, 0x29, 0x2D, 0x2E, 0x2E, 0x37, 0x3F, 0x00, 0x00, 0x02, 0x10},
		{st7735NORON},
		{st7735PWCTR5, 0x8A, 0xEE},
		{st7735VMCTR1, 0x0E},
		{st7735DISPON},
	}); err != nil {
		return
	}
	time.Sleep(100 * time.Millisecond)

	if err = d.SetRotation(config.Rotation); err != nil {
		return
	}
	if err = d.SetContrast(0xFF); err != nil {
		return
	}

	return
}

func (d *st7735) Close() error {
	if err := d.Show(false); err != nil {
		_ = d.c.Close()
		return err
	}
	return d.c.Close()
}

func (d *st7735) Show(show bool) error {
	var command = byte(st7735DISPOFF)
	if show {
		command = byte(st7735DISPON)
	}
	return d.command(command)
}

func (d *st7735) SetContrast(level uint8) error {
	if d.backlight == nil {
		return nil
	}
	//if level == 0xFF {
	//	return d.backlight.Out(gpio.High)
	//}
	const (
		step = gpio.DutyMax / 0xFF
		rate = 2 * physic.KiloHertz
	)
	log.Printf("st7735: backlight duty cycle to %s at %s", step*gpio.Duty(level), rate)
	return d.backlight.PWM(step*gpio.Duty(level), rate)
}

func (d *st7735) SetRotation(rotation Rotation) error {
	rotation &= 3

	var madctl byte
	switch rotation {
	case NoRotation:
		madctl = 0
	case Rotate90:
		madctl = st7735ColumnAddressOrder | st7735PageColumnOrder
	case Rotate180:
		madctl = st7735ColumnAddressOrder | st7735PageAddressOrder
	case Rotate270:
		madctl = st7735PageAddressOrder | st7735PageColumnOrder
	}

	d.rotation = rotation
	log.Printf("madctl %s -> %#02x", rotation, madctl)
	return d.command(st7735MADCTL, madctl)
}

func (d *st7735) SetWindow(x0, y0, x1, y1 int) error {
	if x1 == 0 {
		x1 = d.width - 1
	}
	if y1 == 0 {
		y1 = d.height - 1
	}
	if d.rotation == Rotate90 || d.rotation == Rotate270 {
		x0 += d.rowOffset
		y0 += d.colOffset
		x1 += d.rowOffset
		y1 += d.colOffset
	} else {
		x0 += d.colOffset
		y0 += d.rowOffset
		x1 += d.colOffset
		y1 += d.rowOffset
	}
	log.Printf("st7735 window rotation %s (%d,%d)-(%d,%d)", d.rotation, x0, y0, x1, y1)
	if err := d.commands([][]byte{
		{st7735CASET, byte(x0 >> 8), byte(x0), byte(x1 >> 8), byte(x1)}, // Column address
		{st7735RASET, byte(y0 >> 8), byte(y0), byte(y1 >> 8), byte(y1)}, // Row address
		{st7735RAMWR}, // Write to RAM
	}); err != nil {
		return err
	}
	return nil
}

// Refresh sets the window to full screen and redraws using the internal frame buffer.
func (d *st7735) Refresh() error {
	if err := d.SetWindow(0, 0, 0, 0); err != nil {
		return err
	}
	const batchSize = 4096

	pix := d.Image.(*pixel.CRGB16Image).Pix
	for i, l := 0, len(pix); i < l; i += batchSize {
		j := i + batchSize
		if j > l {
			j = l
		}
		if err := d.data(pix[i:j]...); err != nil {
			return err
		}
	}
	return nil
}
