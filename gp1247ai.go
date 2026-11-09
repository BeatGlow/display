package display

import (
	"fmt"
	"image"
	"time"

	"github.com/BeatGlow/display/pixel"
)

const (
	gp1247AIDefaultWidth     = 253
	gp1247AIDefaultHeight    = 63
	gp1247AIGRAMWidth        = 256
	gp1247AIWriteGRAM        = 0xF0
	gp1247AISetDisplayMode   = 0x80
	gp1247AISetDimming       = 0xA0
	gp1247AISoftwareReset    = 0xAA
	gp1247AISetInternalSpeed = 0xB1
	gp1247AIDisplayOffset    = 0xC0
	gp1247AISetVFDMode       = 0xCC
	gp1247AISetDisplayArea   = 0xE0
	gp1247AIClearGRAM        = 0x55
	gp1247AIStandby          = 0x61
	gp1247AIWakeUp           = 0x6D
	gp1247AIOscillation      = 0x78
)

type gp1247ai struct {
	monoDisplay
	pageSize int
}

// GP1247AI drives the 253x63 Futaba GP1247AI VFD through its SPI command
// interface.
func GP1247AI(conn Conn, config *Config) (Display, error) {
	d := &gp1247ai{monoDisplay: monoDisplay{baseDisplay: baseDisplay{c: conn}}}
	if config.Width == 0 {
		config.Width = gp1247AIDefaultWidth
	}
	if config.Height == 0 {
		config.Height = gp1247AIDefaultHeight
	}
	if config.Width < 1 || config.Width > gp1247AIGRAMWidth || config.Height < 1 || config.Height > 64 {
		return nil, fmt.Errorf("gp1247ai: invalid dimensions %dx%d; controller memory supports 1..256 by 1..64", config.Width, config.Height)
	}
	d.width, d.height = config.Width, config.Height
	d.pageSize = (config.Height + 7) / 8
	if err := d.init(config); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *gp1247ai) String() string {
	return fmt.Sprintf("GP1247AI VFD %dx%d", d.width, d.height)
}

func (d *gp1247ai) Close() error {
	return d.c.Close()
}

func (d *gp1247ai) command(command byte, args ...byte) error {
	for i, arg := range args {
		args[i] = rev8tab[arg]
	}
	return d.c.Command(rev8tab[command], args...)
}

func (d *gp1247ai) data(data []byte) error {
	data = append([]byte(nil), data...)
	for i, b := range data {
		data[i] = rev8tab[b]
	}
	return d.c.Data(data...)
}

func (d *gp1247ai) init(config *Config) error {
	if err := d.monoDisplay.init(config); err != nil {
		return err
	}
	if err := d.command(gp1247AISoftwareReset); err != nil {
		return err
	}
	time.Sleep(time.Millisecond)
	if err := d.commands([][]byte{
		{gp1247AIOscillation, 0x08},
		{gp1247AISetVFDMode, 0x05, 0x00},
		{gp1247AISetDisplayArea, 0xFC, 0x3E, 0x00, 0x20, 0x80, 0x80, 0x80},
		{gp1247AISetInternalSpeed, 0x20, 0x3F, 0x00, 0x01},
		{gp1247AISetDimming, 0x02, 0x00},
		{gp1247AIClearGRAM},
	}...); err != nil {
		return err
	}
	time.Sleep(15 * time.Millisecond)
	if err := d.commands([][]byte{
		{gp1247AIDisplayOffset, 0x00, 0x00},
		{gp1247AISetDisplayMode, 0x80},
	}...); err != nil {
		return err
	}
	if err := d.Show(true); err != nil {
		return err
	}
	return d.Refresh()
}

func (d *gp1247ai) commands(commands ...[]byte) error {
	for _, command := range commands {
		if err := d.command(command[0], command[1:]...); err != nil {
			return err
		}
	}
	return nil
}

func (d *gp1247ai) Show(show bool) error {
	if !show {
		return d.command(gp1247AIStandby)
	}
	if err := d.command(gp1247AIWakeUp); err != nil {
		return err
	}
	time.Sleep(time.Millisecond)
	return d.command(gp1247AISetDisplayMode, 0x80)
}

func (d *gp1247ai) SetContrast(level uint8) error {
	value := uint16(level) * 4
	return d.command(gp1247AISetDimming, byte(value>>8), byte(value))
}

func (d *gp1247ai) Refresh() error {
	pix := d.Image.(*pixel.MonoVerticalLSBImage).Pix
	page := make([]byte, gp1247AIGRAMWidth)
	for y := 0; y < d.pageSize; y++ {
		if err := d.command(gp1247AIWriteGRAM, 0, byte(y*8), 0x07); err != nil {
			return err
		}
		clear(page)
		copy(page, pix[y*d.width:(y+1)*d.width])
		if err := d.data(page); err != nil {
			return err
		}
	}
	return nil
}

func (d *gp1247ai) SetRotation(rotation Rotation) error {
	return d.monoDisplay.SetRotation(rotation)
}

func (d *gp1247ai) Bounds() image.Rectangle {
	return rotationBounds(d.Image.Bounds(), d.rotation)
}
