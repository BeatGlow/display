package streamdeck

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"

	"github.com/BeatGlow/display/streamdeck/internal/jpegn"
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
	return writeJPEGImage(keyImageCommand, byte(key), img, true, send)
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

func writeJPEGImage(command, target byte, img image.Image, rotate bool, send func([]byte) error) error {
	writer := jpegReportWriter{command: command, target: target, send: send}
	opts := jpegn.Options{OptimizeCoding: true, Rotate180: rotate}
	if err := jpegn.Encode(writer, img, opts); err != nil {
		return fmt.Errorf("streamdeck: upload JPEG: %w", err)
	}
	return nil
}

// jpegReportWriter consumes one complete JPEG from the encoder's reusable buffer.
type jpegReportWriter struct {
	command, target byte
	send            func([]byte) error
}

func (w jpegReportWriter) Write(data []byte) (int, error) {
	const chunkSize = modernOutputSize - modernImageHeaderSize
	var report [modernOutputSize]byte
	report[0], report[1], report[2] = 0x02, w.command, w.target

	written := 0
	for page := 0; written < len(data); page++ {
		size := min(len(data)-written, chunkSize)
		end := modernImageHeaderSize + size
		if written+size == len(data) {
			report[3] = 1
			clear(report[end:])
		}
		binary.LittleEndian.PutUint16(report[4:6], uint16(size))
		binary.LittleEndian.PutUint16(report[6:8], uint16(page))
		copy(report[modernImageHeaderSize:end], data[written:written+size])
		if err := w.send(report[:]); err != nil {
			return written, fmt.Errorf("write image report %d: %w", page, err)
		}
		written += size
	}
	return written, nil
}

// rotatedKeyImage rotates a key image 180 degrees without copying its pixels.
type rotatedKeyImage struct{ image.Image }

func (img rotatedKeyImage) At(x, y int) color.Color {
	bounds := img.Bounds()
	return img.Image.At(bounds.Max.X-1-(x-bounds.Min.X), bounds.Max.Y-1-(y-bounds.Min.Y))
}
