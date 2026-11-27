// Package streamdeck controls Elgato Stream Deck devices over USB HID.
// It supports the Stream Deck + on Linux and macOS.
package streamdeck

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"slices"
	"strings"
	"sync"
)

var (
	// ErrNotFound indicates that no supported Stream Deck matched the selection.
	ErrNotFound = errors.New("streamdeck: no matching device found")
	// ErrMultipleDevices indicates that automatic selection requires a device path.
	ErrMultipleDevices = errors.New("streamdeck: multiple devices found")
	// ErrUnsupported indicates that the platform or device lacks an operation.
	ErrUnsupported = errors.New("streamdeck: unsupported operation")
)

const elgatoVendorID = 0x0fd9

// Model describes the keys, dials, and touch strip of a supported Stream Deck.
type Model struct {
	Name                string
	KeyColumns, KeyRows int
	KeySize             image.Point
	DialCount           int
	StripSize           image.Point
}

// DeviceInfo identifies an attached Stream Deck.
type DeviceInfo struct {
	// Path is a platform-specific HID identifier accepted by Open.
	Path  string
	Model Model
}

type modelSpec struct {
	model       Model
	newProtocol func() protocol
}

func lookupModel(productID uint16) (modelSpec, bool) {
	switch productID {
	case plusProductID:
		return modelSpec{model: plusModel, newProtocol: func() protocol { return &plusProtocol{} }}, true
	default:
		return modelSpec{}, false
	}
}

type hidIdentity struct {
	path      string
	productID uint16
}

// A transport exchanges complete HID reports, including their report ID.
// Closing it must interrupt a pending read.
type transport interface {
	readReport(context.Context, []byte) (int, error)
	writeReport([]byte) error
	Close() error
}

// Each open device gets a protocol instance to track input state independently.
type protocol interface {
	events([]byte) []Event
	writeKey(int, image.Image, func([]byte) error) error
	writeStrip(image.Image, func([]byte) error) error
}

// Device represents an open Stream Deck. Its methods may be called concurrently.
type Device struct {
	info     DeviceInfo
	hid      transport
	protocol protocol

	readGate  chan struct{} // Serializes reads and guards protocol input state and pending events.
	pending   []Event
	writeMu   sync.Mutex // Keeps the reports of each image together.
	closeOnce sync.Once
	closeErr  error
	done      chan struct{}
}

// Devices lists the attached Stream Decks supported by this package.
func Devices() ([]DeviceInfo, error) {
	identities, err := enumerateHID()
	if err != nil {
		return nil, fmt.Errorf("streamdeck: discover devices: %w", err)
	}
	var devices []DeviceInfo
	for _, identity := range identities {
		if spec, ok := lookupModel(identity.productID); ok {
			devices = append(devices, DeviceInfo{Path: identity.path, Model: spec.model})
		}
	}
	slices.SortFunc(devices, func(a, b DeviceInfo) int { return strings.Compare(a.Path, b.Path) })
	return devices, nil
}

// Open opens the Stream Deck identified by path. An empty path selects the only
// attached, supported Stream Deck. On Linux, path may be a symlink to a hidraw node.
func Open(path string) (*Device, error) {
	identities, err := enumerateHID()
	if err != nil {
		return nil, fmt.Errorf("streamdeck: discover devices: %w", err)
	}
	if path != "" {
		path, err = resolvePath(path)
		if err != nil {
			return nil, fmt.Errorf("streamdeck: resolve device path: %w", err)
		}
	}
	selected, err := selectDevice(identities, path)
	if err != nil {
		return nil, err
	}
	hid, err := openHID(selected.path, selected.productID)
	if err != nil {
		return nil, fmt.Errorf("streamdeck: open %s: %w", selected.path, err)
	}
	spec, _ := lookupModel(selected.productID)
	info := DeviceInfo{Path: selected.path, Model: spec.model}
	return newDevice(info, hid, spec.newProtocol()), nil
}

func selectDevice(identities []hidIdentity, path string) (hidIdentity, error) {
	var selected hidIdentity
	for _, identity := range identities {
		if _, supported := lookupModel(identity.productID); !supported {
			continue
		}
		if path != "" && identity.path != path {
			continue
		}
		if selected.path != "" {
			return hidIdentity{}, ErrMultipleDevices
		}
		selected = identity
	}
	if selected.path == "" {
		return hidIdentity{}, ErrNotFound
	}
	return selected, nil
}

func newDevice(info DeviceInfo, hid transport, protocol protocol) *Device {
	return &Device{
		info:     info,
		hid:      hid,
		protocol: protocol,
		readGate: make(chan struct{}, 1),
		done:     make(chan struct{}),
	}
}

// Info describes the opened device.
func (d *Device) Info() DeviceInfo {
	return d.info
}

// SetKeyImage sends an image to a zero-based key, ordered left to right and top to bottom.
// The image must match the model's key size.
func (d *Device) SetKeyImage(key int, img image.Image) error {
	d.writeMu.Lock()
	defer d.writeMu.Unlock()
	if d.closed() {
		return os.ErrClosed
	}
	return d.protocol.writeKey(key, img, d.writeReport)
}

// SetStripImage sends an image matching the model's touch strip size.
func (d *Device) SetStripImage(img image.Image) error {
	d.writeMu.Lock()
	defer d.writeMu.Unlock()
	if d.closed() {
		return os.ErrClosed
	}
	return d.protocol.writeStrip(img, d.writeReport)
}

// ReadEvent waits for the next input event or for ctx or the device to close.
// Callers can resume reading after cancellation. Unrecognized and malformed
// reports are ignored.
func (d *Device) ReadEvent(ctx context.Context) (Event, error) {
	select {
	case <-ctx.Done():
		return Event{}, ctx.Err()
	case <-d.done:
		return Event{}, os.ErrClosed
	case d.readGate <- struct{}{}:
		defer func() { <-d.readGate }()
	}

	var report [512]byte
	for {
		if err := ctx.Err(); err != nil {
			return Event{}, err
		}
		if d.closed() {
			return Event{}, os.ErrClosed
		}
		if len(d.pending) > 0 {
			event := d.pending[0]
			d.pending = d.pending[1:]
			return event, nil
		}
		n, err := d.hid.readReport(ctx, report[:])
		if err != nil {
			if d.closed() {
				return Event{}, os.ErrClosed
			}
			return Event{}, fmt.Errorf("streamdeck: read input report: %w", err)
		}
		d.pending = d.protocol.events(report[:n])
	}
}

// Close releases the device and interrupts pending input reads. It is idempotent.
func (d *Device) Close() error {
	d.closeOnce.Do(func() {
		close(d.done)
		d.closeErr = d.hid.Close()
	})
	return d.closeErr
}

func (d *Device) closed() bool {
	select {
	case <-d.done:
		return true
	default:
		return false
	}
}

func (d *Device) writeReport(report []byte) error {
	if d.closed() {
		return os.ErrClosed
	}
	err := d.hid.writeReport(report)
	if err != nil && d.closed() {
		return os.ErrClosed
	}
	return err
}
