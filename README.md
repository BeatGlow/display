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
run its own PNG and trace paths. Supported driver names are `gu3000`, `gp1278`,
`gp1294`, `sh1106`, `ssd1305`, `ssd1306`, `ssd1322`,
`sh1122`, `st7735`, and `st7789`.

```sh
mkdir -p artifacts
for driver in gu3000 gp1278 gp1294 sh1106 ssd1305 ssd1306 ssd1322 sh1122 st7735 st7789; do
  go run ./cmd/display-test -contrast 128 -output "artifacts/$driver.png" -trace "artifacts/$driver.json" offline "$driver"
done
```
