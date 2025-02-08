package display

import (
	"fmt"
	"image"
	"image/color"

	"github.com/BeatGlow/display/conn"
	"github.com/BeatGlow/display/pixel"
	"periph.io/x/conn/v3/gpio"
)

const (
	gp1294DefaultWidth      = 256
	gp1294DefaultHeight     = 48
	gp1264DefaultBrightness = 0x0028
)

const (
	gp1294FrameSync     = 0x08
	gp1294DisplayOff    = 0x61
	gp1294DisplayOn     = 0x6D
	gp1294OscSetting    = 0x78
	gp1294DisplayMode   = 0x80
	gp1294Brightness    = 0xA0
	gp1294Reset         = 0xAA
	gp1294DisplayOffset = 0xC0
	gp1294VFDMode       = 0xCC
	gp1294WriteGRAM     = 0xF0
)

type gp1294 struct {
	conn          Conn
	spiConn       *conn.SPI
	width, height int
	pix           []byte
	pageSize      int
}

func GP1294(c Conn, config *Config) (Display, error) {
	d := &gp1294{conn: c}

	if c, ok := c.Interface().(*conn.SPI); ok {
		d.spiConn = c
		if err := c.SetMode(conn.SPIMode3); err != nil {
			return nil, err
		}
		if err := c.SetMaxSpeed(4_000_000); err != nil {
			return nil, err
		}
	}

	if s, ok := c.(*spiConn); ok {
		if err := s.SetMode(conn.SPIMode0); err != nil {
			return nil, err
		}
		if err := s.SetMaxSpeed(500000); err != nil {
			return nil, err
		}
	}

	if config.Width == 0 {
		config.Width = gp1294DefaultWidth
	}
	if config.Height == 0 {
		config.Height = gp1294DefaultHeight
	}

	d.height = config.Height
	d.width = config.Width
	d.pix = make([]byte, (d.width*d.height)/8)

	if err := d.init(config); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *gp1294) String() string {
	return fmt.Sprintf("GP1294 VFD %dx%d", d.width, d.height)
}

func (d *gp1294) Close() error {
	return d.conn.Close()
}

func (d *gp1294) init(config *Config) (err error) {
	if err = config.Backlight.Out(gpio.High); err != nil {
		return fmt.Errorf("gp1294: error setting backlight on: %w", err)
	}

	if err = d.commands([][]byte{
		{gp1294Reset},
		{gp1294VFDMode, 0x01, 0x1F, 0x00, 0xFF, 0x2F, 0x00, 0x20},
		{gp1294Brightness, byte(gp1264DefaultBrightness), byte(gp1264DefaultBrightness >> 8)},
	}...); err != nil {
		return
	}

	if err = d.clear(); err != nil {
		return
	}

	if err = d.commands([][]byte{
		{gp1294DisplayMode, 0x00},
		{gp1294OscSetting, 0x08},
		{gp1294DisplayOn},
	}...); err != nil {
		return
	}

	return
}

func (d *gp1294) commands(cmds ...[]byte) (err error) {
	for _, cmd := range cmds {
		if err = d.command(cmd[0], cmd[1:]...); err != nil {
			return
		}
	}
	return
}

func (d *gp1294) command(cmd byte, args ...byte) (err error) {
	b := make([]byte, 1+len(args))
	b[0] = rev8tab[cmd]
	for i, data := range args {
		b[i+1] = rev8tab[data]
	}

	if d.spiConn != nil {
		_, err = d.spiConn.Write(b)
	} else {
		err = d.conn.Command(b[0], b[1:]...)
	}
	return
}

func (d *gp1294) Command(cmd byte, args ...byte) error {
	return d.command(cmd, args...)
}

func (d *gp1294) Clear() {
	d.Fill(color.Black)
}

func (d *gp1294) Fill(c color.Color) {
	var fill byte
	if pixel.MonoModel.Convert(c).(pixel.Mono).On {
		fill = 0xFF
	}
	for i := range d.pix {
		d.pix[i] = fill
	}
}

func (d *gp1294) clear() error {
	empty := make([]byte, (d.width*d.height)/8)
	return d.command(gp1294WriteGRAM, append([]byte{
		0, 0, byte(d.height) - 1},
		empty...,
	)...)
}

func (d *gp1294) Bounds() image.Rectangle {
	return image.Rect(0, 0, d.width, d.height)
}

func (d *gp1294) ColorModel() color.Model {
	return pixel.MonoModel
}

func (d *gp1294) At(x, y int) color.Color {
	if !image.Pt(x, y).In(d.Bounds()) {
		return color.Transparent
	}

	offset := x*(d.height/8) + y/8
	if d.pix[offset]&(1<<y%8) != 0 {
		return pixel.On
	}
	return pixel.Off
}

func (d *gp1294) Set(x, y int, c color.Color) {
	if !image.Pt(x, y).In(d.Bounds()) {
		return
	}

	offset := x*(d.height/8) + y/8
	if pixel.MonoModel.Convert(c).(pixel.Mono).On {
		d.pix[offset] |= (1 << (y % 8))
	} else {
		d.pix[offset] &^= (1 << (y % 8))
	}
}

func (d *gp1294) SetContrast(level uint8) error {
	value := uint16(level) << 2
	return d.Command(gp1294Brightness, byte(value), byte(value>>8))
}

func (gp1294) SetRotation(_ Rotation) error {
	return nil
}

func (d *gp1294) Show(show bool) error {
	if show {
		return d.Command(gp1294DisplayOn)
	} else {
		return d.Command(gp1294DisplayOff)
	}
}

func (d *gp1294) Refresh() error {
	return d.command(gp1294WriteGRAM, append([]byte{
		0, 0, byte(d.height) - 1},
		d.pix...,
	)...)
}
