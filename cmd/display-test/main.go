package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"os"
	"strings"
	"time"

	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/host/v3"

	"github.com/BeatGlow/display"
	"github.com/BeatGlow/display/draw"
	"github.com/BeatGlow/display/pixel"
)

func main() {
	widthFlag := flag.Int("width", 0, "Display width")
	heightFlag := flag.Int("height", 0, "Display height")
	useMonoFlag := flag.Bool("mono", false, "Display uses monochrome colors")
	i2cDeviceFlag := flag.Int("i2c-dev", display.DefaultI2CConfig.Device, "I²C device number (default: use first available)")
	i2cAddrFlag := flag.Uint("i2c-addr", uint(display.DefaultI2CConfig.Addr), "I²C device address")
	spiBusFlag := flag.Int("spi-bus", 0, "SPI bus")
	spiDeviceFlag := flag.Int("spi-dev", 0, "SPI device")
	resetPinFlag := flag.String("reset", "GPIO25", "Reset GPIO pin")
	dcPinFlag := flag.String("dc", "GPIO24", "Data/Command GPIO pin (DC)")
	cePinFlag := flag.String("ce", "GPIO8", "Chip enable GPIO pin")
	blPinFlag := flag.String("bl", "GPIO19", "Backlight GPIO pin")
	rotateFlag := flag.String("rotate", "", "Display rotation")
	flag.Parse()

	if flag.NArg() != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <bus> <driver>\n", os.Args[0])
		os.Exit(1)
	}

	var rotation display.Rotation
	switch *rotateFlag {
	case "", "no", "0":
		rotation = display.NoRotation
	case "90", "right", "cw":
		rotation = display.Rotate90
	case "180", "flip":
		rotation = display.Rotate180
	case "270", "left", "ccw":
		rotation = display.Rotate270
	default:
		fatal(fmt.Errorf("invalid rotation %q specified", *rotateFlag))
	}
	fmt.Printf("using rotation: %s\n", rotation)

	if _, err := host.Init(); err != nil {
		fatal(err)
	}

	var (
		config = &display.Config{
			Width:     *widthFlag,
			Height:    *heightFlag,
			Rotation:  rotation,
			UseMono:   *useMonoFlag,
			Backlight: gpioreg.ByName(*blPinFlag),
		}
		conn   display.Conn
		output display.Display
		err    error
	)
	switch busType := flag.Arg(0); busType {
	case "i2c":
		conn, err = display.OpenI2C(&display.I2CConfig{
			Device: *i2cDeviceFlag,
			Addr:   uint8(*i2cAddrFlag),
			Reset:  gpioreg.ByName(*resetPinFlag),
		})
	case "spi":
		conn, err = display.OpenSPI(&display.SPIConfig{
			Bus:    *spiBusFlag,
			Device: *spiDeviceFlag,
			Reset:  gpioreg.ByName(*resetPinFlag),
			DC:     gpioreg.ByName(*dcPinFlag),
			CE:     gpioreg.ByName(*cePinFlag),
		})
	default:
		err = fmt.Errorf("unsupported bus type %q", busType)
	}
	if err != nil {
		fatal(err)
	}
	defer conn.Close()
	fmt.Printf("using connection: %s\n", conn)

	switch driver := strings.ToLower(flag.Arg(1)); driver {
	case "gp1294":
		output, err = display.GP1294(conn, config)
	case "sh1106":
		output, err = display.SH1106(conn, config)
	case "ssd1305":
		output, err = display.SSD1305(conn, config)
	case "ssd1306":
		output, err = display.SSD1306(conn, config)
	case "ssd1322":
		output, err = display.SSD1322(conn, config)
	//case "ssd1326":
	//	output, err = display.SSD1326(conn, config)
	case "st7735":
		output, err = display.ST7735(conn, config)
	case "st7789":
		output, err = display.ST7789(conn, config)
	default:
		err = fmt.Errorf("unsupported driver %q", driver)
	}
	if err != nil {
		fatal(err)
	}

	fmt.Printf("using driver: %s\n", output)
	var (
		offset int
		ticker = time.NewTicker(50 * time.Millisecond)
		r      = output.Bounds()
	)
	defer ticker.Stop()

	// Draw box around edge
	for x := 0; x < r.Max.X; x++ {
		output.Set(x, 0, pixel.On)
		output.Set(x, r.Max.Y-1, pixel.On)
	}
	for y := 0; y < r.Max.Y; y++ {
		output.Set(0, y, pixel.On)
		output.Set(r.Max.X-1, y, pixel.On)
	}
	if err = output.Refresh(); err != nil {
		fatal(err)
	}

	var (
		m         = output.ColorModel()
		bits      int
		isGraphic bool
		drawOp    = draw.Over
	)
	switch m {
	case pixel.MonoModel:
		bits = 1
		fmt.Println("using color model: monochrome")
	case pixel.Gray2Model:
		bits = 2
		fmt.Println("using color model: 2-bit gray")
		isGraphic = true
		drawOp = draw.Src
	case pixel.Gray4Model:
		bits = 4
		fmt.Println("using color model: 4-bit gray")
		isGraphic = true
	case pixel.CRGB15Model:
		bits = 15
		fmt.Println("using color model: 15-bit RGB")
		isGraphic = true
	case pixel.CRGB16Model:
		bits = 16
		fmt.Println("using color model: 16-bit RGB")
		isGraphic = true
	default:
		fmt.Println("using color model: something else")
	}

	var (
		size    = output.Bounds()
		logo    image.Image
		logoPos image.Rectangle
	)
	if isGraphic {
		logo = logoImage(size.Max)
		logoSize := logo.Bounds().Size()
		logoPos.Min = image.Pt(size.Dx()/2-logoSize.X/2, size.Dy()/2-logoSize.Y/2)
		logoPos.Max = logoPos.Min.Add(logoSize)
		fmt.Printf("yay! your %s display can show %d-bit images, plotting %s logo at %s\n", output.Bounds().Size(), bits, logoSize, logoPos)
	}

	fmt.Println("hit control-c to stop...")
	for {
		// Draw gradient inside box
		for y := 1; y < r.Max.Y-1; y++ { // r.Max.Y; y++ {
			for x := 1; x < r.Max.X-1; x++ { // r.Max.X; x++ {
				switch m {
				case pixel.MonoModel:
					if (x+y+offset)%4 == 0 {
						output.Set(x, y, pixel.On)
					} else {
						output.Set(x, y, pixel.Off)
					}
				case pixel.Gray2Model:
					output.Set(x, y, pixel.Gray4{Y: uint8(x+y+offset) & 0x3})
				case pixel.Gray4Model:
					output.Set(x, y, pixel.Gray4{Y: uint8(x+y+offset) & 0xf})
				default:
					output.Set(x, y, color.RGBA{
						R: uint8(x + y + offset),
						G: uint8(x - y + offset),
						B: uint8(x + y - offset),
						A: 0xff,
					})
				}
			}
		}

		if isGraphic {
			draw.Draw(output, logoPos, logo, image.Point{}, drawOp)
		} else {
			box := image.Rect(5, 5, 125, 15)
			draw.RoundedBox(output, box, 5, pixel.On)
		}

		/*
			if err = output.SetContrast(uint8(offset)); err != nil {
				println(err)
			}
		*/

		if err = output.Refresh(); err != nil {
			fatal(err)
		}

		offset++
		<-ticker.C
		//break
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "fatal: "+err.Error())
	os.Exit(1)
}
