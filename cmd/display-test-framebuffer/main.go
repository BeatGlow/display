// Command display-test-framebuffer runs a visual and API check on a framebuffer output.
package main

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/BeatGlow/display"
	"github.com/BeatGlow/display/framebuffer"
	"github.com/BeatGlow/display/framebuffer/fbdev"
	"github.com/BeatGlow/display/framebuffer/kms"
)

//go:embed assets/beatglow-gray4.png
var logoPNG []byte

var rotoBackground *image.RGBA

var rotoFrames int

var rotoBackgroundTime, rotoRingTime, rotoEqualizerTime, rotoLogoTime, rotoMarkerTime, fpsOverlayTime time.Duration

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "display-test-framebuffer:", err)
		os.Exit(1)
	}
}

func run() (err error) {
	backend := flag.String("backend", "auto", "output backend: auto, kms, or fbdev")
	kmsDevice := flag.String("kms-device", "/dev/dri/card0", "DRM card path")
	fbdevDevice := flag.String("fbdev-device", "/dev/fb0", "Linux fbdev path")
	connector := flag.String("connector", "", "connected output name, for example HDMI-A-1")
	width := flag.Int("width", 0, "exact output mode width (default: preferred mode)")
	height := flag.Int("height", 0, "exact output mode height (default: preferred mode)")
	duration := flag.Duration("duration", 0, "run time; 0 runs until interrupted")
	fps := flag.Int("fps", 60, "animation refresh rate (1-120 Hz; KMS waits for vblank)")
	flag.Parse()
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flag.Args())
	}
	if *duration < 0 {
		return errors.New("duration must not be negative")
	}
	if *fps < 1 || *fps > 120 {
		return errors.New("fps must be between 1 and 120")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	kmsConfig := &kms.Config{
		Device:    *kmsDevice,
		Connector: *connector,
		Width:     *width,
		Height:    *height,
	}
	var screen display.Display
	switch *backend {
	case "auto":
		screen, err = framebuffer.Open(&framebuffer.Config{KMS: kmsConfig, FBDev: *fbdevDevice})
	case "kms":
		screen, err = kms.Open(kmsConfig)
	case "fbdev":
		screen, err = fbdev.Open(*fbdevDevice)
	default:
		return fmt.Errorf("unknown backend %q; choose auto, kms, or fbdev", *backend)
	}
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, screen.Close()) }()

	bounds := screen.Bounds()
	if bounds.Empty() {
		return fmt.Errorf("framebuffer returned empty screen bounds %v", bounds)
	}
	logo, err := loadLogo()
	if err != nil {
		return err
	}
	if err := checkDisplayAPI(screen); err != nil {
		return err
	}

	fmt.Printf("Testing %s (requested backend: %s)\n", screen, *backend)
	fmt.Println("The color chart checks channel order and brightness; the final scene is a BeatGlow logo rotozoom.")
	fmt.Println("Press Ctrl-C to stop and restore the previous display mode.")

	// Exercise display visibility controls before starting the visible test pattern.
	if err := screen.Show(false); err != nil {
		return fmt.Errorf("turn display off: %w", err)
	}
	select {
	case <-ctx.Done():
		return nil
	case <-time.After(180 * time.Millisecond):
	}
	if err := screen.Show(true); err != nil {
		return fmt.Errorf("turn display on: %w", err)
	}

	canvas := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	directDraw := false
	if buffered, ok := screen.(interface{ DrawBuffer() *image.RGBA }); ok {
		canvas = buffered.DrawBuffer()
		directDraw = true
	}
	ticker := time.NewTicker(time.Second / time.Duration(*fps))
	defer ticker.Stop()
	started := time.Now()
	chartDuration := 2200 * time.Millisecond
	if *duration > 0 && *duration/4 < chartDuration {
		chartDuration = *duration / 4
	}
	rotating := false
	var frames uint64
	var previousFrame time.Time
	var measuredFPS float64
	var drawTime, copyTime, refreshTime time.Duration
	defer func() {
		elapsed := time.Since(started).Seconds()
		if elapsed > 0 {
			fmt.Printf("Completed %d frames in %.1fs (%.1f Hz); average draw %.1f ms, copy %.1f ms, refresh %.1f ms.\n",
				frames,
				elapsed,
				float64(frames)/elapsed,
				float64(drawTime.Microseconds())/float64(max(frames, 1))/1000,
				float64(copyTime.Microseconds())/float64(max(frames, 1))/1000,
				float64(refreshTime.Microseconds())/float64(max(frames, 1))/1000,
			)
			fmt.Printf("Average rotozoom stages over %d frames: background %.2f ms, rings %.2f ms, equalizer %.2f ms, logo %.2f ms, markers %.2f ms, FPS label %.2f ms.\n",
				rotoFrames,
				float64(rotoBackgroundTime.Microseconds())/float64(max(rotoFrames, 1))/1000,
				float64(rotoRingTime.Microseconds())/float64(max(rotoFrames, 1))/1000,
				float64(rotoEqualizerTime.Microseconds())/float64(max(rotoFrames, 1))/1000,
				float64(rotoLogoTime.Microseconds())/float64(max(rotoFrames, 1))/1000,
				float64(rotoMarkerTime.Microseconds())/float64(max(rotoFrames, 1))/1000,
				float64(fpsOverlayTime.Microseconds())/float64(max(frames, 1))/1000,
			)
		}
	}()
	for {
		elapsed := time.Since(started)
		if *duration > 0 && elapsed >= *duration {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		elapsed = time.Since(started)
		if *duration > 0 && elapsed >= *duration {
			return nil
		}
		frameStarted := time.Now()
		if !previousFrame.IsZero() {
			instantFPS := 1 / frameStarted.Sub(previousFrame).Seconds()
			if measuredFPS == 0 {
				measuredFPS = instantFPS
			} else {
				measuredFPS = measuredFPS*0.8 + instantFPS*0.2
			}
		}
		drawStarted := time.Now()
		if elapsed < chartDuration {
			drawColorChart(canvas, logo, elapsed.Seconds())
		} else {
			if !rotating {
				fmt.Println("Color and grayscale checks complete; starting the logo rotozoom.")
				rotating = true
			}
			drawRotozoom(canvas, logo, elapsed.Seconds()-chartDuration.Seconds())
		}
		fpsStarted := time.Now()
		drawFPS(canvas, measuredFPS)
		fpsOverlayTime += time.Since(fpsStarted)
		drawTime += time.Since(drawStarted)
		if !directDraw {
			copyStarted := time.Now()
			if err := copyFrame(screen, canvas); err != nil {
				return err
			}
			copyTime += time.Since(copyStarted)
		}
		refreshStarted := time.Now()
		if err := screen.Refresh(); err != nil {
			return fmt.Errorf("refresh framebuffer output: %w", err)
		}
		refreshTime += time.Since(refreshStarted)
		frames++
		previousFrame = frameStarted
	}
}

