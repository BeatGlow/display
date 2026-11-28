package streamdeck

import (
	"bytes"
	"os"
)

type serialReport struct {
	id             byte
	size, offset   int
	lengthPrefixed bool
}

var (
	shortLegacySerial = serialReport{id: 0x03, size: 17, offset: 5}
	longLegacySerial  = serialReport{id: 0x03, size: 32, offset: 5}
	modernSerial      = serialReport{id: 0x06, size: 32, offset: 2, lengthPrefixed: true}
)

// SerialNumber queries the unit serial from the opened Stream Deck. The serial
// follows the unit across USB ports. Check for duplicates before using it as a
// persistent profile key.
//
// It retries transient HID failures up to twice and returns an empty string
// if the serial is unavailable or the query fails.
func (d *Device) SerialNumber() string {
	d.writeMu.Lock()
	defer d.writeMu.Unlock()
	var serial string
	if err := d.retryHID("unit serial query", func() error {
		var err error
		serial, err = d.readSerialNumber()
		return err
	}); err != nil {
		return ""
	}
	return serial
}

func (d *Device) readSerialNumber() (string, error) {
	d.serialMu.Lock()
	defer d.serialMu.Unlock()
	if d.closed() {
		return "", os.ErrClosed
	}

	configuration := d.serialReport
	var buffer [32]byte
	report := buffer[:configuration.size]
	report[0] = configuration.id
	n, err := d.hid.getFeatureReport(report)
	if d.closed() {
		return "", os.ErrClosed
	}
	if err != nil {
		return "", &hidTransferError{cause: err}
	}
	if n < configuration.offset || n > len(report) || report[0] != configuration.id {
		return "", nil
	}
	end := n
	if configuration.lengthPrefixed {
		end = configuration.offset + int(report[1])
		if end > n {
			return "", nil
		}
	}
	value := bytes.TrimSpace(bytes.TrimRight(report[configuration.offset:end], "\x00"))
	for _, b := range value {
		if b < 0x20 || b > 0x7e {
			return "", nil
		}
	}
	return string(value), nil
}
