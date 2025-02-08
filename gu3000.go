package display

import (
	"fmt"

	"github.com/BeatGlow/display/pixel"
)

const (
	gu3000DefaultWidth        = 256
	gu3000DefaultHeight       = 128
	gu3000BroadcastDAD        = 0xff // all displays
	gu3000BitImageWrite       = 0x46
	gu3000BoxBitImageWrite    = 0x42
	gu3000DisplayStartAddress = 0x53
	gu3000DisplaySynchronous  = 0x57
	gu3000BrightnessLevel     = 0x58
)

type gu3000 struct {
	monoDisplay
	dad uint8 // data address
}

func GU3000(conn Conn, config *Config) (Display, error) {
	d := &gu3000{
		monoDisplay: monoDisplay{
			baseDisplay: baseDisplay{
				c: conn,
			},
		},
		dad: gu3000BroadcastDAD,
	}

	if config.Width == 0 {
		config.Width = gu3000DefaultWidth
	}
	if config.Height == 0 {
		config.Height = gu3000DefaultHeight
	}

	if err := d.init(config); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *gu3000) init(config *Config) error {
	if err := d.monoDisplay.init(config); err != nil {
		return err
	}

	if err := d.flush(); err != nil {
		return err
	}

	return nil
}

func (d *gu3000) flush() (err error) {
	return d.c.Data(make([]byte, d.width*d.height)...)
}

func (d *gu3000) String() string {
	return fmt.Sprintf("GU3000 VFD %dx%d", d.width, d.height)
}

func (d *gu3000) Command(cmd byte, args ...byte) (err error) {
	return d.c.Data(append(
		[]byte{0x02, 0x44, d.dad, cmd},
		args...,
	)...)
}

func (d *gu3000) setDisplayStartAddress(addr uint16) (err error) {
	return d.Command(gu3000DisplayStartAddress, byte(addr&0xff), byte(addr>>8))
}

func (d *gu3000) SetContrast(level uint8) error {
	return d.Command(gu3000BrightnessLevel, level)
}

func (d *gu3000) Refresh() error {
	size := uint16(d.width) * uint16(d.height) / 8
	if err := d.Command(gu3000BitImageWrite, 0x00, 0x00, byte(size&0xff), byte(size>>8)); err != nil {
		return nil
	}
	return d.c.Data(d.Image.(*pixel.MonoVerticalLSBImage).Pix...)
}