func copyFrame(screen display.Display, frame *image.RGBA) error {
	if copier, ok := screen.(interface{ CopyFrom(image.Image) error }); ok {
		return copier.CopyFrom(frame)
	}
	draw.Draw(screen, screen.Bounds(), frame, frame.Bounds().Min, draw.Src)
	return nil
}

func drawFPS(dst *image.RGBA, fps float64) {
	label := fmt.Sprintf("%4.1f FPS", fps)
	width := len(label)*7 + 14
	if dst.Bounds().Dx() < width+12 || dst.Bounds().Dy() < 26 {
		return
	}
	fillRect(dst, image.Rect(6, 6, 6+width, 26), color.RGBA{R: 5, G: 10, B: 24, A: 255})
	drawer := font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(color.RGBA{R: 70, G: 245, B: 255, A: 255}),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(13, 21),
	}
	drawer.DrawString(label)
}

func checkDisplayAPI(screen display.Display) error {
	if err := screen.SetRotation(display.NoRotation); err != nil {
		return fmt.Errorf("set no rotation: %w", err)
	}
	if err := screen.SetContrast(128); err != nil && !errors.Is(err, kms.ErrNotSupported) {
		return fmt.Errorf("set contrast: %w", err)
	}
	if err := screen.SetRotation(display.Rotate90); err != nil && !errors.Is(err, kms.ErrNotSupported) {
		return fmt.Errorf("set 90-degree rotation: %w", err)
	}
	if err := screen.SetRotation(display.NoRotation); err != nil {
		return fmt.Errorf("restore no rotation: %w", err)
	}

	probe := screen.Bounds().Min
	screen.Fill(color.RGBA{R: 19, G: 113, B: 227, A: 255})
	if got := color.RGBAModel.Convert(screen.At(probe.X, probe.Y)).(color.RGBA); got != (color.RGBA{R: 19, G: 113, B: 227, A: 255}) {
		return fmt.Errorf("Fill/At mismatch: got %#v", got)
	}
	screen.Clear()
	if got := color.RGBAModel.Convert(screen.At(probe.X, probe.Y)).(color.RGBA); got != (color.RGBA{A: 255}) {
		return fmt.Errorf("Clear/At mismatch: got %#v", got)
	}
	setColor := color.RGBA{R: 211, G: 47, B: 109, A: 255}
	screen.Set(probe.X, probe.Y, setColor)
	if got := color.RGBAModel.Convert(screen.At(probe.X, probe.Y)).(color.RGBA); got != setColor {
		return fmt.Errorf("Set/At mismatch: got %#v, want %#v", got, setColor)
	}
	screen.Clear()
	return nil
}

