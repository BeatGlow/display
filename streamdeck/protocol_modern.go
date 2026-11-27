package streamdeck

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
)

const (
	modernOutputSize      = 1024
	modernImageHeaderSize = 8
	keyImageCommand       = 0x07
)

// The Classic (2019 and MK.2) and XL use the same key commands and rotation.
type modernKeyProtocol struct {
	keys []byte
}

func newModernKeyProtocol(model Model) protocol {
	return &modernKeyProtocol{keys: make([]byte, model.KeyColumns*model.KeyRows)}
}

func (p *modernKeyProtocol) writeKey(key int, img image.Image, send func([]byte) error) error {
	return writeJPEGImage(keyImageCommand, byte(key), rotatedKeyImage{img}, send)
}

func (p *modernKeyProtocol) events(report []byte) []Event {
	command, payload := modernInput(report)
	if command != 0x00 || len(payload) != len(p.keys) {
		return nil
	}
	return buttonEvents(p.keys, payload, KeyPress, KeyRelease)
}

// modernInput extracts the command and declared payload, excluding HID padding.
func modernInput(report []byte) (command byte, payload []byte) {
	if len(report) < 4 || report[0] != 0x01 {
		return 0, nil
	}
	size := int(binary.LittleEndian.Uint16(report[2:4]))
	if size > len(report)-4 {
		return 0, nil
	}
	return report[1], report[4 : 4+size]
}

func writeJPEGImage(command, target byte, img image.Image, send func([]byte) error) error {
	var jpegData bytes.Buffer
	if err := jpeg.Encode(&jpegData, img, nil); err != nil {
		return fmt.Errorf("streamdeck: encode JPEG: %w", err)
	}

	const chunkSize = modernOutputSize - modernImageHeaderSize
	for page, remaining := 0, jpegData.Bytes(); len(remaining) > 0; page++ {
		size := min(len(remaining), chunkSize)
		var report [modernOutputSize]byte
		report[0], report[1], report[2] = 0x02, command, target
		if size == len(remaining) {
			report[3] = 1
		}
		binary.LittleEndian.PutUint16(report[4:6], uint16(size))
		binary.LittleEndian.PutUint16(report[6:8], uint16(page))
		copy(report[modernImageHeaderSize:], remaining[:size])
		if err := send(report[:]); err != nil {
			return fmt.Errorf("streamdeck: write image report %d: %w", page, err)
		}
		remaining = remaining[size:]
	}
	return nil
}

// rotatedKeyImage rotates a key image 180 degrees without copying its pixels.
type rotatedKeyImage struct{ image.Image }

func (img rotatedKeyImage) At(x, y int) color.Color {
	bounds := img.Bounds()
	return img.Image.At(bounds.Max.X-1-(x-bounds.Min.X), bounds.Max.Y-1-(y-bounds.Min.Y))
}
