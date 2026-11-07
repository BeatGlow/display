# Display

[![Go Reference](https://pkg.go.dev/badge/github.com/BeatGlow/display.svg)](https://pkg.go.dev/github.com/BeatGlow/display)

`github.com/BeatGlow/display` is a Go library for drawing to small OLED, LCD, and VFD displays. It pairs hardware display drivers with Go's standard image interfaces, so graphics code can work across different controllers and display types.

## Features

- Monochrome, grayscale, and color display support.
- I²C, SPI, UART, and 8080-style parallel connections.
- Linux framebuffer and DRM/KMS display backends.
- CPU-accelerated pixel conversion for KMS scanout buffers.
- Image composition, shape drawing, packed pixel formats, fonts, widgets, and menus.
- A KMS demonstration utility with a color and brightness chart, frame-rate display, and animated logo.

## Display support

Drivers support display families including SSD1305, SSD1306, SSD1322, SSD1362, SH1106, SH1122, ST7735, ST7789, GU3000, GU256X64D-7000, GP1247AI, GP1287BI, and GP1294AI. See the [Go package reference](https://pkg.go.dev/github.com/BeatGlow/display) for API details.

## Draw with Go image interfaces

Display drivers implement `display.Display`, which also satisfies Go's `draw.Image` interface. The same drawing code can target a hardware display or any compatible image:

```go
import (
	"image"
	"image/color"

	"github.com/BeatGlow/display/draw"
)

func render(dst draw.Image) {
	draw.Box(dst, image.Rect(8, 8, 72, 32), color.White)
	draw.Line(dst, image.Pt(0, 0), image.Pt(127, 63), color.White)
}
```

After drawing to a display's image buffer, call `Refresh` to update the panel. Drivers expose display controls such as rotation, contrast, and power where supported.

## Packages

| Package | Purpose |
| --- | --- |
| `display` | Display interface, hardware connections, and panel drivers |
| `pixel` | Packed monochrome, grayscale, and RGB image formats |
| `draw` | Image composition and shape drawing helpers |
| `draw/menu` | Menu model for compact interfaces |
| `ui` and `ui/font` | Widgets, styles, themes, and text rendering |
| `framebuffer` | Linux framebuffer as a display destination |
| `kms` | Linux DRM/KMS output with mode selection and synchronized page flips |

See the [`kms` package README](kms/README.md) for pixel-conversion benchmarks and end-to-end KMS performance results.

## Install

```sh
go get github.com/BeatGlow/display
```

The library is distributed under the [MIT License](LICENSE.md).
