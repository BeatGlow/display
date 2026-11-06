package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
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
	uartPortFlag := flag.String("uart-port", "", "UART port name or alias")
	uartBaudFlag := flag.Int("uart-baud", 38400, "UART baud rate (module default)")
	uartBusyPinFlag := flag.String("uart-busy", "", "UART SBUSY GPIO pin (optional)")
	resetPinFlag := flag.String("reset", "GPIO25", "Reset GPIO pin")
	dcPinFlag := flag.String("dc", "GPIO24", "Data/Command GPIO pin (DC)")
	cePinFlag := flag.String("ce", "GPIO8", "Chip enable GPIO pin")
	blPinFlag := flag.String("bl", "GPIO19", "Backlight GPIO pin")
	rotateFlag := flag.String("rotate", "", "Display rotation")
	colorFlag := flag.String("color", "mono", "Offline virtual display color model: mono, gray2, gray4, rgb15, rgb16")
	outputFlag := flag.String("output", "display.png", "Offline preview PNG path")
	traceFlag := flag.String("trace", "display.trace.json", "Offline command/data trace path")
	contrastFlag := flag.Int("contrast", -1, "Set driver contrast/brightness for offline checks (0-255)")
	flag.Parse()

	if flag.NArg() != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <bus> <driver>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "       %s [options] offline virtual\n", os.Args[0])
		os.Exit(1)
	}
	if *contrastFlag < -1 || *contrastFlag > 255 {
		fatal(fmt.Errorf("contrast must be -1 (unset) or in the range 0-255"))
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

	offline := flag.Arg(0) == "offline"
	if !offline {
		if _, err := host.Init(); err != nil {
			fatal(err)
		}
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
	var offlineBacklight *offlinePin
	if offline {
		offlineBacklight = &offlinePin{}
		config.Backlight = offlineBacklight
	}
	switch busType := flag.Arg(0); busType {
	case "offline":
		conn = &recordingConn{}
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
	case "uart":
		conn, err = display.OpenUART(&display.UARTConfig{
			Port: *uartPortFlag,
			Baud: *uartBaudFlag,
			Busy: gpioreg.ByName(*uartBusyPinFlag),
		})
	default:
		err = fmt.Errorf("unsupported bus type %q", busType)
	}
	if err != nil {
		fatal(err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			fatal(fmt.Errorf("failed to close display connection: %w", err))
		}
	}()
	fmt.Printf("using connection: %s\n", conn)

	switch driver := strings.ToLower(flag.Arg(1)); driver {
	case "gp1247ai":
		output, err = display.GP1247AI(conn, config)
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
	case "sh1122":
		output, err = display.SH1122(conn, config)
	case "gp1278", "gp1287":
		output, err = display.GP1278(conn, config)
	case "gu3000":
		output, err = display.GU3000(conn, config)
	case "gu256x64d7000":
		output, err = display.GU256X64D7000(conn, config)
	case "virtual":
		if !offline {
			err = fmt.Errorf("the virtual driver is only available with the offline bus")
			break
		}
		output, err = newVirtualDisplay(config.Width, config.Height, *colorFlag)
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
	if recorder, ok := conn.(*recordingConn); ok {
		if err := recorder.VerifyInitialization(strings.ToLower(flag.Arg(1))); err != nil {
			fatal(err)
		}
	}
	contrastEventsStart := 0
	if recorder, ok := conn.(*recordingConn); ok {
		contrastEventsStart = len(recorder.events)
	}
	if offline && *contrastFlag >= 0 {
		if err = output.SetContrast(uint8(*contrastFlag)); err != nil {
			fatal(err)
		}
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
		m      = output.ColorModel()
		bits   int
		drawOp = draw.Over
	)
	switch m {
	case pixel.MonoModel:
		bits = 1
		fmt.Println("using color model: monochrome")
	case pixel.Gray2Model:
		bits = 2
		fmt.Println("using color model: 2-bit gray")
		drawOp = draw.Src
	case pixel.Gray4Model:
		bits = 4
		fmt.Println("using color model: 4-bit gray")
	case pixel.CRGB15Model:
		bits = 15
		fmt.Println("using color model: 15-bit RGB")
	case pixel.CRGB16Model:
		bits = 16
		fmt.Println("using color model: 16-bit RGB")
	default:
		fmt.Println("using color model: something else")
	}

	var (
		size    = output.Bounds()
		logo    image.Image
		logoPos image.Rectangle
	)
	logo = logoImage(size.Size(), m == pixel.MonoModel)
	logoSize := logo.Bounds().Size()
	logoPos.Min = image.Pt(size.Dx()/2-logoSize.X/2, size.Dy()/2-logoSize.Y/2)
	logoPos.Max = logoPos.Min.Add(logoSize)
	fmt.Printf("plotting BeatGlow logo on %s %d-bit display at %s\n", size.Size(), bits, logoPos)

	if offline {
		fmt.Println("rendering one offline frame...")
	} else {
		fmt.Println("hit control-c to stop...")
	}
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

		draw.Draw(output, logoPos, logo, image.Point{}, drawOp)

		/*
			if err = output.SetContrast(uint8(offset)); err != nil {
				println(err)
			}
		*/

		eventStart := 0
		if recorder, ok := conn.(*recordingConn); ok {
			eventStart = len(recorder.events)
		}
		if err = output.Refresh(); err != nil {
			fatal(err)
		}

		offset++
		if offline {
			var preview image.Image = output
			var validationErrors []string
			if recorder, ok := conn.(*recordingConn); ok {
				if driver := strings.ToLower(flag.Arg(1)); driver != "virtual" {
					var simulated image.Image
					simulated, err = recorder.VerifyFrame(driver, output, eventStart)
					if err != nil {
						validationErrors = append(validationErrors, err.Error())
						fmt.Printf("emulation failed; previewing software framebuffer: %v\n", err)
					} else {
						preview = simulated
						fmt.Println("emulated display memory matches the requested framebuffer")
					}
					if *contrastFlag >= 0 {
						verified, verifyErr := recorder.VerifyContrast(driver, contrastEventsStart, uint8(*contrastFlag), offlineBacklight)
						if verifyErr != nil {
							validationErrors = append(validationErrors, verifyErr.Error())
						} else if verified {
							fmt.Printf("contrast command verified for level %d\n", *contrastFlag)
						} else {
							fmt.Printf("%s has no contrast control in its current driver\n", driver)
						}
					}
				}
				fmt.Printf("captured %d command calls and %d data bytes\n", recorder.commands, recorder.dataBytes)
				trace, traceErr := os.Create(*traceFlag)
				if traceErr != nil {
					fatal(traceErr)
				}
				if err = json.NewEncoder(trace).Encode(recorder.events); err != nil {
					_ = trace.Close()
					fatal(err)
				}
				if err = trace.Close(); err != nil {
					fatal(err)
				}
				fmt.Printf("wrote connection trace to %s\n", *traceFlag)
			}
			if *contrastFlag >= 0 {
				preview = atBrightness(preview, strings.ToLower(flag.Arg(1)), uint8(*contrastFlag))
			}
			file, createErr := os.Create(*outputFlag)
			if createErr != nil {
				fatal(createErr)
			}
			if err = png.Encode(file, preview); err != nil {
				_ = file.Close()
				fatal(err)
			}
			if err = file.Close(); err != nil {
				fatal(err)
			}
			fmt.Printf("wrote offline preview to %s\n", *outputFlag)
			if len(validationErrors) != 0 {
				fatal(fmt.Errorf("offline validation failed: %s", strings.Join(validationErrors, "; ")))
			}
			return
		}
		<-ticker.C
		//break
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "fatal: "+err.Error())
	os.Exit(1)
}
