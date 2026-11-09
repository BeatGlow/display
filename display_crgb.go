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
	d.rotation = config.Rotation % 4
	d.bufferRotation = d.rotation
	return nil
}

type crgb16Display struct {
	baseDisplay
}

func (d *crgb16Display) init(config *Config, order binary.ByteOrder) error {
	d.Image = pixel.NewCRGB16Image(config.Width, config.Height)
	d.Image.(*pixel.CRGB16Image).Order = order
	d.rotation = config.Rotation % 4
	d.bufferRotation = d.rotation
	return nil
}
