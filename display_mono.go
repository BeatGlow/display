package display

import (
	"github.com/BeatGlow/display/pixel"
)

type monoDisplay struct {
	baseDisplay
	halted bool
}

func (d *monoDisplay) init(config *Config) error {
	d.Image = pixel.NewMonoVerticalLSBImage(config.Width, config.Height)
	d.width = config.Width
	d.height = config.Height
	d.rotation = config.Rotation
	return nil
}

func (d *monoDisplay) Close() error {
	if !d.halted {
		if err := d.Show(false); err != nil {
			return err
		}
		d.halted = true
	}
	return nil
}

func (d *monoDisplay) Show(show bool) error {
	// NB: don't use d.send here
	if show {
		return d.command(ssd1xxxSetDisplayOn)
	} else {
		return d.command(ssd1xxxSetDisplayOff)
	}
}

func (d *monoDisplay) SetContrast(level uint8) error {
	return d.command(ssd1xxxSetContrast, level)
}

func (d *monoDisplay) SetRotation(rotation Rotation) error {
	d.rotation = rotation
	return nil
}
