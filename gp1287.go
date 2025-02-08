package display

import (
	"fmt"
	"time"

	"github.com/BeatGlow/display/pixel"
)

const (
	gp1278DefaultWidth  = 256
	gp1278DefaultHeight = 50
)

const (
	gp1278ClearGRAM             = 0x55
	gp127Standby                = 0x61
	gp127WakeUp                 = 0x62
	gp1278SetDisplayMode        = 0x80
	gp1278SetDimming            = 0xA0
	gp1278SoftwareReset         = 0xAA
	gp1278SetInternalSpeed      = 0xB1
	gp1278DisplayPositionOffset = 0xC0
	gp1278SetVFDMode            = 0xCC
	gp1278SetDisplayArea        = 0xE0
	gp1278WriteGRAM             = 0xF0
	gp1278NormalDisplay         = 0x00
	gp1278InvertDisplay         = 0x01
)

type gp1278 struct {
	monoDisplay
	pageSize int
}

// GP1278 is a driver for GP1287AI/BI VFD displays.
func GP1278(conn Conn, config *Config) (Display, error) {
	d := &gp1278{
		monoDisplay: monoDisplay{
			baseDisplay: baseDisplay{
				c: conn,
			},
		},
	}

	if config.Width == 0 {
		config.Width = gp1278DefaultWidth
	}
	if config.Height == 0 {
		config.Height = gp1278DefaultHeight
	}

	d.height = config.Height
	d.width = config.Width
	d.pageSize = config.Height >> 3

	if err := d.init(config); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *gp1278) String() string {
	return fmt.Sprintf("GP1278 VFD %dx%d", d.width, d.height)
}

func (d *gp1278) Close() error {
	return d.c.Close()
}

func (d *gp1278) command(command byte, args ...byte) (err error) {
	// Swap bits, since the driver expects LSB first.
	for i, v := range args {
		args[i] = rev8tab[v]
	}
	return d.c.Command(rev8tab[command], args...)
}

func (d *gp1278) commands(commands ...[]byte) (err error) {
	for _, command := range commands {
		if err = d.command(command[0], command[1:]...); err != nil {
			return
		}
	}
	return
}

func (d *gp1278) data(data ...byte) error {
	// Swap bits, since the driver expects LSB first.
	for i, v := range data {
		data[i] = rev8tab[v]
	}
	return d.c.Data(data...)
}

func (d *gp1278) init(config *Config) (err error) {
	// init base
	if err = d.monoDisplay.init(config); err != nil {
		return
	}

	// init display
	if err = d.commands([][]byte{
		{gp1278SoftwareReset},
		{gp1278SetVFDMode, 0x02, 0x00},
		{gp1278SetDisplayArea, 0xFF, 0x31, 0x00, 0x20, 0x00, 0x00, 0x80},
		{gp1278SetInternalSpeed, 0x20, 0x3F, 0x00, 0x01},
	}...); err != nil {
		return
	}

	time.Sleep(10 * time.Millisecond)

	if err = d.commands([][]byte{
		{gp1278SetDimming, 0x01, 0xFF}, // [0x000...0x3ff]
		{gp1278ClearGRAM},
	}...); err != nil {
		return
	}

	time.Sleep(15 * time.Millisecond)

	if err = d.commands([][]byte{
		{gp1278DisplayPositionOffset, 0x00, 0x00},
		{gp1278SetDisplayMode, gp1278NormalDisplay},
	}...); err != nil {
		return
	}

	return d.Refresh()
}

func (d *gp1278) Show(show bool) (err error) {
	if show {
		if err = d.command(gp127WakeUp); err != nil {
			return
		}
		time.Sleep(1 * time.Millisecond) // Wait for internal oscillator to stabilize
		return d.command(gp1278SetDisplayMode, gp1278NormalDisplay)
	} else {
		return d.command(gp127Standby)
	}
}

func (d *gp1278) SetContrast(level uint8) error {
	level16 := uint16(level) * 4 // in range [0..1023]
	return d.command(gp1278SetDimming, uint8(level16>>8), uint8(level16))
}

func (d *gp1278) Refresh() error {
	pix := d.Image.(*pixel.MonoVerticalLSBImage).Pix
	for page := 0; page < d.pageSize; page++ {
		offset := uint8(page) * 8
		if err := d.command(
			gp1278WriteGRAM,
			0x00,         // x
			byte(offset), // y
			0x07,         // columns (all)
		); err != nil {
			return err
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

// rev8tab from math/bits/bits_tables.go
const rev8tab = "" +
	"\x00\x80\x40\xc0\x20\xa0\x60\xe0\x10\x90\x50\xd0\x30\xb0\x70\xf0" +
	"\x08\x88\x48\xc8\x28\xa8\x68\xe8\x18\x98\x58\xd8\x38\xb8\x78\xf8" +
	"\x04\x84\x44\xc4\x24\xa4\x64\xe4\x14\x94\x54\xd4\x34\xb4\x74\xf4" +
	"\x0c\x8c\x4c\xcc\x2c\xac\x6c\xec\x1c\x9c\x5c\xdc\x3c\xbc\x7c\xfc" +
	"\x02\x82\x42\xc2\x22\xa2\x62\xe2\x12\x92\x52\xd2\x32\xb2\x72\xf2" +
	"\x0a\x8a\x4a\xca\x2a\xaa\x6a\xea\x1a\x9a\x5a\xda\x3a\xba\x7a\xfa" +
	"\x06\x86\x46\xc6\x26\xa6\x66\xe6\x16\x96\x56\xd6\x36\xb6\x76\xf6" +
	"\x0e\x8e\x4e\xce\x2e\xae\x6e\xee\x1e\x9e\x5e\xde\x3e\xbe\x7e\xfe" +
	"\x01\x81\x41\xc1\x21\xa1\x61\xe1\x11\x91\x51\xd1\x31\xb1\x71\xf1" +
	"\x09\x89\x49\xc9\x29\xa9\x69\xe9\x19\x99\x59\xd9\x39\xb9\x79\xf9" +
	"\x05\x85\x45\xc5\x25\xa5\x65\xe5\x15\x95\x55\xd5\x35\xb5\x75\xf5" +
	"\x0d\x8d\x4d\xcd\x2d\xad\x6d\xed\x1d\x9d\x5d\xdd\x3d\xbd\x7d\xfd" +
	"\x03\x83\x43\xc3\x23\xa3\x63\xe3\x13\x93\x53\xd3\x33\xb3\x73\xf3" +
	"\x0b\x8b\x4b\xcb\x2b\xab\x6b\xeb\x1b\x9b\x5b\xdb\x3b\xbb\x7b\xfb" +
	"\x07\x87\x47\xc7\x27\xa7\x67\xe7\x17\x97\x57\xd7\x37\xb7\x77\xf7" +
	"\x0f\x8f\x4f\xcf\x2f\xaf\x6f\xef\x1f\x9f\x5f\xdf\x3f\xbf\x7f\xff"
