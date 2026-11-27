package streamdeck

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
)

const (
	plusProductID   = 0x0084
	plusKeyCols     = 4
	plusKeyRows     = 2
	plusKeySize     = 120
	plusDialCount   = 4
	plusStripWidth  = 800
	plusStripHeight = 100

	plusOutputSize      = 1024
	plusImageHeaderSize = 8
)

var plusModel = Model{
	Name:       "Stream Deck +",
	KeyColumns: plusKeyCols,
	KeyRows:    plusKeyRows,
	KeySize:    image.Pt(plusKeySize, plusKeySize),
	DialCount:  plusDialCount,
	StripSize:  image.Pt(plusStripWidth, plusStripHeight),
}

// Stream Deck + report formats are documented at
// https://docs.elgato.com/streamdeck/hid/stream-deck-plus/.
type plusProtocol struct {
	keys  [plusKeyCols * plusKeyRows]byte
	dials [plusDialCount]byte
}

func (p *plusProtocol) writeKey(key int, img image.Image, send func([]byte) error) error {
	if key < 0 || key >= len(p.keys) {
		return fmt.Errorf("streamdeck: key %d outside [0, %d)", key, len(p.keys))
	}
	return writePlusImage(0x07, byte(key), img, image.Pt(plusKeySize, plusKeySize), send)
}

func (p *plusProtocol) writeStrip(img image.Image, send func([]byte) error) error {
	return writePlusImage(0x0b, 0, img, image.Pt(plusStripWidth, plusStripHeight), send)
}

func writePlusImage(command, target byte, img image.Image, dimensions image.Point, send func([]byte) error) error {
	if img == nil || img.Bounds().Size() != dimensions {
		return fmt.Errorf("streamdeck: expected a %d×%d image", dimensions.X, dimensions.Y)
	}
	var jpegData bytes.Buffer
	if err := jpeg.Encode(&jpegData, img, nil); err != nil {
		return fmt.Errorf("streamdeck: encode JPEG: %w", err)
	}

	const chunkSize = plusOutputSize - plusImageHeaderSize
	for page, remaining := 0, jpegData.Bytes(); len(remaining) > 0; page++ {
		size := min(len(remaining), chunkSize)
		var report [plusOutputSize]byte
		report[0], report[1], report[2] = 0x02, command, target
		if size == len(remaining) {
			report[3] = 1
		}
		binary.LittleEndian.PutUint16(report[4:6], uint16(size))
		binary.LittleEndian.PutUint16(report[6:8], uint16(page))
		copy(report[plusImageHeaderSize:], remaining[:size])
		if err := send(report[:]); err != nil {
			return fmt.Errorf("streamdeck: write image report %d: %w", page, err)
		}
		remaining = remaining[size:]
	}
	return nil
}

func (p *plusProtocol) events(report []byte) []Event {
	if len(report) < 4 || report[0] != 0x01 {
		return nil
	}
	size := int(binary.LittleEndian.Uint16(report[2:4]))
	if size > len(report)-4 {
		return nil
	}
	payload := report[4 : 4+size]
	switch report[1] {
	case 0x00:
		if size == len(p.keys) {
			return buttonEvents(p.keys[:], payload, KeyPress, KeyRelease)
		}
	case 0x02:
		return touchEvents(payload)
	case 0x03:
		if size == len(p.dials)+1 {
			return p.dialEvents(payload)
		}
	}
	return nil
}

func buttonEvents(previous, current []byte, press, release EventKind) []Event {
	for _, state := range current {
		if state > 1 {
			return nil
		}
	}

	var events []Event
	for i, state := range current {
		if state == previous[i] {
			continue
		}
		kind := release
		if state == 1 {
			kind = press
		}
		events = append(events, Event{Kind: kind, Index: i})
		previous[i] = state
	}
	return events
}

func (p *plusProtocol) dialEvents(payload []byte) []Event {
	switch payload[0] {
	case 0x00:
		return buttonEvents(p.dials[:], payload[1:], DialPress, DialRelease)
	case 0x01:
		var events []Event
		for i, rotation := range payload[1:] {
			if delta := int(int8(rotation)); delta != 0 {
				events = append(events, Event{Kind: DialRotate, Index: i, Delta: delta})
			}
		}
		return events
	default:
		return nil
	}
}

func touchEvents(payload []byte) []Event {
	// Some firmware uses the 14-byte payload for taps and long presses too.
	if len(payload) != 10 && len(payload) != 14 {
		return nil
	}
	event := Event{Position: touchPoint(payload[2:6])}
	switch payload[0] {
	case 0x01:
		event.Kind = TouchTap
	case 0x02:
		event.Kind = TouchLongPress
	case 0x03:
		if len(payload) != 14 {
			return nil
		}
		event.Kind = TouchSwipe
		event.End = touchPoint(payload[6:10])
	default:
		return nil
	}
	return []Event{event}
}

func touchPoint(data []byte) image.Point {
	return image.Pt(
		int(binary.LittleEndian.Uint16(data[:2])),
		int(binary.LittleEndian.Uint16(data[2:4])),
	)
}