func loadLogo() (*image.Gray, error) {
	decoded, err := png.Decode(bytes.NewReader(logoPNG))
	if err != nil {
		return nil, fmt.Errorf("decode embedded BeatGlow logo: %w", err)
	}
	gray := image.NewGray(decoded.Bounds())
	for y := gray.Bounds().Min.Y; y < gray.Bounds().Max.Y; y++ {
		for x := gray.Bounds().Min.X; x < gray.Bounds().Max.X; x++ {
			gray.Set(x, y, color.GrayModel.Convert(decoded.At(x, y)))
		}
	}
	return gray, nil
}

func drawColorChart(dst *image.RGBA, logo *image.Gray, elapsed float64) {
	bounds := dst.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	columns := make([][3]uint8, w)
	for x := range w {
		r, g, b := hsv(float64(x)/float64(w)+elapsed*0.08, 0.8, 1)
		columns[x] = [3]uint8{r, g, b}
	}
	for y := 0; y < h; y++ {
		brightness := 0.08 + 0.13*float64(h-y)/float64(h)
		for x := 0; x < w; x++ {
			c := columns[x]
			setPixel(dst, x, y, color.RGBA{
				R: uint8(float64(c[0]) * brightness),
				G: uint8(float64(c[1]) * brightness),
				B: uint8(float64(c[2]) * brightness),
				A: 255,
			})
		}
	}
	palette := []color.RGBA{
		{R: 255, A: 255}, {G: 255, A: 255}, {B: 255, A: 255},
		{R: 255, G: 255, A: 255}, {R: 255, B: 255, A: 255}, {G: 255, B: 255, A: 255},
		{R: 255, G: 255, B: 255, A: 255},
	}
	barY := h * 48 / 100
	barH := max(2, h*18/100)
	gap := max(1, w/180)
	barW := max(1, (w-gap*(len(palette)+1))/len(palette))
	for i, c := range palette {
		x := gap + i*(barW+gap)
		fillRect(dst, image.Rect(x, barY, min(x+barW, w-gap), min(barY+barH, h)), c)
	}
	grayY := h * 76 / 100
	grayH := max(2, h*8/100)
	for i := 0; i < 16; i++ {
		level := uint8(i * 17)
		x0 := i * w / 16
		x1 := (i + 1) * w / 16
		fillRect(dst, image.Rect(x0, grayY, x1, min(grayY+grayH, h)), color.RGBA{R: level, G: level, B: level, A: 255})
	}
	stripY := h * 91 / 100
	for x := 0; x < w; x++ {
		r, g, b := hsv(float64(x)/float64(max(w, 1))+elapsed*0.1, 1, 1)
		fillRect(dst, image.Rect(x, stripY, x+1, h), color.RGBA{R: r, G: g, B: b, A: 255})
	}
	logoScale := float64(min(w, h)) * 0.30 / float64(logo.Bounds().Dx())
	paintLogo(dst, logo, float64(w)/2, float64(h)*0.25, logoScale, 0, elapsed*0.05)
	drawBorder(dst, color.RGBA{R: 70, G: 220, B: 255, A: 255})
}

