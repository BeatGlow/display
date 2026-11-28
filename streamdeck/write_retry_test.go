package streamdeck

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"os"
	"slices"
	"testing"
	"testing/synctest"
)

func TestImageUploadRestartsAfterHIDWriteFailure(t *testing.T) {
	cases := []struct {
		name      string
		productID uint16
		size      image.Point
		send      func(*Device, image.Image) error
	}{
		{"Plus key", plusProductID, image.Pt(120, 120), func(d *Device, img image.Image) error {
			return d.SetKeyImage(0, img)
		}},
		{"Plus strip", plusProductID, image.Pt(800, 100), (*Device).SetStripImage},
		{"Original key", 0x0060, image.Pt(72, 72), func(d *Device, img image.Image) error {
			return d.SetKeyImage(0, img)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			img := multiReportRetryImage(tc.size)
			healthy, original := testDeckFor(t, tc.productID, nil)
			if err := tc.send(healthy, img); err != nil {
				t.Fatal(err)
			}
			if len(original.output) < 2 {
				t.Fatal("fixture must span more than one report")
			}

			d, wire := testDeckFor(t, tc.productID, nil)
			wire.writeFailure = func([]byte) error {
				if len(wire.output) == 2 {
					return errors.New("temporary HID failure after the first report")
				}
				return nil
			}
			if err := tc.send(d, img); err != nil {
				t.Fatal(err)
			}

			want := append(slices.Clone(original.output[:2]), original.output...)
			if !slices.EqualFunc(wire.output, want, bytes.Equal) {
				t.Fatalf("got %d reports, want a failed two-report attempt followed by the complete image (%d reports)", len(wire.output), len(want))
			}
		})
	}
}

func TestImageUploadStopsWhenHIDWriteCannotRecover(t *testing.T) {
	permanent := errors.New("persistent HID write error")
	cases := []struct {
		name       string
		failure    error
		wantWrites int
	}{
		{"persistent failure uses the retry limit", permanent, 3},
		{"permission failure returns immediately", os.ErrPermission, 1},
		{"closed transport returns immediately", os.ErrClosed, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, wire := testDeck(t, nil)
			wire.writeFailure = func([]byte) error { return tc.failure }
			img := image.NewRGBA(image.Rect(0, 0, 120, 120))
			if err := d.SetKeyImage(0, img); !errors.Is(err, tc.failure) {
				t.Fatalf("SetKeyImage error = %v, want %v", err, tc.failure)
			}
			if len(wire.output) != tc.wantWrites {
				t.Fatalf("got %d writes, want %d", len(wire.output), tc.wantWrites)
			}
		})
	}
}

func TestCloseInterruptsImageRetry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		d, wire := testDeck(t, nil)
		wire.writeFailure = func([]byte) error { return errors.New("temporary HID failure") }
		result := make(chan error, 1)
		go func() {
			result <- d.SetKeyImage(0, image.NewRGBA(image.Rect(0, 0, 120, 120)))
		}()
		synctest.Wait()
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
		if err := <-result; !errors.Is(err, os.ErrClosed) {
			t.Fatalf("SetKeyImage error = %v, want os.ErrClosed", err)
		}
		if len(wire.output) != 1 {
			t.Fatalf("got %d writes, want the first attempt only", len(wire.output))
		}
	})
}

func multiReportRetryImage(size image.Point) image.Image {
	img := image.NewRGBA(image.Rectangle{Max: size})
	for y := range size.Y {
		for x := range size.X {
			img.SetRGBA(x, y, color.RGBA{R: byte(x * y), G: byte(x*x + y), B: byte(y*y + x), A: 255})
		}
	}
	return img
}
