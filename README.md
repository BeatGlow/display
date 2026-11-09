# Display

[![Go Reference](https://pkg.go.dev/badge/github.com/BeatGlow/display.svg)](https://pkg.go.dev/github.com/BeatGlow/display)

`github.com/BeatGlow/display` is a Go library for drawing to small OLED, LCD, and VFD displays. It brings hardware display drivers and Go's familiar image interfaces together, so application code can draw pixels, text, and shapes without being tied to one controller.

## What you can build with it

- Drive monochrome, grayscale, and color displays through I²C, SPI, or 8080-style parallel connections.
- Work with displays through the standard `image.Image`, `draw.Image`, and `color.Color` interfaces.
- Compose graphics with pixel formats and drawing helpers designed for small displays.
- Build compact interfaces with the widget, font, and menu packages.
- Select a Linux DRM/KMS or fbdev display through `framebuffer`, or open a backend directly from its subpackage.
- Drive Linux and FreeBSD 15.1 amd64 displays through DRM/KMS, with connector and mode selection plus vblank-synchronized page flips.
- Optimized 16-bit image blits, solid spans, RGBA copies, and pixel-buffer fills.

## Rendering benchmark highlights

Three-run medians for `baseDisplay.Fill` versus per-pixel writes on 400×128
CRGB16 buffers:

| Host | `Fill` | Per-pixel reference |
| --- | ---: | ---: |
| Raspberry Pi 3 | 44.4 µs | 20.39 ms |
| Apple M2 | 2.38 µs | 829 µs |
| AMD Ryzen 9 7900 | 2.42 µs | 481 µs |
| Intel N100 | 4.38 µs | 1.02 ms |

A no-op SPI writer isolates chunk-loop overhead for a 102,400-byte payload; it
excludes SPI driver and hardware transfer time:

| Host | 4,096-byte chunks (25 writes) | 16,384-byte chunks (7 writes) | One write |
| --- | ---: | ---: | ---: |
| Raspberry Pi 3 | 1.85 µs | 563 ns | 45.8 ns |
| Apple M2 | 65.5 ns | 15.7 ns | 2.1 ns |
| AMD Ryzen 9 7900 | 44.8 ns | 13.5 ns | 1.34 ns |
| Intel N100 | 119.5 ns | 30.4 ns | 2.72 ns |

Reproduce with `go test . -run '^$' -bench 'Benchmark(SPITransferChunking|DisplayFillCRGB16)$' -benchmem -benchtime=1s -count=3`. See the `pixel`, `draw`, and `framebuffer/kms` package READMEs for conversion results and benchmark commands.

## Display controllers

The package includes drivers for SSD1305, SSD1306, SSD1322, SSD1362, SH1106, SH1122, ST7735, ST7789, GU3000, GU256X64D-7000, GP1247AI, GP1287BI, and GP1294AI. Check the [Go package reference](https://pkg.go.dev/github.com/BeatGlow/display) for the current API and controller details.

## A familiar drawing model

Display drivers implement `display.Display`, which is also a Go `draw.Image`. Use the same drawing code with a hardware display, an in-memory image, or another compatible destination. `framebuffer.Open(nil)` selects the first available framebuffer backend using its defaults, and the returned display can be passed directly as `dst`:

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

`Display.SetRotation` accepts `NoRotation`, `Rotate90`, `Rotate180`, and
`Rotate270`. Angles are clockwise; 90° and 270° swap the logical width and
height returned by `Bounds`. Draw through the usual `image.Image` methods after
setting the orientation. Linux fbdev and KMS currently rotate in software when
the framebuffer is refreshed. Run `go run ./cmd/display-test-framebuffer
-rotation 90` to exercise the full-resolution framebuffer demo at 90° (use
`-rotation 180` or `-rotation 270` for the other quarter-turns).

## Packages

| Package | Purpose |
| --- | --- |
| `display` | Display interface, controller drivers, and hardware connections |
| `pixel` | Packed monochrome, grayscale, and RGB image formats |
| `draw` | Image composition and shape drawing helpers |
| `draw/menu` | Menu model for compact, button-driven interfaces |
| `ui` and `ui/font` | Widgets, styles, themes, and text rendering |
| `framebuffer` | Automatic Linux/FreeBSD DRM/KMS and Linux fbdev selection |
| `framebuffer/fbdev` | Linux fbdev output |
| `framebuffer/kms` | Linux and FreeBSD 15.1 amd64 DRM/KMS output |

The KMS backend uses optimized CPU pixel conversion to prepare scanout buffers from the library's image formats. See the [`framebuffer/kms` package README](framebuffer/kms/README.md) for benchmarks and end-to-end performance results.

## Install

```sh
go get github.com/BeatGlow/display
```

The library is distributed under the [MIT License](LICENSE.md).
