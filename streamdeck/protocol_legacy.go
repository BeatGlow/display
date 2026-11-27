package streamdeck

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
)

const legacyImageHeaderSize = 16

// The Original and Mini families report key snapshots without a separate
// command or payload length. Their BMP transfers differ in page size and numbering.
type legacyKeyProtocol struct {
	keys     []byte
	columns  int
	original bool
}

func newMiniProtocol(model Model) protocol {
	return &legacyKeyProtocol{
		keys: make([]byte, model.KeyColumns*model.KeyRows),
	}
}

func newOriginalProtocol(model Model) protocol {
	return &legacyKeyProtocol{
		keys:     make([]byte, model.KeyColumns*model.KeyRows),
		columns:  model.KeyColumns,
		original: true,
	}
}

func (p *legacyKeyProtocol) writeKey(key int, img image.Image, send func([]byte) error) error {
	var nativeImage image.Image = miniKeyImage{img}
	reportSize, page := 1024, 0
	if p.original {
		nativeImage = rotatedKeyImage{img}
		reportSize, page = 8191, 1
	}
	data := encodeKeyBMP(nativeImage)

	chunkSize := reportSize - legacyImageHeaderSize
	if p.original {
		// The 2017 model expects two equal chunks in its padded output reports.
		chunkSize = len(data) / 2
	}
	report := make([]byte, reportSize)
	report[0], report[1], report[5] = 0x02, 0x01, byte(p.wireKey(key)+1)
	for len(data) > 0 {
		size := min(len(data), chunkSize)
		report[2] = byte(page)
		if size == len(data) {
			report[4] = 1
		}
		copy(report[legacyImageHeaderSize:], data[:size])
		clear(report[legacyImageHeaderSize+size:])
		if err := send(report); err != nil {
			return fmt.Errorf("streamdeck: write image report %d: %w", page, err)
		}
		data = data[size:]
		page++
	}
	return nil
}

func (p *legacyKeyProtocol) events(report []byte) []Event {
	if len(report) < 1+len(p.keys) || report[0] != 0x01 {
		return nil
	}
	keys := report[1 : 1+len(p.keys)]
	if !p.original {
		return buttonEvents(p.keys, keys, KeyPress, KeyRelease)
	}

	var reordered [15]byte
	for key := range keys {
		reordered[key] = keys[p.wireKey(key)]
	}
	return buttonEvents(p.keys, reordered[:len(keys)], KeyPress, KeyRelease)
}

func (p *legacyKeyProtocol) wireKey(key int) int {
	if !p.original {
		return key
	}
	column := key % p.columns
	return key - column + p.columns - 1 - column
}

// Mini firmware expects the documented quarter-turn and a scanline-axis
// correction for bottom-up BMPs. Together they transpose the square key image.
type miniKeyImage struct{ image.Image }

func (img miniKeyImage) At(x, y int) color.Color {
	origin := img.Bounds().Min
	return img.Image.At(origin.X+y-origin.Y, origin.Y+x-origin.X)
}

// encodeKeyBMP produces an uncompressed, 24-bit BMP with bottom-up BGR rows.
func encodeKeyBMP(img image.Image) []byte {
	const (
		headerSize     = 54   // BITMAPFILEHEADER plus BITMAPINFOHEADER.
		pixelsPerMeter = 3780 // 96 dpi.
	)
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	stride := (width*3 + 3) &^ 3
	data := make([]byte, headerSize+stride*height)
	copy(data, "BM")
	binary.LittleEndian.PutUint32(data[2:6], uint32(len(data)))
	binary.LittleEndian.PutUint32(data[10:14], headerSize)
	binary.LittleEndian.PutUint32(data[14:18], 40)
	binary.LittleEndian.PutUint32(data[18:22], uint32(width))
	binary.LittleEndian.PutUint32(data[22:26], uint32(height))
	binary.LittleEndian.PutUint16(data[26:28], 1)
	binary.LittleEndian.PutUint16(data[28:30], 24)
	binary.LittleEndian.PutUint32(data[34:38], uint32(stride*height))
	binary.LittleEndian.PutUint32(data[38:42], pixelsPerMeter)
	binary.LittleEndian.PutUint32(data[42:46], pixelsPerMeter)

	for y := range height {
		row := data[headerSize+y*stride:]
		for x := range width {
			r, g, b, _ := img.At(bounds.Min.X+x, bounds.Max.Y-1-y).RGBA()
			row[3*x], row[3*x+1], row[3*x+2] = byte(b>>8), byte(g>>8), byte(r>>8)
		}
	}
	return data
}
