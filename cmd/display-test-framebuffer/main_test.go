package main

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/BeatGlow/display"
	"github.com/BeatGlow/display/framebuffer/kms"
)

type testFramebuffer struct {
	img          *image.RGBA
	contrastErr  error
	rotationErr  map[display.Rotation]error
	rotationCall int
}

func newTestFramebuffer(width, height int) *testFramebuffer {
	return &testFramebuffer{img: image.NewRGBA(image.Rect(0, 0, width, height))}
}

func (f *testFramebuffer) ColorModel() color.Model     { return color.RGBAModel }
func (f *testFramebuffer) Bounds() image.Rectangle     { return f.img.Bounds() }
func (f *testFramebuffer) At(x, y int) color.Color     { return f.img.At(x, y) }
func (f *testFramebuffer) Set(x, y int, c color.Color) { f.img.Set(x, y, c) }
func (f *testFramebuffer) String() string              { return "test framebuffer" }
func (f *testFramebuffer) Close() error                { return nil }
func (f *testFramebuffer) Clear()                      { f.Fill(color.Black) }
func (f *testFramebuffer) Fill(c color.Color) {
	for y := f.Bounds().Min.Y; y < f.Bounds().Max.Y; y++ {
		for x := f.Bounds().Min.X; x < f.Bounds().Max.X; x++ {
			f.Set(x, y, c)
		}
	}
}
func (f *testFramebuffer) Show(bool) error         { return nil }
func (f *testFramebuffer) SetContrast(uint8) error { return f.contrastErr }
func (f *testFramebuffer) SetRotation(rotation display.Rotation) error {
	f.rotationCall++
	return f.rotationErr[rotation]
}
func (*testFramebuffer) Refresh() error { return nil }

type copyTestFramebuffer struct {
	*testFramebuffer
	copyErr error
}

func (f *copyTestFramebuffer) CopyFrom(src image.Image) error {
	if f.copyErr != nil {
		return f.copyErr
	}
	imageDraw(f.img, f.img.Bounds(), src, src.Bounds().Min)
	return nil
}

func imageDraw(dst *image.RGBA, r image.Rectangle, src image.Image, sp image.Point) {
	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			dst.Set(r.Min.X+x, r.Min.Y+y, src.At(sp.X+x, sp.Y+y))
		}
	}
}

