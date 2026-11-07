package framebuffer

import (
	"errors"
	"image"
	"image/color"
	"testing"

	"github.com/BeatGlow/display"
	"github.com/BeatGlow/display/framebuffer/kms"
)

type testDisplay struct{}

func (testDisplay) ColorModel() color.Model            { return color.RGBAModel }
func (testDisplay) Bounds() image.Rectangle            { return image.Rect(0, 0, 1, 1) }
func (testDisplay) At(int, int) color.Color            { return color.RGBA{} }
func (testDisplay) Set(int, int, color.Color)          {}
func (testDisplay) String() string                     { return "test display" }
func (testDisplay) Close() error                       { return nil }
func (testDisplay) Clear()                             {}
func (testDisplay) Fill(color.Color)                   {}
func (testDisplay) Show(bool) error                    { return nil }
func (testDisplay) SetContrast(uint8) error            { return nil }
func (testDisplay) SetRotation(display.Rotation) error { return nil }
func (testDisplay) Refresh() error                     { return nil }

func TestOpenPrefersKMS(t *testing.T) {
	want := testDisplay{}
	fbdevCalled := false
	got, err := openWith(&Config{}, func(*kms.Config) (display.Display, error) {
		return want, nil
	}, func(string) (display.Display, error) {
		fbdevCalled = true
		return nil, errors.New("unexpected fbdev attempt")
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != want || fbdevCalled {
		t.Fatalf("got display %v, fbdev called %v; want KMS display and no fallback", got, fbdevCalled)
	}
}

func TestOpenFallsBackToFBDev(t *testing.T) {
	want := testDisplay{}
	wantKMSConfig := &kms.Config{Connector: "HDMI-A-1"}
	config := &Config{KMS: wantKMSConfig, FBDev: "/dev/fb2"}
	gotKMS, gotFBDev := false, false
	got, err := openWith(config, func(got *kms.Config) (display.Display, error) {
		gotKMS = got == wantKMSConfig
		return nil, errors.New("KMS unavailable")
	}, func(path string) (display.Display, error) {
		gotFBDev = path == "/dev/fb2"
		return want, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != want || !gotKMS || !gotFBDev {
		t.Fatalf("got display %v, KMS config passed %v, fbdev path passed %v", got, gotKMS, gotFBDev)
	}
}

func TestOpenJoinsBackendErrors(t *testing.T) {
	kmsErr := errors.New("KMS unavailable")
	fbdevErr := errors.New("fbdev unavailable")
	_, err := openWith(&Config{}, func(*kms.Config) (display.Display, error) {
		return nil, kmsErr
	}, func(string) (display.Display, error) {
		return nil, fbdevErr
	})
	if !errors.Is(err, kmsErr) || !errors.Is(err, fbdevErr) {
		t.Fatalf("error %v does not preserve both backend causes", err)
	}
}
