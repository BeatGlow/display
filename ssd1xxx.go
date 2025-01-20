package display

import "github.com/BeatGlow/display/pixel"

const (
	ssd1xxxSetLowColumn          = 0x00
	ssd1xxxSetHighColumn         = 0x10
	ssd1xxxSetMemoryMode         = 0x20
	ssd1xxxSetColumnAddr         = 0x21
	ssd1xxxSetPageAddr           = 0x22
	ssd1xxxSetStartLine          = 0x40
	ssd1xxxSetContrast           = 0x81
	ssd1xxxSetChargePump         = 0x8D
	ssd1xxxSetRemap              = 0xA0
	ssd1xxxSetSegmentRemap       = 0xA1
	ssd1xxxSetDisplayAllOnResume = 0xA4
	ssd1xxxSetDisplayAllOn       = 0xA5
	ssd1xxxSetNormalDisplay      = 0xA6
	ssd1xxxSetInvertDisplay      = 0xA7
	ssd1xxxSetMultiplexRatio     = 0xA8
	ssd1xxxSetDisplayOff         = 0xAE
	ssd1xxxSetDisplayOn          = 0xAF
	ssd1xxxSetComScanInc         = 0xC0
	ssd1xxxSetComScanDec         = 0xC8
	ssd1xxxSetDisplayOffset      = 0xD3
	ssd1xxxSetDisplayClockDiv    = 0xD5
	ssd1xxxSetPrecharge          = 0xD9
	ssd1xxxSetComPins            = 0xDA
	ssd1xxxSetVCOMDetect         = 0xDB
	ssd1xxxSetCommandLock        = 0xFD
	externalVCC                  = 0x1
	switchCapVCC                 = 0x2
)

type ssd1xxxDisplay struct {
	monoDisplay
	useMono bool
	halted  bool
}

func (d *ssd1xxxDisplay) init(config *Config) error {
	d.Image = pixel.NewMonoVerticalLSBImage(config.Width, config.Height)
	d.width = config.Width
	d.height = config.Height
	d.rotation = config.Rotation
	d.useMono = config.UseMono // for 4-bit gray scale capable displays
	return nil
}

func (d *ssd1xxxDisplay) Halt() error {
	if !d.halted {
		if err := d.Show(false); err != nil {
			return err
		}
		d.halted = true
	}
	return nil
}

func (d *ssd1xxxDisplay) Show(show bool) error {
	// NB: don't use d.send here
	if show {
		return d.command(ssd1xxxSetDisplayOn)
	} else {
		return d.command(ssd1xxxSetDisplayOff)
	}
}

func (d *ssd1xxxDisplay) SetContrast(level uint8) error {
	return d.command(ssd1xxxSetContrast, level)
}

func (d *ssd1xxxDisplay) SetRotation(rotation Rotation) error {
	d.rotation = rotation
	return nil
}
