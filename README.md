# Display

## Offline display test

`cmd/display-test` can exercise the drawing code and render a PNG without
connecting a display. Use `offline` as the bus and `virtual` as the driver to
choose any supported framebuffer color model and dimensions:

```sh
go run ./cmd/display-test -width 256 -height 64 -color gray4 -output /tmp/preview.png offline virtual
```

Supported virtual color models are `mono`, `gray2`, `gray4`, `rgb15`, and
`rgb16`. The generated PNG uses the selected size. The demo draws a border,
gradient or monochrome pattern, and logo.

The same offline connection can run any supported display driver, capture its
command/data traffic, verify the emitted framebuffer encoding, and reconstruct
the received display memory for the PNG:

```sh
go run ./cmd/display-test -contrast 128 -output /tmp/ssd1306.png -trace /tmp/ssd1306-trace.json offline ssd1306
```

Driver-backed previews use that driver's color model and default dimensions;
`-width` and `-height` override them where supported. No hardware writes occur
in offline mode. `-contrast` exercises and checks the driver's contrast command
or backlight PWM when supported; the PNG applies a linear global brightness
scale as a visual estimate. The JSON trace records exact command and data
bytes for CI artifacts. The `virtual` driver remains available for framebuffer
dimensions and color-model previews without a specific controller protocol.

For a CI matrix, run the offline command once for each driver and give each
run its own PNG and trace paths. Supported driver names are `gu3000`, `gp1247ai`,
`gp1278`, `gu256x64d7000`, `gp1294`, `sh1106`, `ssd1305`, `ssd1306`, `ssd1322`,
`sh1122`, `st7735`, and `st7789`.

```sh
mkdir -p artifacts
for driver in gu3000 gp1247ai gu256x64d7000 gp1278 gp1294 sh1106 ssd1305 ssd1306 ssd1322 sh1122 st7735 st7789; do
  go run ./cmd/display-test -contrast 128 -output "artifacts/$driver.png" -trace "artifacts/$driver.json" offline "$driver"
done
```

## KMS performance

Single-goroutine Go benchmarks for RGBA-to-XRGB8888 conversion at 400×1280:

| Test system | SIMD | SIMD time | Scalar time | Gain |
| --- | --- | ---: | ---: | ---: |
| Raspberry Pi 3, Linux ARM64 | NEON | 2.61 ms | 19.23 ms | 7.4× |
| Apple M2, macOS ARM64 | NEON | 60.4 µs | 991 µs | 16.4× |
| AMD Ryzen 9 7900, Linux x86_64 | SSE2 | 59.9 µs | 712 µs | 11.9× |
| Intel N100, Linux x86_64 | SSE2 | 131 µs | 1.52 ms | 11.6× |

The KMS demo also sustained about 51 FPS on the Pi 3 at 400×1280 with a 60 FPS target. This end-to-end figure includes rendering, scanout conversion, and synchronized page flips.

## GU256X64D-7000 VFD

The GU256X64D-7000 driver uses the same API for the module's 8-bit parallel
and asynchronous serial interfaces. Pass a `Conn` from `OpenParallel` for
parallel mode, or open the serial port with `OpenUART`:

```go
import "periph.io/x/conn/v3/gpio/gpioreg"

conn, err := display.OpenUART(&display.UARTConfig{
	Port: "/dev/ttyUSB0",
	Baud: 38400,
	Busy: gpioreg.ByName("GPIO17"), // optional, through an SBUSY level shifter
})
if err != nil {
	return err
}
defer conn.Close()

screen, err := display.GU256X64D7000(conn, &display.Config{})
if err != nil {
	return err
}
```

The serial input is RS-232 level, so connect a TTL UART through a suitable
RS-232 level converter. The module defaults to 38400 baud when both baud
selection jumpers are open; set `UARTConfig.Baud` to match its jumper setting.
You can pass the optional `UARTConfig.Busy` pin to honor the module's SBUSY
signal (use a level shifter for this output too). The driver uses a monochrome
framebuffer (one bit per dot); the module's brightness setting changes the
whole display through eight global levels. The offline check verifies the
bitmap command, transfer size, framebuffer data, brightness command, and
reconstructed display image.

For parallel mode, pass a connection from `display.OpenParallel` with the
module's D0-D7 and /WR pins configured to the same driver constructor.

The command-line demo can open the serial interface with:

```sh
go run ./cmd/display-test -uart-port /dev/ttyUSB0 -uart-baud 38400 uart gu256x64d7000
```

See [the datasheet](doc/GU256X64D-7000.pdf) for interface, command, and
electrical details.

The GP1247AI driver supports the 253x63 Futaba VFD through an SPI connection.
Use `offline gp1247ai` to verify its 8-bit-page framebuffer transfer and
capture a reconstructed preview. The GP1287BI 256x50 and GP1294AI 256x48
drivers are also supported; see their panel and carrier specifications in
`doc/`.
