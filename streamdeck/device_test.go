package streamdeck

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"testing"
	"time"
)

func TestReadEventDeliversChangesAndGestures(t *testing.T) {
	reports := [][]byte{
		{1, 0, 8, 0, 1},                      // Truncated key snapshot.
		{1, 0, 8, 0, 1, 0, 0, 0, 0, 0, 0, 2}, // Invalid key state.
		{1, 0, 8, 0, 1, 0, 0, 0, 0, 0, 0, 1},
		{1, 0, 8, 0, 1, 0, 0, 0, 0, 0, 0, 1}, // Duplicate snapshot.
		{1, 0, 8, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		{1, 3, 5, 0, 0, 0, 0, 0, 1},
		{1, 3, 5, 0, 1, 2, 0, 0xfd, 0},
		{1, 3, 5, 0, 0, 0, 0, 0, 0},
		{1, 2, 14, 0, 1, 1, 0xa2, 1, 0x30, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{1, 2, 14, 0, 2, 1, 0x8b, 1, 0x3f, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{1, 2, 14, 0, 3, 0, 0x90, 1, 25, 0}, // Truncated swipe.
		{1, 2, 14, 0, 3, 0, 0x90, 1, 25, 0, 0x58, 2, 50, 0, 0, 0, 0, 0},
	}
	d, _ := testDeck(t, reports)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	want := []Event{
		{Kind: KeyPress, Index: 0},
		{Kind: KeyPress, Index: 7},
		{Kind: KeyRelease, Index: 0},
		{Kind: DialPress, Index: 3},
		{Kind: DialRotate, Index: 0, Delta: 2},
		{Kind: DialRotate, Index: 2, Delta: -3},
		{Kind: DialRelease, Index: 3},
		{Kind: TouchTap, Position: image.Pt(418, 48)},
		{Kind: TouchLongPress, Position: image.Pt(395, 63)},
		{Kind: TouchSwipe, Position: image.Pt(400, 25), End: image.Pt(600, 50)},
	}
	for i, expected := range want {
		event, err := d.ReadEvent(ctx)
		if err != nil || event != expected {
			t.Fatalf("event %d = %+v, %v, want %+v", i, event, err, expected)
		}
	}
}

func TestCancelReadThenResumeAndClose(t *testing.T) {
	d, wire := testDeck(t, nil)

	read := func(ctx context.Context) <-chan error {
		result := make(chan error, 1)
		go func() {
			_, err := d.ReadEvent(ctx)
			result <- err
		}()
		select {
		case <-wire.reading:
		case <-time.After(time.Second):
			t.Fatal("reader did not reach the HID transport")
		}
		return result
	}
	expect := func(result <-chan error, want error) {
		t.Helper()
		select {
		case err := <-result:
			if !errors.Is(err, want) {
				t.Fatalf("read error = %v, want %v", err, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("read did not return %v", want)
		}
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := read(ctx)
	cancel()
	expect(result, context.Canceled)

	wire.input <- []byte{1, 0, 8, 0, 0, 0, 0, 0, 1, 0, 0, 0}
	resumeCtx, stop := context.WithTimeout(t.Context(), time.Second)
	defer stop()
	event, err := d.ReadEvent(resumeCtx)
	if err != nil || event != (Event{Kind: KeyPress, Index: 4}) {
		t.Fatalf("read after cancel = %+v, %v", event, err)
	}
	<-wire.reading

	result = read(t.Context())
	for range 2 {
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
	}
	expect(result, os.ErrClosed)
	if err := d.SetKeyImage(0, image.NewRGBA(image.Rect(0, 0, 120, 120))); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("write after close = %v, want os.ErrClosed", err)
	}
}

func TestKeyAndStripImageTransfers(t *testing.T) {
	cases := []struct {
		name            string
		width, height   int
		command, target byte
		send            func(*Device, image.Image) error
	}{
		{
			name: "last key", width: 120, height: 120, command: 0x07, target: 7,
			send: func(d *Device, img image.Image) error { return d.SetKeyImage(7, img) },
		},
		{
			name: "touch strip", width: 800, height: 100, command: 0x0b,
			send: (*Device).SetStripImage,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, wire := testDeck(t, nil)
			img := image.NewRGBA(image.Rect(10, 20, 10+tc.width, 20+tc.height))
			for y := range tc.height {
				for x := range tc.width {
					value := color.RGBA{R: 255, A: 255}
					if x >= tc.width/2 {
						value = color.RGBA{B: 255, A: 255}
					}
					// Make the JPEG span several HID reports.
					if y >= tc.height/2 {
						value = color.RGBA{R: uint8(x * y), G: uint8(x + y*y), A: 255}
					}
					img.SetRGBA(x+10, y+20, value)
				}
			}

			if err := tc.send(d, img); err != nil {
				t.Fatal(err)
			}
			if len(wire.output) < 2 {
				t.Fatalf("got %d reports, want a multi-report transfer", len(wire.output))
			}
			var data bytes.Buffer
			for page, report := range wire.output {
				if len(report) != 1024 || !bytes.Equal(report[:3], []byte{2, tc.command, tc.target}) {
					t.Fatalf("report %d has incorrect length or command: % x", page, report[:min(8, len(report))])
				}
				wantLast := byte(0)
				if page == len(wire.output)-1 {
					wantLast = 1
				}
				if got := binary.LittleEndian.Uint16(report[6:8]); int(got) != page || report[3] != wantLast {
					t.Fatalf("report %d has page %d, final flag %d", page, got, report[3])
				}
				size := int(binary.LittleEndian.Uint16(report[4:6]))
				if size < 1 || size > 1016 || (wantLast == 0 && size != 1016) {
					t.Fatalf("report %d has invalid payload size %d", page, size)
				}
				for _, padding := range report[8+size:] {
					if padding != 0 {
						t.Fatalf("report %d has nonzero padding", page)
					}
				}
				data.Write(report[8 : 8+size])
			}

			decoded, err := jpeg.Decode(&data)
			if err != nil {
				t.Fatal(err)
			}
			if got := decoded.Bounds().Size(); got != img.Bounds().Size() {
				t.Fatalf("decoded size = %v, want %v", got, img.Bounds().Size())
			}
			leftRed, _, leftBlue, _ := decoded.At(tc.width/4, tc.height/4).RGBA()
			rightRed, _, rightBlue, _ := decoded.At(tc.width*3/4, tc.height/4).RGBA()
			if leftRed < 0xf000 || leftBlue > 0x1000 || rightBlue < 0xf000 || rightRed > 0x1000 {
				t.Fatal("image orientation changed")
			}

			count := len(wire.output)
			if err := tc.send(d, image.NewRGBA(image.Rect(0, 0, 1, 1))); err == nil {
				t.Fatal("accepted wrong image dimensions")
			}
			if err := d.SetKeyImage(8, img); err == nil {
				t.Fatal("accepted out-of-range key")
			}
			if len(wire.output) != count {
				t.Fatal("invalid image request sent HID reports")
			}
		})
	}
}

type testTransport struct {
	input   chan []byte
	reading chan struct{}
	closed  chan struct{}
	output  [][]byte
}

func testDeck(t *testing.T, reports [][]byte) (*Device, *testTransport) {
	t.Helper()
	return testDeckFor(t, plusProductID, reports)
}

func testDeckFor(t *testing.T, productID uint16, reports [][]byte) (*Device, *testTransport) {
	t.Helper()
	spec, ok := lookupModel(productID)
	if !ok {
		t.Fatalf("no registered model for 0x%04x", productID)
	}

	transport := &testTransport{
		input:   make(chan []byte, max(len(reports), 1)),
		reading: make(chan struct{}, 1),
		closed:  make(chan struct{}),
	}
	for _, report := range reports {
		transport.input <- report
	}
	d := newDevice(DeviceInfo{Model: spec.model}, transport, spec.newProtocol())
	t.Cleanup(func() { _ = d.Close() })
	return d, transport
}

func (w *testTransport) readReport(ctx context.Context, p []byte) (int, error) {
	select {
	case w.reading <- struct{}{}:
	default:
	}
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-w.closed:
		return 0, os.ErrClosed
	case report := <-w.input:
		return copy(p, report), nil
	}
}

func (w *testTransport) writeReport(report []byte) error {
	w.output = append(w.output, bytes.Clone(report))
	return nil
}

func (w *testTransport) Close() error {
	close(w.closed)
	return nil
}