func drawRotozoom(dst *image.RGBA, logo *image.Gray, elapsed float64) {
	rotoFrames++
	bounds := dst.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	backgroundStarted := time.Now()
	if rotoBackground == nil || rotoBackground.Bounds().Size() != bounds.Size() {
		rotoBackground = image.NewRGBA(bounds)
		drawRotozoomBackground(rotoBackground)
	}
	copy(dst.Pix, rotoBackground.Pix)
	rotoBackgroundTime += time.Since(backgroundStarted)
	centerX, centerY := float64(w)/2, float64(h)/2
	minDimension := float64(min(w, h))
	ringsStarted := time.Now()
	for ring := 0; ring < 3; ring++ {
		radius := minDimension * (0.26 + float64(ring)*0.11 + 0.012*math.Sin(elapsed*2+float64(ring)))
		for step := range 360 {
			angle := float64(step)*math.Pi/180 + elapsed*(0.10+float64(ring)*0.025)
			x := int(math.Round(centerX + math.Cos(angle)*radius))
			y := int(math.Round(centerY + math.Sin(angle)*radius))
			r, g, b := hsv(float64(step)/360+elapsed*0.04, 0.95, 0.9)
			if image.Pt(x, y).In(dst.Bounds()) {
				setPixel(dst, x, y, color.RGBA{R: r, G: g, B: b, A: 255})
			}
		}
	}
	rotoRingTime += time.Since(ringsStarted)
	equalizerStarted := time.Now()
	drawEqualizer(dst, elapsed)
	rotoEqualizerTime += time.Since(equalizerStarted)
	baseScale := minDimension * 0.58 / float64(logo.Bounds().Dx())
	scale := baseScale * (0.86 + 0.12*math.Sin(elapsed*1.7))
	logoStarted := time.Now()
	paintLogo(dst, logo, centerX, centerY, scale, elapsed*0.92, elapsed*0.07)
	rotoLogoTime += time.Since(logoStarted)
	// Add two bright corner markers to make the panel edges and orientation clear.
	markersStarted := time.Now()
	marker := max(2, min(w, h)/55)
	for _, point := range []image.Point{{X: 8, Y: 8}, {X: w - 9, Y: 8}, {X: 8, Y: h - 9}, {X: w - 9, Y: h - 9}} {
		fillRect(dst, image.Rect(point.X-marker, point.Y-marker, point.X+marker+1, point.Y+marker+1), color.RGBA{R: 245, G: 249, B: 255, A: 255})
	}
	rotoMarkerTime += time.Since(markersStarted)
}

func drawRotozoomBackground(dst *image.RGBA) {
	w, h := dst.Bounds().Dx(), dst.Bounds().Dy()
	colors := make([][3]uint8, w)
	for x := range w {
		r, g, b := hsv(float64(x)/float64(max(w, 1)), 0.95, 1)
		colors[x] = [3]uint8{r, g, b}
	}
	for y := range h {
		level := uint16(26 + 36*(h-y)/max(h, 1))
		row := dst.Pix[y*dst.Stride:]
		for x, c := range colors {
			i := x * 4
			row[i], row[i+1], row[i+2], row[i+3] = uint8(uint16(c[0])*level>>8), uint8(uint16(c[1])*level>>8), uint8(uint16(c[2])*level>>8), 255
		}
	}
}

func drawEqualizer(dst *image.RGBA, elapsed float64) {
	w, h := dst.Bounds().Dx(), dst.Bounds().Dy()
	count := min(36, max(12, w/24))
	barWidth := max(1, w/(count*2))
	gap := max(1, (w-count*barWidth)/(count+1))
	baseline := h * 88 / 100
	for i := range count {
		wave := (math.Sin(elapsed*3+float64(i)*0.46) + 1) / 2
		barHeight := max(2, int(float64(h)*0.04+wave*float64(h)*0.11))
		x := gap + i*(barWidth+gap)
		r, g, b := hsv(float64(i)/float64(count)+elapsed*0.035, 0.9, 0.95)
		fillRect(dst, image.Rect(x, baseline-barHeight, min(x+barWidth, w), baseline), color.RGBA{R: r, G: g, B: b, A: 255})
	}
}

