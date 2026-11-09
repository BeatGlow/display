package display

import (
	"errors"
	"image"
	"image/color"
	"image/draw"

	"periph.io/x/conn/v3/gpio"
)

// Errors returned by display drivers and connection constructors.
var (
	// ErrBounds indicates that a requested display region is outside its bounds.
	ErrBounds = errors.New("display: out of bounds")
	// ErrNotReady indicates that a display did not become ready in time.
	ErrNotReady = errors.New("display: ready timeout")
	// ErrResetPin indicates that the configured reset pin is invalid; it wraps an [InvalidPin].
	ErrResetPin = InvalidPin{"reset"}
	// ErrDCPin indicates that the configured data/command pin is invalid; it wraps an [InvalidPin].
	ErrDCPin = InvalidPin{"data/command (DC)"}
)

// Rotation defines the orientation of display pixels.
type Rotation uint8

// Supported display rotations.
const (
	// NoRotation keeps the display in its native orientation.
	NoRotation Rotation = iota
	// Rotate90 rotates the display 90 degrees clockwise.
	Rotate90
	// Rotate180 rotates the display 180 degrees.
	Rotate180
	// Rotate270 rotates the display 270 degrees clockwise.
	Rotate270
)

// String returns the rotation as a human-readable angle.
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

	// String returns a description of the display.
	String() string

	// Close releases the display driver resources.
	Close() error

	// Clear sets every pixel in the display buffer to black.
	Clear()

	// Fill sets every pixel in the display buffer to one color.
	Fill(color.Color)

	// Show turns the display output on or off.
	Show(bool) error

	// SetContrast sets the display contrast level.
	SetContrast(level uint8) error

	// SetRotation sets the display pixel orientation in clockwise 90-degree
	// increments. Quarter turns swap the width and height returned by Bounds.
	SetRotation(Rotation) error

	// Refresh writes the display buffer to the device.
	Refresh() error
}

// Config is the display configuration.
type Config struct {
	// Width is the display width in pixels.
	Width int

	// Height is the display height in pixels.
	Height int

	// Rotation is the display orientation.
	Rotation Rotation

	// UseMono selects monochrome mode on displays that support grayscale.
	UseMono bool

	// Reset is the optional display reset output pin.
	Reset gpio.PinOut

	// Backlight is the optional display backlight output pin.
	Backlight gpio.PinOut
}

type baseDisplay struct {
	draw.Image
	c              Conn
	width          int
	height         int
	colOffset      int
	rowOffset      int
	rotation       Rotation
	bufferRotation Rotation
}

func (d *baseDisplay) Bounds() image.Rectangle {
	return rotationBounds(d.Image.Bounds(), d.bufferRotation)
}

func (d *baseDisplay) At(x, y int) color.Color {
	bounds := d.Bounds()
	if !image.Pt(x, y).In(bounds) {
		return color.Transparent
	}
	x, y = rotationPoint(d.Image.Bounds(), d.bufferRotation, x, y)
	return d.Image.At(x, y)
}

func (d *baseDisplay) Set(x, y int, c color.Color) {
	if !image.Pt(x, y).In(d.Bounds()) {
		return
	}
	x, y = rotationPoint(d.Image.Bounds(), d.bufferRotation, x, y)
	d.Image.Set(x, y, c)
}

func (d *baseDisplay) SetRotation(rotation Rotation) error {
	d.rotation = rotation % 4
	d.bufferRotation = d.rotation
	return nil
}

// setHardwareRotation records the requested orientation while using controller
// address remapping for 180-degree turns. Quarter turns remain software mapped.
func (d *baseDisplay) setHardwareRotation(rotation Rotation, setFlip func(bool) error) error {
	rotation %= 4
	flip := rotation == Rotate180
	if err := setFlip(flip); err != nil {
		return err
	}
	d.rotation = rotation
	d.bufferRotation = rotation
	if flip {
		d.bufferRotation = NoRotation
	}
	return nil
}

func rotationBounds(bounds image.Rectangle, rotation Rotation) image.Rectangle {
	if rotation%4 == Rotate90 || rotation%4 == Rotate270 {
		return image.Rect(0, 0, bounds.Dy(), bounds.Dx())
	}
	return bounds
}

func rotationPoint(bounds image.Rectangle, rotation Rotation, x, y int) (int, int) {
	switch rotation % 4 {
	case Rotate90:
		return bounds.Min.X + y, bounds.Max.Y - 1 - x
	case Rotate180:
		return bounds.Max.X - 1 - x, bounds.Max.Y - 1 - y
	case Rotate270:
		return bounds.Max.X - 1 - y, bounds.Min.Y + x
	default:
		return x, y
	}
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
	if i, ok := d.Image.(interface{ Fill(color.Color) }); ok {
		i.Fill(c)
		return
	}
	i := d.Image
	r := i.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			i.Set(x, y, c)
		}
	}
}
