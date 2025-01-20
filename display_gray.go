package display

import (
	"github.com/BeatGlow/display/pixel"
)

type gray2Display struct {
	monoDisplay
	useMono bool
}

func (d *gray2Display) init(config *Config) error {
	_ = d.monoDisplay.init(config)
	if d.useMono = config.UseMono; !d.useMono {
		d.Image = pixel.NewGray2Image(config.Width, config.Height)
	}
	return nil
}

type gray4Display struct {
	baseDisplay
	useMono bool
}

func (d *gray4Display) init(config *Config) error {
	d.Image = pixel.NewGray4Image(config.Width, config.Height)
	d.width = config.Width
	d.height = config.Height
	d.rotation = config.Rotation
	return nil
}

func (d *gray4Display) Clear() {
	i := d.Image.(*pixel.Gray4Image)
	for j := range i.Pix {
		i.Pix[j] = 0
	}
}
