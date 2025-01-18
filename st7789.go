package display

import (
	"encoding/binary"
	"fmt"
	"time"

	"periph.io/x/conn/v3/gpio"

	"github.com/BeatGlow/display/conn"
)

const (
	st7789DefaultWidth  = 240
	st7789DefaultHeight = 240
)

// Registers (from st7789.pdf).
const (
	st7789NOP       = 0x00
	st7789SWRESET   = 0x01
	st7789RDDID     = 0x04
	st7789RDDST     = 0x09
	st7789RDDPM     = 0x0A
	st7789RDDMADCTL = 0x0B
	st7789RDDCOLMOD = 0x0C
	st7789RDDIM     = 0x0D
	st7789RDDSM     = 0x0E
	st7789RDDSDR    = 0x0F
	st7789SLPIN     = 0x10
	st7789SLPOUT    = 0x11 // Sleep Out
	st7789PTLON     = 0x12
	st7789NORON     = 0x13
	st7789INVOFF    = 0x20
	st7789INVON     = 0x21 // Display Inversion On
	st7789GAMSET    = 0x26
	st7789DISPOFF   = 0x28 // Display Off
	st7789DISPON    = 0x29 // Display On
	st7789CASET     = 0x2A // Column Address Set
	st7789RASET     = 0x2B // Row Address Set
	st7789RAMWR     = 0x2C // Memory Write
	st7789RAMRD     = 0x2E
	st7789PTLAR     = 0x30
	st7789VSCRDEF   = 0x33
	st7789TEOFF     = 0x34
	st7789TEON      = 0x35
	st7789MADCTL    = 0x36 // Memory Data Access Control
	st7789VSCRSADD  = 0x37
	st7789IDMOFF    = 0x38
	st7789IDMON     = 0x39
	st7789COLMOD    = 0x3A // Interface Pixel Format
	st7789RAMWRC    = 0x3C
	st7789RAMRDC    = 0x3E
	st7789TESCAN    = 0x44
	st7789RDTESCAN  = 0x45
	st7789WRDISBV   = 0x51
	st7789RDDISBV   = 0x52
	st7789WRCTRLD   = 0x53
	st7789RDCTRLD   = 0x54
	st7789WRCACE    = 0x55
	st7789RDCABC    = 0x56
	st7789WRCABCMB  = 0x5E
	st7789RDCABCMB  = 0x5F
	st7789RDABCSDR  = 0x68
	st7789RDID1     = 0xDA
	st7789RDID2     = 0xDB
	st7789RDID3     = 0xDC
	st7789RAMCTRL   = 0xB0
	st7789RGBCTRL   = 0xB1
	st7789PORCTRL   = 0xB2 // Porch Setting
	st7789FRCTRL1   = 0xB3
	st7789GCTRL     = 0xB7 // Gate Control
	st7789DGMEN     = 0xBA
	st7789VCOMS     = 0xBB // VCOM Setting
	st7789LCMCTRL   = 0xC0 // LCM Control
	st7789IDSET     = 0xC1
	st7789VDVVRHEN  = 0xC2 // VDV and VRH Command Enable
	st7789VRHS      = 0xC3 // VRH Set
	st7789VDVSET    = 0xC4 // VDV Set
	st7789VCMOFSET  = 0xC5 // VCOM Offset Set
	st7789FRCTR2    = 0xC6 // Frame Rate Control in Normal Mode
	st7789CABCCTRL  = 0xC7
	st7789REGSEL1   = 0xC8
	st7789REGSEL2   = 0xCA
	st7789PWMFRSEL  = 0xCC
	st7789PWCTRL1   = 0xD0 // Power Control 1
	st7789VAPVANEN  = 0xD2
	st7789CMD2EN    = 0xDF5A6902
	st7789PVGAMCTRL = 0xE0 // Positive Voltage Gamma Control
	st7789NVGAMCTRL = 0xE1 // Negative Voltage Gamma Control
	st7789DGMLUTR   = 0xE2
	st7789DGMLUTB   = 0xE3
	st7789GATECTRL  = 0xE4
	st7789PWCTRL2   = 0xE8
	st7789EQCTRL    = 0xE9
	st7789PROMCTRL  = 0xEC
	st7789PROMEN    = 0xFA
	st7789NVMSET    = 0xFC
	st7789PROMACT   = 0xFE
	MADCTL_MY       = 0x80
	MADCTL_MX       = 0x40
	MADCTL_MV       = 0x20
	MADCTL_ML       = 0x10
	MADCTL_RGB      = 0x00
	MADCTL_BGR      = 0x08
	MADCTL_MH       = 0x04
)

const (
	_                           byte = 1 << iota // D0: reserved
	_                                            // D1: reserved
	st7789DisplayDataLatchOrder                  // D2: MH
	st7789RGBOrder                               // D3: RGB
	st7789LineAddressOrder                       // D4: ML
	st7789PageColumnOrder                        // D5: MV
	st7789ColumnAddressOrder                     // D6: MX
	st7789PageAddressOrder                       // D7: MY
)

type st7789 struct {
	crgb16Display
}

