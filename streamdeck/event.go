package streamdeck

import "image"

// EventKind identifies the control or gesture that generated an Event.
type EventKind uint8

const (
	_ EventKind = iota
	KeyPress
	KeyRelease
	DialPress
	DialRelease
	DialRotate
	TouchTap
	TouchLongPress
	TouchSwipe
)

// Event describes a hardware input on the Stream Deck +.
type Event struct {
	Kind EventKind
	// Index is the zero-based key or dial number for key and dial events.
	Index int
	// Delta is the number of dial ticks (positive clockwise) for DialRotate.
	Delta int
	// Position is the touch location or swipe start in strip coordinates.
	Position image.Point
	// End is the swipe end in strip coordinates.
	End image.Point
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
