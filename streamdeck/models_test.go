package streamdeck

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
	"time"

	"golang.org/x/image/bmp"
)

type keyModelCase struct {
	name                  string
	product               uint16
	columns, rows, size   int
	key, wireKey          int
	reportSize, chunkSize int
	firstPage             int
	bitmap                bool
	wireCorners           [4]int
}

var keyModelCases = []keyModelCase{
	{
		name: "Stream Deck Original", product: 0x0060, columns: 5, rows: 3, size: 72,
		key: 5, wireKey: 9, reportSize: 8191, chunkSize: 7803, firstPage: 1, bitmap: true,
		wireCorners: [4]int{3, 2, 1, 0},
	},
	{
		name: "Stream Deck Mini", product: 0x0063, columns: 3, rows: 2, size: 80,
		key: 3, wireKey: 3, reportSize: 1024, chunkSize: 1008, bitmap: true,
		wireCorners: [4]int{0, 2, 1, 3},
	},
	{
		name: "Stream Deck XL", product: 0x006c, columns: 8, rows: 4, size: 96,
		key: 31, wireKey: 31, reportSize: 1024, chunkSize: 1016,
		wireCorners: [4]int{3, 2, 1, 0},
	},
	{
		name: "Stream Deck 2019", product: 0x006d, columns: 5, rows: 3, size: 72,
		key: 14, wireKey: 14, reportSize: 1024, chunkSize: 1016,
		wireCorners: [4]int{3, 2, 1, 0},
	},
	{
		name: "Stream Deck MK.2", product: 0x0080, columns: 5, rows: 3, size: 72,
		key: 14, wireKey: 14, reportSize: 1024, chunkSize: 1016,
		wireCorners: [4]int{3, 2, 1, 0},
	},
	{
		name: "Stream Deck Mini 2022", product: 0x0090, columns: 3, rows: 2, size: 80,
		key: 3, wireKey: 3, reportSize: 1024, chunkSize: 1008, bitmap: true,
		wireCorners: [4]int{0, 2, 1, 3},
	},
}

func TestKeyModelsReportPhysicalKeysInLogicalOrder(t *testing.T) {
	for _, tc := range keyModelCases {
		t.Run(tc.name, func(t *testing.T) {
			count := tc.columns * tc.rows
			header := []byte{0x01, 0x00, byte(count), 0x00}
			if tc.bitmap {
				header = []byte{0x01}
			}
			pressed := append(bytes.Clone(header), make([]byte, count)...)
			released := bytes.Clone(pressed)
			pressed[len(header)+tc.wireKey] = 1
			d, _ := testDeckFor(t, tc.product, [][]byte{
				pressed[:len(pressed)-1], pressed, pressed, released,
			})

			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			for _, expected := range []Event{
				{Kind: KeyPress, Index: tc.key},
				{Kind: KeyRelease, Index: tc.key},
			} {
				event, err := d.ReadEvent(ctx)
				if err != nil || event != expected {
					t.Fatalf("ReadEvent = %+v, %v, want %+v", event, err, expected)
				}
			}
		})
	}
}

func TestKeyModelsUploadImagesInNativeFormatAndOrientation(t *testing.T) {
	palette := [...]color.RGBA{
		{R: 255, A: 255},
		{G: 255, A: 255},
		{B: 255, A: 255},
		{R: 255, G: 255, A: 255},
	}
	for _, tc := range keyModelCases {
		t.Run(tc.name, func(t *testing.T) {
			d, wire := testDeckFor(t, tc.product, nil)
			wantModel := Model{
				Name: tc.name, KeyColumns: tc.columns, KeyRows: tc.rows,
				KeySize: image.Pt(tc.size, tc.size),
			}
			if got := d.Info().Model; got != wantModel {
				t.Fatalf("model = %+v, want %+v", got, wantModel)
			}
			img := image.NewRGBA(image.Rect(5, 9, 5+tc.size, 9+tc.size))
			for y := range tc.size {
				for x := range tc.size {
					corner := 2*(y/(tc.size/2)) + x/(tc.size/2)
					img.SetRGBA(5+x, 9+y, palette[corner])
				}
			}

			if err := d.SetStripImage(img); !errors.Is(err, ErrUnsupported) {
				t.Fatalf("SetStripImage = %v, want ErrUnsupported", err)
			}
			if len(wire.output) != 0 {
				t.Fatal("unsupported strip request sent reports")
			}
			if err := d.SetKeyImage(tc.key, img); err != nil {
				t.Fatal(err)
			}
			decoded := decodeKeyTransfer(t, tc, wire.output)
			if got := decoded.Bounds().Size(); got != wantModel.KeySize {
				t.Fatalf("decoded size = %v, want %v", got, wantModel.KeySize)
			}
			for outputCorner, inputCorner := range tc.wireCorners {
				x := (1 + 2*(outputCorner%2)) * tc.size / 4
				y := (1 + 2*(outputCorner/2)) * tc.size / 4
				got := color.RGBAModel.Convert(decoded.At(x, y)).(color.RGBA)
				want := palette[inputCorner]
				if !nearColor(got, want) {
					t.Fatalf("decoded corner (%d, %d) = %v, want %v", x, y, got, want)
				}
			}
		})
	}
}

func decodeKeyTransfer(t *testing.T, tc keyModelCase, reports [][]byte) image.Image {
	t.Helper()
	if len(reports) == 0 {
		t.Fatal("image upload sent no reports")
	}
	var encoded bytes.Buffer
	var bmpRemaining int
	for i, report := range reports {
		if len(report) != tc.reportSize {
			t.Fatalf("report %d = %d bytes, want %d", i, len(report), tc.reportSize)
		}
		var last byte
		if i == len(reports)-1 {
			last = 1
		}
		offset := 8
		size := int(binary.LittleEndian.Uint16(report[4:6]))
		if tc.bitmap {
			offset = 16
			if i == 0 {
				bmpRemaining = int(binary.LittleEndian.Uint32(report[offset+2 : offset+6]))
			}
			size = min(bmpRemaining, tc.chunkSize)
			bmpRemaining -= size
			want := make([]byte, offset)
			copy(want, []byte{2, 1, byte(tc.firstPage + i), 0, last, byte(tc.wireKey + 1)})
			if !bytes.Equal(report[:offset], want) {
				t.Fatalf("legacy report %d header = % x, want % x", i, report[:offset], want)
			}
		} else {
			want := []byte{2, 7, byte(tc.wireKey), last}
			if !bytes.Equal(report[:4], want) || int(binary.LittleEndian.Uint16(report[6:8])) != i {
				t.Fatalf("JPEG report %d has incorrect header % x", i, report[:offset])
			}
		}
		if size < 1 || size > tc.chunkSize || offset+size > len(report) || (last == 0 && size != tc.chunkSize) {
			t.Fatalf("report %d has invalid chunk size %d", i, size)
		}
		for _, padding := range report[offset+size:] {
			if padding != 0 {
				t.Fatalf("report %d has nonzero padding", i)
			}
		}
		encoded.Write(report[offset : offset+size])
	}

	decode := jpeg.Decode
	if tc.bitmap {
		if bmpRemaining != 0 {
			t.Fatalf("BMP upload is missing %d bytes", bmpRemaining)
		}
		decode = bmp.Decode
	}
	decoded, err := decode(&encoded)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func nearColor(got, want color.RGBA) bool {
	return nearChannel(got.R, want.R) && nearChannel(got.G, want.G) && nearChannel(got.B, want.B)
}

func nearChannel(got, want uint8) bool {
	diff := int(got) - int(want)
	return -12 <= diff && diff <= 12
}
