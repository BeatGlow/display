//go:build !ios

package streamdeck

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/telesma-app/hid"
)

type darwinHID struct {
	*hid.Device
}

func enumerateHID() ([]hidIdentity, error) {
	var identities []hidIdentity
	var enumerationErr error
	for info, err := range hid.Enumerate(hid.WithVendorID(elgatoVendorID)) {
		if info != nil && info.Path != "" && info.ProductID.Value != nil {
			identities = append(identities, hidIdentity{
				path:      info.Path,
				productID: *info.ProductID.Value,
			})
			continue
		}
		enumerationErr = errors.Join(enumerationErr, err)
	}
	if len(identities) == 0 {
		return nil, enumerationErr
	}
	return identities, nil
}

func resolvePath(path string) (string, error) {
	return path, nil
}

func openHID(path string, _ uint16) (transport, error) {
	device, err := hid.OpenPath(path)
	if err != nil {
		return nil, err
	}
	return &darwinHID{Device: device}, nil
}

func (d *darwinHID) readReport(ctx context.Context, report []byte) (int, error) {
	return d.Read(ctx, report)
}

func (d *darwinHID) writeReport(report []byte) error {
	n, err := d.Write(context.Background(), report)
	if err == nil && n != len(report) {
		return io.ErrShortWrite
	}
	return err
}

func (d *darwinHID) getFeatureReport(report []byte) (int, error) {
	n, err := d.GetFeatureReport(report)
	if err == nil {
		return n, nil
	}
	// The HID library exposes these IOKit status codes only in the error text.
	switch err.Error() {
	case "IOHIDDeviceGetReport failed: 0xe00002c7": // kIOReturnUnsupported
		return 0, ErrUnsupported
	case "IOHIDDeviceGetReport failed: 0xe00002c0", // kIOReturnNoDevice
		"IOHIDDeviceGetReport failed: 0xe00002cd": // kIOReturnNotOpen
		return 0, os.ErrClosed
	}
	return n, err
}
