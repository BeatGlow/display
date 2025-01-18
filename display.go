// Package display contains drivers for hardware displays.
package display

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"log"
	"os"

	"periph.io/x/conn/v3/gpio"

	"github.com/BeatGlow/display/pixel"
)

var debug bool

func init() {
	debug = os.Getenv("DISPLAY_DEBUG") != ""
}

// Errors
var (
	ErrBounds = errors.New("oled: out of display bounds")
)

// Rotation defines pixel rotation.
type Rotation uint8

// Supported rotations.
const (
	NoRotation Rotation = iota
	Rotate90            // Rotate 90° clock wise
	Rotate180           // Rotate 180°
	Rotate270           // Rotate 270° clock wise
)

// Display is an OLED display.
type Display interface {
	// At returns the color of the pixel at (x, y).
	At(x, y int) color.Color

	// Set the pixel color at (x, y).
	Set(x, y int, c color.Color)

	// Bounds is the display bounding box (dimensions).
	Bounds() image.Rectangle

	// ColorModel used by the display.
	ColorModel() color.Model

	// Show toggles the display on or off.
	Show(bool) error

	// SetContrast adjusts the contrast level.
	SetContrast(level uint8) error

	// SetRotation adjusts the pixel rotation.
	SetRotation(Rotation) error

	// Refresh redraws the display.
	Refresh() error
}

// Config is the display configuration.
type Config struct {
	// Width of the display in pixels.
	Width int

	// Height of the display in pixels.
	Height int

	// Rotation of the display.
	Rotation Rotation

	// UseMono sets 1-color monochrome mode on displays that support grayscale.
	UseMono bool

	// Reset pin
	Reset gpio.PinOut
}

type display struct {
	c            Conn
	halted       bool
	columnOffset int
	rowOffset    int
	rotation     Rotation
	buf          *pixel.MonoVerticalLSBImage
}

func (d *display) init(config *Config) error {
	d.buf = pixel.NewMonoVerticalLSBImage(config.Width, config.Height)
	return nil
}

// send transparently re-enabled a halted display
func (d *display) send(data []byte, isCommand bool) error {
	if d.halted {
		if err := d.Show(true); err != nil {
			return err
		}
		d.halted = false
	}
	if !isCommand && debug {
		log.Printf("data %s", hex.EncodeToString(data))
	}
	return d.c.Send(data, isCommand)
}

// command sends a command
func (d *display) command(cmd byte, args ...byte) (err error) {
	if debug {
		log.Printf("command %#02x data %#02x", cmd, args)
	}
	/*
		if err = d.send([]byte{cmd}, true); err != nil {
			return
		}
		if len(args) == 0 {
			return
		}
		return d.send(args, false)
	*/
	return d.send(append([]byte{cmd}, args...), true)
}

func (d *display) commands(cmds ...[]byte) (err error) {
	for _, cmd := range cmds {
		if err = d.command(cmd[0], cmd[1:]...); err != nil {
			return
		}
	}
	return
}

func (d *display) Halt() error {
	if !d.halted {
		if err := d.Show(false); err != nil {
			return err
		}
		d.halted = true
	}
	return nil
}

func (d *display) Show(show bool) error {
	// NB: don't use d.send here
	if show {
		return d.c.Send([]byte{setDisplayOn}, true)
	} else {
		return d.c.Send([]byte{setDisplayOff}, true)
	}
}

func (d *display) SetContrast(level uint8) error {
	return d.command(setContrast, level)
}

func (d *display) SetRotation(rotation Rotation) error {
	d.rotation = rotation
	return nil
}

func (d *display) At(x, y int) color.Color {
	return d.buf.At(x, y)
}

func (d *display) Set(x, y int, c color.Color) {
	d.buf.Set(x, y, c)
}

func (d display) Bounds() image.Rectangle {
	return d.buf.Rect
}

func (display) ColorModel() color.Model {
	return pixel.MonoModel
}

type grayDisplay struct {
	display
	buf     *pixel.Gray4Image
	useMono bool
}

func (d *grayDisplay) init(config *Config) error {
	d.buf = pixel.NewGray4Image(config.Width, config.Height)
	d.useMono = config.UseMono

	return d.display.init(config) // init base
}

func (d *grayDisplay) Bounds() image.Rectangle {
	return d.buf.Bounds()
}

func (d *grayDisplay) At(x, y int) color.Color {
	if d.useMono {
		return d.display.buf.At(x, y)
	}
	return d.buf.At(x, y)
}

func (d *grayDisplay) Set(x, y int, c color.Color) {
	if d.useMono {
		d.display.buf.Set(x, y, c)
		return
	}
	d.buf.Set(x, y, c)
}

func (grayDisplay) ColorModel() color.Model {
	return pixel.Gray4Model
}

type crgb16Display struct {
	display
	buf *pixel.CRGB16Image
}

func (d *crgb16Display) init(config *Config, order binary.ByteOrder) error {
	d.buf = pixel.NewCRGB16Image(config.Width, config.Height)
	d.buf.Order = order
	return nil
}

func (d *crgb16Display) Bounds() image.Rectangle {
	return d.buf.Bounds()
}

func (c *crgb16Display) ColorModel() color.Model {
	return pixel.CRGB16Model
}

func (d *crgb16Display) At(x, y int) color.Color {
	return d.buf.At(x, y)
}

func (d *crgb16Display) Set(x, y int, c color.Color) {
	d.buf.Set(x, y, c)
}
