package display

import (
	"encoding/binary"

	"github.com/BeatGlow/display/pixel"
)

type crgb15Display struct {
	baseDisplay
}

func (d *crgb15Display) init(config *Config, order binary.ByteOrder) error {
	d.Image = pixel.NewCRGB15Image(config.Width, config.Height)
	d.Image.(*pixel.CRGB15Image).Order = order
	return nil
}

func (d *crgb15Display) Clear() {
	i := d.Image.(*pixel.CRGB15Image)
	for j := range i.Pix {
		i.Pix[j] = 0
	}
}

type crgb16Display struct {
	baseDisplay
}

func (d *crgb16Display) init(config *Config, order binary.ByteOrder) error {
	d.Image = pixel.NewCRGB16Image(config.Width, config.Height)
	d.Image.(*pixel.CRGB16Image).Order = order
	return nil
}

func (d *crgb16Display) Clear() {
	i := d.Image.(*pixel.CRGB16Image)
	for j := range i.Pix {
		i.Pix[j] = 0
	}
}
