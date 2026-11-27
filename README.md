# Display

[![Go Reference](https://pkg.go.dev/badge/github.com/BeatGlow/display.svg)](https://pkg.go.dev/github.com/BeatGlow/display)

`github.com/BeatGlow/display` provides Go drivers for small OLED, LCD, and VFD displays. Applications can draw pixels, text, and shapes through the same image interfaces across the supported controllers.

## Features

- Drive monochrome, grayscale, and color displays through I²C, SPI, or 8080-style parallel connections.
- Connect to the GU256X64D-7000 through its native 8-bit parallel driver or the Nano Every proxy. The proxy uses 2 MHz SPI or 1 Mbaud USB CDC and accepts monochrome, three-level, and four-level grayscale frames, plus native monochrome bitmap and text commands.
- Use the standard `image.Image`, `draw.Image`, and `color.Color` interfaces to draw to displays.
- Compose graphics with pixel formats and drawing helpers designed for small displays.
- Select a Linux DRM/KMS or fbdev display through `framebuffer`, or open a backend directly from its subpackage.
- Use DRM/KMS output on Linux and FreeBSD 15.1 amd64, with connector and mode selection and vblank-synchronized page flips.
- Use optimized 16-bit image blits, solid spans, RGBA copies, and pixel-buffer fills.

## Rendering benchmarks

These values are the medians of three runs comparing `baseDisplay.Fill` with
per-pixel writes on 400×128 CRGB16 buffers:

| Host | `Fill` | Per-pixel reference |
| --- | ---: | ---: |
| Raspberry Pi 3 | 44.4 µs | 20.39 ms |
| Apple M2 | 2.38 µs | 829 µs |
| AMD Ryzen 9 7900 | 2.42 µs | 481 µs |
| Intel N100 | 4.38 µs | 1.02 ms |

A no-op SPI writer measures chunk-loop overhead for a 102,400-byte payload.
These numbers exclude SPI driver and hardware transfer time:

| Host | 4,096-byte chunks (25 writes) | 16,384-byte chunks (7 writes) | One write |
| --- | ---: | ---: | ---: |
| Raspberry Pi 3 | 1.85 µs | 563 ns | 45.8 ns |
| Apple M2 | 65.5 ns | 15.7 ns | 2.1 ns |
| AMD Ryzen 9 7900 | 44.8 ns | 13.5 ns | 1.34 ns |
| Intel N100 | 119.5 ns | 30.4 ns | 2.72 ns |

Reproduce with `go test . -run '^$' -bench 'Benchmark(SPITransferChunking|DisplayFillCRGB16)$' -benchmem -benchtime=1s -count=3`. See the `pixel`, `draw`, and `framebuffer/kms` package READMEs for conversion results and benchmark commands.

## Display controllers

Drivers are available for SSD1305, SSD1306, SSD1322, SSD1362, SH1106, SH1122, ST7735, ST7789, GU3000, GU256X64D-7000, GP1247AI, GP1287AI/BI, and GP1294AI. See the [Go package reference](https://pkg.go.dev/github.com/BeatGlow/display) for API details.

The Nano Every proxy adds monochrome and three- or four-level grayscale support for the GU256X64D-7000 over SPI or USB. See the [proxy firmware, wiring, and protocol](support/gu256x64d7000_proxy/README.md).

## Draw to any destination

The display controller and framebuffer drivers implement `display.Display`, which satisfies Go's `draw.Image` interface. The same drawing code works with a hardware display, an in-memory image, or another compatible destination. `framebuffer.Open(nil)` tries KMS first, then fbdev, using each backend's defaults. Pass the returned display directly as `dst`:

```go
import (
	"image"
	"image/color"
	"log"

	"github.com/BeatGlow/display/draw"
	"github.com/BeatGlow/display/framebuffer"
)

func main() {
	dst, err := framebuffer.Open(nil)
	if err != nil {
		log.Fatal(err)
	}
	defer dst.Close()

	draw.Box(dst, image.Rect(8, 8, 72, 32), color.White)
	draw.Line(dst, image.Pt(0, 0), image.Pt(127, 63), color.White)
}
```

Each driver also exposes controls such as rotation, contrast, and display power where supported.

## Packages

| Package | Purpose |
| --- | --- |
| `display` | Display interface, controller drivers, and hardware connections |
| `conn` | Direct I²C and SPI device access |
| `pixel` | Packed monochrome, grayscale, and RGB images, plus planar YU12 images |
| `draw` | Image composition, anti-aliased lines, and shape drawing helpers |
| `framebuffer` | Automatic Linux/FreeBSD DRM/KMS and Linux fbdev selection |
| `framebuffer/fbdev` | Linux fbdev output |
| `framebuffer/kms` | Linux and FreeBSD 15.1 amd64 DRM/KMS output with vblank-synchronized page flips |
| `streamdeck` | [Elgato Stream Deck + keys, dials, and touch strip](streamdeck/README.md) on Linux and macOS |

The KMS backend uses optimized CPU pixel conversion to prepare scanout buffers from the library's image formats. See the [`framebuffer/kms` package README](framebuffer/kms/README.md) for benchmarks and end-to-end performance results.

## Install

```sh
go get github.com/BeatGlow/display
```

The library is distributed under the [MIT License](LICENSE.md).
