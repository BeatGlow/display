// Package display contains drivers for hardware displays.
package display

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"os"

	"github.com/BeatGlow/display/pixel"
	"periph.io/x/conn/v3/gpio"
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
	// Close the display driver.
	Close() error

	// Clear the display buffer.
	Clear()

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
	switch i := d.Image.(type) {
	case *pixel.MonoImage:
		for j := range i.Pix {
			i.Pix[j] = 0
		}
	case *pixel.MonoVerticalLSBImage:
		for j := range i.Pix {
			i.Pix[j] = 0
		}
	case *pixel.Gray2Image:
		for j := range i.Pix {
			i.Pix[j] = 0
		}
	case *pixel.Gray4Image:
		for j := range i.Pix {
			i.Pix[j] = 0
		}
	case *pixel.CRGB15Image:
		for j := range i.Pix {
			i.Pix[j] = 0
		}
	case *pixel.CRGB16Image:
		for j := range i.Pix {
			i.Pix[j] = 0
		}
	}
}
