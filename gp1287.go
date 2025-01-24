package display

import (
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

func (d *gp1278) Close() error {
	return d.c.Close()
}

func (d *gp1278) command(command byte, args ...byte) (err error) {
	return d.c.Command(swap8(command), swap8s(args)...)
}

func (d *gp1278) commands(commands ...[]byte) (err error) {
	for _, command := range commands {
		if err = d.command(command[0], command[1:]...); err != nil {
			return
		}
	}
	return
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

func swap8(v byte) byte {
	return ((((v) & 0x80) >> 7) | (((v) & 0x40) >> 5) | (((v) & 0x20) >> 3) | (((v) & 0x10) >> 1) | (((v) & 0x08) << 1) | (((v) & 0x04) << 3) | (((v) & 0x02) << 5) | (((v) & 0x01) << 7))
}

func swap8s(b []byte) []byte {
	for i, v := range b {
		b[i] = swap8(v)
	}
	return b
}
