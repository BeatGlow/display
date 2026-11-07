//go:build linux || freebsd

package framebuffer

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"io/fs"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/BeatGlow/display"
	"github.com/BeatGlow/display/framebuffer/fbdev"
	"github.com/BeatGlow/display/framebuffer/kms"
)

func TestHardwareKMS(t *testing.T) {
	screen := openHardwareDisplay(t, "/dev/dri/card0", func(path string) (display.Display, error) {
		return kms.Open(&kms.Config{Device: path})
	}, func(err error) bool {
		return errors.Is(err, kms.ErrNotSupported) || errors.Is(err, kms.ErrNoConnector) ||
			errors.Is(err, fs.ErrPermission) || errors.Is(err, syscall.EBUSY) ||
			errors.Is(err, syscall.ENODEV) || errors.Is(err, syscall.ENOTTY) ||
			strings.Contains(err.Error(), "connected outputs; select one")
	})
	testDisplayFunctions(t, screen)
}

func TestHardwareFBDev(t *testing.T) {
	screen := openHardwareDisplay(t, "/dev/fb0", func(path string) (display.Display, error) {
		return fbdev.Open(path)
	}, func(err error) bool {
		return errors.Is(err, fbdev.ErrNotSupported) || errors.Is(err, fs.ErrPermission) ||
			errors.Is(err, syscall.ENODEV) || errors.Is(err, syscall.ENOTTY)
	})
	testDisplayFunctions(t, screen)
}

func openHardwareDisplay(t *testing.T, path string, open func(string) (display.Display, error), unavailable func(error) bool) display.Display {
	t.Helper()
	device, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Skipf("hardware device %s is unavailable: %v", path, err)
	}
	if err := device.Close(); err != nil {
		t.Fatalf("close hardware device probe %s: %v", path, err)
	}

	screen, err := open(path)
	if err != nil {
		if unavailable(err) {
			t.Skipf("no usable display at %s: %v", path, err)
		}
		t.Fatalf("open display at %s: %v", path, err)
	}
	return screen
}

func testDisplayFunctions(t *testing.T, screen display.Display) {
	t.Helper()
	closed := false
	t.Cleanup(func() {
		if !closed {
			if err := screen.Close(); err != nil {
				t.Errorf("close display: %v", err)
			}
		}
	})
	bounds := screen.Bounds()
	if bounds.Empty() {
		t.Fatalf("display returned empty bounds %v", bounds)
	}
	if screen.ColorModel() == nil {
		t.Fatal("display returned a nil color model")
	}
	if screen.String() == "" {
		t.Fatal("display returned an empty description")
	}

	original := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(original, original.Bounds(), screen, bounds.Min, draw.Src)
	t.Cleanup(func() {
		_ = screen.Show(true)
		draw.Draw(screen, bounds, original, original.Bounds().Min, draw.Src)
		if err := screen.Refresh(); err != nil {
			t.Errorf("restore display contents: %v", err)
		}
		err := screen.Close()
		closed = true
		if err != nil {
			t.Errorf("close display: %v", err)
		}
	})

	if err := screen.SetContrast(128); err != nil && !errors.Is(err, kms.ErrNotSupported) {
		t.Errorf("SetContrast: %v", err)
	}
	if err := screen.SetRotation(display.Rotate90); err != nil && !errors.Is(err, kms.ErrNotSupported) {
		t.Errorf("SetRotation(90): %v", err)
	}
	if err := screen.SetRotation(display.NoRotation); err != nil {
		t.Errorf("SetRotation(0): %v", err)
	}
	if err := screen.Show(false); err != nil {
		t.Errorf("Show(false): %v", err)
	}
	if err := screen.Show(true); err != nil {
		t.Errorf("Show(true): %v", err)
	}

	probe := bounds.Min
	setAndCheckPixel(t, screen, probe, color.RGBA{R: 211, G: 47, B: 109, A: 255})
	screen.Fill(color.RGBA{R: 19, G: 113, B: 227, A: 255})
	checkPixel(t, screen, probe, color.RGBA{R: 19, G: 113, B: 227, A: 255})
	screen.Clear()
	checkPixel(t, screen, probe, color.RGBA{A: 255})
	if err := screen.Refresh(); err != nil {
		t.Errorf("Refresh: %v", err)
	}

	if copier, ok := screen.(interface{ CopyFrom(image.Image) error }); ok {
		frame := image.NewRGBA(image.Rect(5, 7, 5+bounds.Dx(), 7+bounds.Dy()))
		frame.Set(5, 7, color.RGBA{R: 83, G: 151, B: 29, A: 255})
		if err := copier.CopyFrom(frame); err != nil {
			t.Errorf("CopyFrom: %v", err)
		} else {
			checkPixel(t, screen, probe, color.RGBA{R: 83, G: 151, B: 29, A: 255})
		}
		if err := copier.CopyFrom(nil); err == nil {
			t.Error("CopyFrom(nil) succeeded; want an error")
		}
	}
	if buffer, ok := screen.(interface{ DrawBuffer() *image.RGBA }); ok {
		buffer.DrawBuffer().Set(probe.X, probe.Y, color.RGBA{R: 37, G: 101, B: 197, A: 255})
		checkPixel(t, screen, probe, color.RGBA{R: 37, G: 101, B: 197, A: 255})
	}
}

func setAndCheckPixel(t *testing.T, screen display.Display, point image.Point, want color.RGBA) {
	t.Helper()
	screen.Set(point.X, point.Y, want)
	checkPixel(t, screen, point, want)
}

func checkPixel(t *testing.T, screen display.Display, point image.Point, want color.RGBA) {
	t.Helper()
	want = color.RGBAModel.Convert(screen.ColorModel().Convert(want)).(color.RGBA)
	got := color.RGBAModel.Convert(screen.At(point.X, point.Y)).(color.RGBA)
	if got != want {
		t.Errorf("pixel at %v = %#v, want %#v", point, got, want)
	}
}
