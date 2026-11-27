package streamdeck

import (
	"encoding/binary"
	"image"
)

const (
	plusProductID   = 0x0084
	plusKeyCols     = 4
	plusKeyRows     = 2
	plusKeySize     = 120
	plusDialCount   = 4
	plusStripWidth  = 800
	plusStripHeight = 100
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
	return writeJPEGImage(keyImageCommand, byte(key), img, send)
}

func (p *plusProtocol) writeStrip(img image.Image, send func([]byte) error) error {
	return writeJPEGImage(0x0b, 0, img, send)
}

func (p *plusProtocol) events(report []byte) []Event {
	command, payload := modernInput(report)
	switch command {
	case 0x00:
		if len(payload) == len(p.keys) {
			return buttonEvents(p.keys[:], payload, KeyPress, KeyRelease)
		}
	case 0x02:
		return touchEvents(payload)
	case 0x03:
		if len(payload) == len(p.dials)+1 {
			return p.dialEvents(payload)
		}
	}
	return nil
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
