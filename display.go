// Package display contains drivers for hardware displays.
package display

import (
	"errors"
	"image/color"
	"image/draw"

	"github.com/BeatGlow/display/pixel"
	"periph.io/x/conn/v3/gpio"
)

// Errors
var (
	ErrBounds   = errors.New("display: out of bounds")
	ErrNotReady = errors.New("display: ready timeout")
	ErrResetPin = InvalidPin{"reset"}
	ErrDCPin    = InvalidPin{"data/command (DC)"}
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

func (r Rotation) String() string {
	switch r % 4 {
	case Rotate90:
		return "90°"
	case Rotate180:
		return "180°"
	case Rotate270:
		return "270°"
	default:
		return "0°"
	}
}

// Display is an OLED display.
type Display interface {
	draw.Image

	// String gives a description of the display.
	String() string

	// Close the display driver.
	Close() error

	// Clear the display buffer.
	Clear()

	// Fill the display buffer with a single color.
	Fill(color.Color)

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

	// Backlight pin
	Backlight gpio.PinOut
}

type baseDisplay struct {
	draw.Image
	c         Conn
	width     int
	height    int
	colOffset int
	rowOffset int
	rotation  Rotation
}

func (d *baseDisplay) data(data ...byte) error {
	return d.c.Data(data...)
}

func (d *baseDisplay) command(command byte, data ...byte) error {
	return d.c.Command(command, data...)
}

func (d *baseDisplay) commands(commands ...[]byte) (err error) {
	for _, command := range commands {
		if err = d.c.Command(command[0], command[1:]...); err != nil {
			return
		}
	}
	return
}

func (d *baseDisplay) Clear() {
	d.Fill(color.Black)
}

func (d *baseDisplay) Fill(c color.Color) {
	switch i := d.Image.(type) {
	case *pixel.MonoImage:
		i.Fill(c)
	case *pixel.Gray2Image:
		i.Fill(c)
	case *pixel.Gray4Image:
		i.Fill(c)
	case *pixel.CRGB15Image:
		i.Fill(c)
	case *pixel.CRGB16Image:
		i.Fill(c)
	default:
		r := i.Bounds()
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				i.Set(x, y, c)
			}
		}
	}
}