func paintLogo(dst *image.RGBA, logo *image.Gray, centerX, centerY, scale, angle, hueOffset float64) {
	width, height := float64(logo.Bounds().Dx()), float64(logo.Bounds().Dy())
	cosine, sine := math.Cos(angle), math.Sin(angle)
	stepX, stepY := cosine/scale, -sine/scale
	stepXFixed := int64(stepX * 65536)
	stepYFixed := int64(stepY * 65536)
	extentX := (math.Abs(cosine)*width + math.Abs(sine)*height) * scale / 2
	extentY := (math.Abs(sine)*width + math.Abs(cosine)*height) * scale / 2
	minX := max(0, int(math.Floor(centerX-extentX)))
	maxX := min(dst.Bounds().Dx(), int(math.Ceil(centerX+extentX)))
	minY := max(0, int(math.Floor(centerY-extentY)))
	maxY := min(dst.Bounds().Dy(), int(math.Ceil(centerY+extentY)))
	palette := make([][3]uint8, int(width))
	for x := range palette {
		palette[x][0], palette[x][1], palette[x][2] = hsv(float64(x)/width+hueOffset, 0.78, 1)
	}
	for y := minY; y < maxY; y++ {
		dx := float64(minX) + 0.5 - centerX
		dy := float64(y) + 0.5 - centerY
		sourceX := int64(((dx*cosine+dy*sine)/scale + width/2) * 65536)
		sourceY := int64(((-dx*sine+dy*cosine)/scale + height/2) * 65536)
		rowOffset := y*dst.Stride + minX*4
		for x := minX; x < maxX; x++ {
			if sourceX >= 0 && sourceY >= 0 && sourceX < int64(width*65536) && sourceY < int64(height*65536) {
				sx, sy := int(sourceX>>16), int(sourceY>>16)
				alpha := 255 - logo.Pix[sy*logo.Stride+sx]
				if alpha >= 4 {
					c := palette[sx]
					i := rowOffset
					inverse := uint16(255 - alpha)
					a := uint16(alpha)
					dst.Pix[i] = uint8((uint16(c[0])*a + uint16(dst.Pix[i])*inverse) / 255)
					dst.Pix[i+1] = uint8((uint16(c[1])*a + uint16(dst.Pix[i+1])*inverse) / 255)
					dst.Pix[i+2] = uint8((uint16(c[2])*a + uint16(dst.Pix[i+2])*inverse) / 255)
					dst.Pix[i+3] = 255
				}
			}
			sourceX += stepXFixed
			sourceY += stepYFixed
			rowOffset += 4
		}
	}
}

func drawBorder(dst *image.RGBA, c color.RGBA) {
	w, h := dst.Bounds().Dx(), dst.Bounds().Dy()
	if w == 0 || h == 0 {
		return
	}
	for x := range w {
		setPixel(dst, x, 0, c)
		setPixel(dst, x, h-1, c)
	}
	for y := range h {
		setPixel(dst, 0, y, c)
		setPixel(dst, w-1, y, c)
	}
}

func fillRect(dst *image.RGBA, rect image.Rectangle, c color.RGBA) {
	rect = rect.Intersect(dst.Bounds())
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			setPixel(dst, x, y, c)
		}
	}
}

func setPixel(dst *image.RGBA, x, y int, c color.RGBA) {
	index := y*dst.Stride + x*4
	dst.Pix[index], dst.Pix[index+1], dst.Pix[index+2], dst.Pix[index+3] = c.R, c.G, c.B, c.A
}

func hsv(hue, saturation, value float64) (uint8, uint8, uint8) {
	hue = math.Mod(hue, 1)
	if hue < 0 {
		hue++
	}
	sector := hue * 6
	chroma := value * saturation
	x := chroma * (1 - math.Abs(math.Mod(sector, 2)-1))
	var red, green, blue float64
	switch int(sector) {
	case 0:
		red, green = chroma, x
	case 1:
		red, green = x, chroma
	case 2:
		green, blue = chroma, x
	case 3:
		green, blue = x, chroma
	case 4:
		red, blue = x, chroma
	default:
		red, blue = chroma, x
	}
	match := value - chroma
	return uint8((red + match) * 255), uint8((green + match) * 255), uint8((blue + match) * 255)
}