func TestScreenshotAndFrameCopy(t *testing.T) {
	screen := newTestFramebuffer(4, 3)
	screen.Set(1, 2, color.RGBA{R: 21, G: 83, B: 177, A: 255})
	path := filepath.Join(t.TempDir(), "frame.png")
	if err := saveScreenshot(screen, path); err != nil {
		t.Fatalf("saveScreenshot: %v", err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	decoded, decodeErr := png.Decode(file)
	closeErr := file.Close()
	if decodeErr != nil || closeErr != nil {
		t.Fatalf("decode saved screenshot: decode=%v close=%v", decodeErr, closeErr)
	}
	if got := color.RGBAModel.Convert(decoded.At(1, 2)).(color.RGBA); got != (color.RGBA{R: 21, G: 83, B: 177, A: 255}) {
		t.Fatalf("screenshot pixel = %#v", got)
	}
	if err := saveScreenshot(screen, filepath.Join(t.TempDir(), "missing", "frame.png")); err == nil {
		t.Fatal("saveScreenshot accepted a missing destination directory")
	}

	frame := image.NewRGBA(image.Rect(0, 0, 4, 3))
	frame.Set(0, 0, color.RGBA{R: 200, G: 30, B: 10, A: 255})
	fallback := newTestFramebuffer(4, 3)
	if err := copyFrame(fallback, frame); err != nil {
		t.Fatalf("fallback copyFrame: %v", err)
	}
	if got := fallback.img.RGBAAt(0, 0); got != (color.RGBA{R: 200, G: 30, B: 10, A: 255}) {
		t.Fatalf("fallback copied pixel = %#v", got)
	}
	copying := &copyTestFramebuffer{testFramebuffer: newTestFramebuffer(4, 3)}
	if err := copyFrame(copying, frame); err != nil {
		t.Fatalf("CopyFrom copyFrame: %v", err)
	}
	copyErr := errors.New("copy failed")
	copying.copyErr = copyErr
	if err := copyFrame(copying, frame); !errors.Is(err, copyErr) {
		t.Fatalf("CopyFrom error = %v, want %v", err, copyErr)
	}
}

func TestDisplayAPICheck(t *testing.T) {
	if err := checkDisplayAPI(newTestFramebuffer(8, 8)); err != nil {
		t.Fatalf("checkDisplayAPI: %v", err)
	}
	unsupported := &testFramebuffer{
		img:         image.NewRGBA(image.Rect(0, 0, 8, 8)),
		contrastErr: kms.ErrNotSupported,
		rotationErr: map[display.Rotation]error{display.Rotate90: kms.ErrNotSupported},
	}
	if err := checkDisplayAPI(unsupported); err != nil {
		t.Fatalf("unsupported optional controls: %v", err)
	}
	rotationErr := errors.New("rotation failed")
	if err := checkDisplayAPI(&testFramebuffer{img: image.NewRGBA(image.Rect(0, 0, 8, 8)), rotationErr: map[display.Rotation]error{display.NoRotation: rotationErr}}); !errors.Is(err, rotationErr) {
		t.Fatalf("rotation error = %v, want %v", err, rotationErr)
	}
	contrastErr := errors.New("contrast failed")
	if err := checkDisplayAPI(&testFramebuffer{img: image.NewRGBA(image.Rect(0, 0, 8, 8)), contrastErr: contrastErr}); !errors.Is(err, contrastErr) {
		t.Fatalf("contrast error = %v, want %v", err, contrastErr)
	}
}

func TestFramebufferDrawingHelpers(t *testing.T) {
	logo, err := loadLogo()
	if err != nil || logo.Bounds().Empty() {
		t.Fatalf("loadLogo: bounds=%v err=%v", logo.Bounds(), err)
	}

	chart := image.NewRGBA(image.Rect(0, 0, 160, 120))
	drawColorChart(chart, logo, 0.5)
	if chart.RGBAAt(0, 0).A != 255 || chart.RGBAAt(159, 119).A != 255 {
		t.Fatal("color chart did not paint the canvas")
	}

	background := image.NewRGBA(image.Rect(0, 0, 160, 120))
	drawRotozoomBackground(background)
	if background.RGBAAt(20, 20).A != 255 {
		t.Fatal("rotozoom background left an unpainted pixel")
	}
	drawEqualizer(background, 1.25)
	paintLogo(background, logo, 80, 60, 0.3, 0.4, 0.2)
	drawBorder(background, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	if got := background.RGBAAt(0, 0); got != (color.RGBA{R: 1, G: 2, B: 3, A: 255}) {
		t.Fatalf("drawBorder corner = %#v", got)
	}

	oldFrames := rotoFrames
	rotoBackground = nil
	drawRotozoom(background, logo, 0.75)
	if rotoFrames != oldFrames+1 || rotoBackground == nil {
		t.Fatalf("drawRotozoom state: frames=%d background=%v", rotoFrames, rotoBackground != nil)
	}
	rotoBackground = nil

	fillRect(background, image.Rect(-4, -3, 3, 2), color.RGBA{R: 20, A: 255})
	if got := background.RGBAAt(0, 0); got != (color.RGBA{R: 20, A: 255}) {
		t.Fatalf("clipped fillRect pixel = %#v", got)
	}
	fillRect(background, image.Rect(300, 300, 302, 302), color.RGBA{A: 255})
	drawBorder(image.NewRGBA(image.Rectangle{}), color.RGBA{A: 255})
	if got := readRGBA(newTestFramebuffer(1, 1), image.Pt(0, 0)); got != (color.RGBA{}) {
		t.Fatalf("readRGBA on blank screen = %#v", got)
	}
	if got := convertToDisplayRGBA(newTestFramebuffer(1, 1), color.RGBA{R: 4, G: 5, B: 6, A: 255}); got != (color.RGBA{R: 4, G: 5, B: 6, A: 255}) {
		t.Fatalf("convertToDisplayRGBA = %#v", got)
	}

	large := image.NewRGBA(image.Rect(0, 0, 120, 40))
	drawFPS(large, 59.9)
	if countOpaque(large) == 0 {
		t.Fatal("drawFPS did not paint a sufficiently large canvas")
	}
	small := image.NewRGBA(image.Rect(0, 0, 8, 8))
	drawFPS(small, math.NaN())
	if countOpaque(small) != 0 {
		t.Fatal("drawFPS modified a too-small canvas")
	}
	for _, tc := range []struct {
		hue  float64
		want color.RGBA
	}{
		{0, color.RGBA{R: 255, A: 255}},
		{1.0 / 3, color.RGBA{G: 255, A: 255}},
		{2.0 / 3, color.RGBA{B: 255, A: 255}},
		{-1.0 / 3, color.RGBA{B: 255, A: 255}},
	} {
		r, g, b := hsv(tc.hue, 1, 1)
		if got := (color.RGBA{R: r, G: g, B: b, A: 255}); got != tc.want {
			t.Errorf("hsv(%v,1,1) = %#v, want %#v", tc.hue, got, tc.want)
		}
	}
}

func countOpaque(img *image.RGBA) int {
	n := 0
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if img.RGBAAt(x, y).A != 0 {
				n++
			}
		}
	}
	return n
}