func ST7789(c Conn, config *Config) (Display, error) {
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

	d := &st7789{
		crgb16Display: crgb16Display{
			display: display{
				c: c,
			},
		},
	}

	// Common initialization
	if err := d.init(config); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *st7789) String() string {
	return fmt.Sprintf("ST7789 %dx%d", d.buf.Rect.Dx(), d.buf.Rect.Dy())
}

// command shadows display.command
func (d *st7789) command(command byte, data ...byte) (err error) {
	if err = d.send([]byte{command}, true); err != nil {
		return
	}
	for _, data := range data {
		if err = d.send([]byte{data}, false); err != nil {
			return
		}
	}
	return
}

// commands shadows display.commands to call our local command implementation.
func (d *st7789) commands(commands [][]byte) (err error) {
	for _, command := range commands {
		if err = d.send([]byte{command[0]}, true); err != nil {
			return
		}
		for _, data := range command[1:] {
			if err = d.send([]byte{data}, false); err != nil {
				return
			}
		}
	}
	return
}

func (d *st7789) init(config *Config) (err error) {
	if config.Width == 0 {
		config.Width = st7789DefaultWidth
	}
	if config.Height == 0 {
		config.Height = st7789DefaultHeight
	}

	// init base
	if err = d.crgb16Display.init(config, binary.BigEndian); err != nil {
		return
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
	if err = d.command(st7789SLPOUT); err != nil { // Sleep Out
		return
	}
	time.Sleep(150 * time.Millisecond)

	if err = d.SetRotation(config.Rotation); err != nil {
		return
	}
	if err = d.commands([][]byte{
		//{0x36, 0x00},       // Memory Data Access Control (TODO(maze): fix rotation)
		{st7789COLMOD, 0x05},        // Interface Pixel Format: 8-bit data bus for 16-bit/pixel (RGB 5-6-5-bit input)
		{st7789PORCTRL, 0x0C, 0x0C}, // Porch Setting: default
		{st7789GCTRL, 0x35},         // Gate Control: 13.26V / -10.43V (default)
		{st7789VCOMS, 0x1A},         // VCOM Setting: 0.75V (default is 0x20 / 0.9V)
		{st7789LCMCTRL, 0x2C},       // LCM Control: default
		{st7789VDVVRHEN, 0x01},      // VDV and VRH Command Enable: default
		{st7789VRHS, 0x0B},          // VRH Set: default (4.1V+( vcom+vcom offset+vdv))
		{st7789VDVSET, 0x20},        // VDV Set: default (0V)
		{st7789VCMOFSET, 0x20},      // VCOM Offset Set: default (0V)
		{st7789FRCTR2, 0x0F},        // Frame Rate Control in Normal Mode: 60Hz (default)
		{st7789PWCTRL1, 0xA4, 0xA1}, // Power Control 1: default
		{st7789INVON},               // Partial Display Mode On
		{st7789PVGAMCTRL, 0x00, 0x19, 0x1E, 0x0A, 0x09, 0x15, 0x3D, 0x44, 0x51, 0x12, 0x03, 0x00, 0x3F, 0x3F}, // Positive Voltage Gamma Control: default
		{st7789NVGAMCTRL, 0x00, 0x18, 0x1E, 0x0A, 0x09, 0x25, 0x3F, 0x43, 0x52, 0x33, 0x03, 0x00, 0x3F, 0x3F}, // Negative Voltage Gamma Control: default
		{st7789DISPON}, // Display On
	}); err != nil {
		return
	}
	time.Sleep(100 * time.Millisecond)

	return
}

func (d *st7789) SetWindow(x0, y0, x1, y1 int) error {
	if x1 == 0 {
		x1 = d.buf.Rect.Max.X - 1
	}
	if y1 == 0 {
		y1 = d.buf.Rect.Max.Y - 1
	}
	if err := d.commands([][]byte{
		{st7789CASET, byte(x0 >> 8), byte(x0), byte(x1 >> 8), byte(x1)}, // Column address
		{st7789RASET, byte(y0 >> 8), byte(y0), byte(y1 >> 8), byte(y1)}, // Row address
		{st7789RAMWR}, // Write to RAM
	}); err != nil {
		return err
	}
	return nil
}

func (d *st7789) SetRotation(r Rotation) error {
	var value uint8
	switch r {
	case NoRotation:
		d.rowOffset = 0
		d.columnOffset = 0
	case Rotate90:
		value = st7789PageColumnOrder | st7789ColumnAddressOrder // MX|MV
	case Rotate180:
		value = st7789ColumnAddressOrder | st7789PageAddressOrder // MX|MY
	case Rotate270:
		value = st7789PageColumnOrder | st7789PageAddressOrder // MY|MV
	}
	return d.command(st7789MADCTL, value)
}

// Refresh sets the window to full screen and redraws using the internal frame buffer.
func (d *st7789) Refresh() error {
	if err := d.SetWindow(0, 0, 0, 0); err != nil {
		return err
	}
	const batchSize = 4096
	for i, l := 0, len(d.buf.Pix); i < l; i += batchSize {
		j := i + batchSize
		if j > l {
			j = l
		}
		if err := d.send(d.buf.Pix[i:j], false); err != nil {
			return err
		}
	}
	return nil
}
