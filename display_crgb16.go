package display

import (
	"encoding/binary"

	"github.com/BeatGlow/display/pixel"
)

type crgb16Display struct {
	baseDisplay
}

func (d *crgb16Display) init(config *Config, order binary.ByteOrder) error {
	d.Image = pixel.NewCRGB16Image(config.Width, config.Height)
	d.Image.(*pixel.CRGB16Image).Order = order
	return nil
}
