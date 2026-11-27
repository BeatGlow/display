//go:build !ios

package streamdeck

import (
	"context"
	"errors"
	"io"

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
