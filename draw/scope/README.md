# SVG oscilloscope splash screens

The scope package compiles SVG outlines into a repeating X-Y DAC signal. A
Renderer owns its signal, filter state, phosphor charge, blur buffers, optical
remap, and color tables. Create it once for the destination bounds, then update
and render it from one goroutine.

The [BeatGlow splash SVG](testdata/beatglow.svg) contains monochrome outlines of
the waveform-circle mark and Fugaz One wordmark. The runnable
[example](example_test.go) embeds this asset and renders it at 400×128 with
`CRTBulge = 0.31`.

```go
cfg := scope.DefaultConfig()
renderer, err := scope.NewRenderer(dst.Bounds(), cfg)
if err != nil {
    return err
}
if err := renderer.CompileSVG(strings.NewReader(splashSVG)); err != nil {
    return err
}

// Call from the application's display loop with measured elapsed time.
renderer.UpdatePhysics(time.Second / 60)
if err := renderer.BlitToDisplay(dst); err != nil {
    return err
}
```

For hardware output, dst is a display.Display. BlitToDisplay calls RenderTo
then dst.Refresh(). RenderTo also accepts an in-memory image/draw.Image and
leaves presentation to the caller. The application controls screen power with
dst.Show(true).

CompileSVG handles paths, lines, rectangles, circles, ellipses, polygons and
polylines, including nested affine transforms, Bézier curves and arcs. Convert
text and SVG clones to paths first. Paint, masks and stroke widths are ignored.
Compilation can allocate. If it fails, the previous drawing remains in place.

UpdatePhysics advances the repeating signal, applies white thermal noise,
optional mains hum, AC coupling, vector slew limiting and a second-order
Butterworth low-pass, then decays and deposits phosphor charge. StepSignal
prepares a signal frame for the next UpdatePhysics call, which consumes it
without advancing the DAC again. Calling StepSignal again replaces the pending
frame. Use UpdatePhysics for the normal frame loop. Zero or negative time steps
do nothing. Steps longer than one second are limited to one second to cap
catch-up work after a pause.

RenderTo uses precomputed bilinear curvature coordinates and color tables.
Color destinations receive green phosphor with pale highlights. Gray3Level,
Gray2, Gray4, and standard grayscale destinations receive luminance with gamma
1.8.
Monochrome destinations receive a fixed 4×4 Bayer pattern. The packed CRGB16,
Gray2, Gray4 and mono formats and standard RGBA have direct buffer paths.
For other draw.Image destinations, RenderTo calls Set with preboxed colors and
respects driver rotation. Heights below 128 pixels use sparse grid
intersections.
The default beam radius is derived from the height and capped at 2 pixels
for the 7×7 electron kernel. Explicit radii range from 0.1 to 2 pixels.

RenderTo and UpdatePhysics allocate no heap memory after initialization and
compilation. Hardware Refresh may allocate or block according to its driver.
Recreate the renderer after changing display bounds or static configuration.
Renderer owns no goroutine or device resources, so it needs no Close call.

Run the CPU frame benchmark with:

```sh
go test ./draw/scope -run '^$' -bench BenchmarkRenderFrame -benchmem
```

The benchmark includes a 60 Hz physics step, decay, bloom, remapping and buffer
writes at 400×128. It excludes device transfer time. Measure on the intended
hardware before using its result as a frame-budget guarantee.

A 1-second benchmark run on an Apple M2 (darwin/arm64), 400×128,
measured 1.12 ms for RGBA, 1.19 ms for CRGB16, 1.25 ms for Gray4 and
1.27 ms for Mono. All reported 0 B/op and 0 allocs/op.
These are host CPU measurements. Raspberry Pi and display-transfer timing
have not been measured.
