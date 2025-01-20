package display

import (
	"fmt"
	"image"
	"time"

	"github.com/BeatGlow/display/conn"
	"github.com/BeatGlow/display/pixel"
	"periph.io/x/conn/v3/gpio"
)

const (
	ssd1322DefaultWidth  = 256
	ssd1322DefaultHeight = 64
)

const (
	ssd1322EnableGrayScaleTable = 0x00
	ssd1322SetColumnAddress     = 0x15
	ssd1322WriteRAM             = 0x5C
	ssd1322SetRowAddress        = 0x75
	ssd1322SetRemap             = 0xA0
	ssd1322SetDisplayStartLine  = 0xA1
	ssd1322SetDisplayOffset     = 0xA2
	//ssd1322SetDisplayNormal       = 0xA4
	ssd1322SetDisplayAllOn        = 0xA5
	ssd1322SetNormalDisplay       = 0xA6
	ssd1322SetInverseDIsplay      = 0xA7
	ssd1322SetExitPartialDisplay  = 0xA9
	ssd1322SetFunction            = 0xAB
	ssd1322SetDislpayOff          = 0xAE
	ssd1322SetDisplayOn           = 0xAF
	ssd1322SetPhaseLength         = 0xB1
	ssd1322SetFrontClockDiv       = 0xB3
	ssd1322SetDisplayEnhancementA = 0xB4
	ssd1322SetGPIO                = 0xB5
	ssd1322SetSecondPrecharge     = 0xB6
	ssd1322SetGrayScaleTable      = 0xB8
	ssd1322SetDefaultGrayscale    = 0xB9
	ssd1322SetPrechargeVoltage    = 0xBB
	ssd1322SetVCOMHVoltage        = 0xBE
	ssd1322SetContrast            = 0xC1
	ssd1322SetMasterCurrent       = 0xC7
	ssd1322SetMultiplexRatio      = 0xCA
	ssd1322SetDisplayEnhancementB = 0xD1
	ssd1322SetCommandLock         = 0xFD
)

var (
	ssd1322SupportedSizes = []image.Point{
		image.Pt(256, 64),
		image.Pt(256, 48),
		image.Pt(256, 32),
		image.Pt(128, 64),
		image.Pt(128, 48),
		image.Pt(128, 32),
		image.Pt(64, 64),
		image.Pt(64, 48),
		image.Pt(64, 32),
	}
)

type ssd1322 struct {
	ssd1xxxDisplay
}

// SSD1322 is a driver for Solomon Systech SSD1322 OLED display.
func SSD1322(c Conn, config *Config) (Display, error) {
	// Update mode and speed
	if spi, ok := c.(SPI); ok {
		if err := spi.SetMode(conn.SPIMode3); err != nil {
			return nil, err
		}
		if err := spi.SetMaxSpeed(2500000); err != nil {
			return nil, err
		}
	}

	d := &ssd1322{
		ssd1xxxDisplay: ssd1xxxDisplay{
			monoDisplay: monoDisplay{
				baseDisplay: baseDisplay{
					c: c,
				},
			},
		},
	}

	if err := d.c.Reset(gpio.Low); err != nil {
		return nil, err
	}
	time.Sleep(150 * time.Millisecond)
	if err := d.c.Reset(gpio.High); err != nil {
		return nil, err
	}
	time.Sleep(250 * time.Millisecond)
	if err := d.init(config); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *ssd1322) String() string {
	bounds := d.Bounds()
	return fmt.Sprintf("SSD1322 %dx%d", bounds.Dx(), bounds.Dy())
}

