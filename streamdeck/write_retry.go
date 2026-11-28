package streamdeck

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const (
	hidTransferAttempts = 3
	firstHIDRetryDelay  = 10 * time.Millisecond
)

// hidTransferError distinguishes a transport failure from a report or image error.
type hidTransferError struct{ cause error }

func (e *hidTransferError) Error() string { return e.cause.Error() }
func (e *hidTransferError) Unwrap() error { return e.cause }

// writeImage runs with writeMu held. Every attempt starts a fresh image
// transfer because a failed write can leave a partial image on the device.
func (d *Device) writeImage(upload func() error) error {
	return d.retryHID("image upload", upload)
}

func (d *Device) retryHID(operation string, transfer func() error) error {
	delay := firstHIDRetryDelay
	for attempt := 1; ; attempt++ {
		err := transfer()
		if err == nil || errors.Is(err, os.ErrClosed) || errors.Is(err, os.ErrPermission) || errors.Is(err, ErrUnsupported) {
			return err
		}
		if _, ok := errors.AsType[*hidTransferError](err); !ok {
			return err
		}
		if attempt == hidTransferAttempts {
			return fmt.Errorf("streamdeck: %s failed after %d attempts: %w", operation, attempt, err)
		}

		timer := time.NewTimer(delay)
		select {
		case <-d.done:
			timer.Stop()
			return os.ErrClosed
		case <-timer.C:
		}
		delay *= 2
	}
}