func (d *ssd1322) init(config *Config) (err error) {
	if config.Width == 0 {
		config.Width = ssd1322DefaultWidth
	}
	if config.Height == 0 {
		config.Height = ssd1322DefaultHeight
	}

	var supported bool
	for _, size := range ssd1322SupportedSizes {
		if supported = size.X == config.Width && size.Y == config.Height; supported {
			break
		}
	}
	if !supported {
		return fmt.Errorf("display: SSD1322 unsupported size %dx%d", config.Width, config.Height)
	}

	// init base
	if err = d.ssd1xxxDisplay.init(config); err != nil {
		return
	}
	d.Image = pixel.NewGray4Image(d.width, d.height)

	// init display
	if err = d.commands(
		[]byte{ssd1322SetCommandLock, 0x12},                      // Unlock OLED driver IC
		[]byte{ssd1322SetDislpayOff},                             // 0xAE
		[]byte{ssd1322SetFrontClockDiv, 0x91},                    // 0xB3
		[]byte{ssd1322SetMultiplexRatio, byte(d.width - 1)},      // 0xCA
		[]byte{ssd1322SetDisplayOffset, 0x00},                    // 0xA2
		[]byte{ssd1322SetDisplayStartLine, 0x00},                 // 0xA1
		[]byte{ssd1322SetRemap, 0x14, 0x11},                      // Horizontal address increment,Disable Column Address Re-map,Enable Nibble Re-map,Scan from COM[N-1] to COM0,Disable COM Split Odd Even; Enable Dual COM mode
		[]byte{ssd1322SetGPIO, 0x00},                             // Disable GPIO Pins Input
		[]byte{ssd1322SetFunction, 0x01},                         // Selection external VDD
		[]byte{ssd1322SetDisplayEnhancementA, 0xA0, 0x05 | 0xFD}, // Enable external VSL; Enhanced low GS display quality;default is 0xb5(normal),
		[]byte{ssd1322SetContrast, 0x7F},                         // 0xFF - default is 0x7f
		[]byte{ssd1322SetMasterCurrent, 0x0F},                    // Default is 0x0F
		[]byte{ssd1322SetDefaultGrayscale},                       // Grayscale 4-bit
		[]byte{ssd1322SetPhaseLength, 0xE2},                      // Default is 0x74
		[]byte{ssd1322SetDisplayEnhancementB, 0x82, 0x20},        // Reserved; default is 0xa2(normal)
		[]byte{ssd1322SetPrechargeVoltage, 0x1F},                 // 0.6xVcc
		[]byte{ssd1322SetSecondPrecharge, 0x08},                  // Default
		[]byte{ssd1322SetVCOMHVoltage, 0x07},                     // 0.86xVcc;default is 0x04
		[]byte{ssd1322SetNormalDisplay},                          // Normal display
		[]byte{ssd1322SetExitPartialDisplay},
	); err != nil {
		return
	}
	time.Sleep(2 * time.Millisecond)

	if err = d.clearRAM(); err != nil {
		return
	}

	if err = d.SetContrast(0xFF); err != nil {
		return
	}
	if err = d.Refresh(); err != nil {
		return
	}
	if err = d.Show(true); err != nil {
		return
	}

	return
}

func (d *ssd1322) clearRAM() (err error) {
	if err = d.commands(
		[]byte{ssd1322SetColumnAddress, 0x00, 0x77},
		[]byte{ssd1322SetRowAddress, 0x00, 0x7F},
		[]byte{ssd1322WriteRAM},
	); err != nil {
		return
	}

	blank := make([]byte, 120) // 480/4
	for y := 0; y < 128; y++ {
		if err = d.data(blank...); err != nil {
			return
		}
	}
	return
}

func (d *ssd1322) SetContrast(level uint8) error {
	return d.command(ssd1322SetContrast, level)
}

func (d *ssd1322) setWindow(x, y, width, height int) error {
	var (
		x0 = byte((480-d.width)/8) + byte(x/4)
		x1 = x0 + byte(width/4) - 1
		y0 = byte(y)
		y1 = y0 + byte(height) - 1
	)
	return d.commands(
		[]byte{ssd1322SetRowAddress, y0, y1},
		[]byte{ssd1322SetColumnAddress, x0, x1},
	)
}

// Refresh needs to be duplicated here, otherwise we can't access the gray buf.
func (d *ssd1322) Refresh() error {
	if err := d.setWindow(0, 0, d.width, d.height); err != nil {
		return err
	}
	if err := d.command(ssd1322WriteRAM); err != nil {
		return err
	}

	switch i := d.Image.(type) {
	case *pixel.MonoVerticalLSBImage:
		return d.data(i.Pix...) // 2048 bytes @ 256x64x1
	case *pixel.Gray2Image:
		return d.data(i.Pix...) // 4096 bytes @ 256x64x2
	case *pixel.Gray4Image:
		return d.data(i.Pix...) // 8192 bytes @ 256x64x4
	}
	return nil
}

// Interface checks
var (
	_ Display = (*ssd1322)(nil)
)
