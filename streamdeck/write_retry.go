package streamdeck

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const (
	imageWriteAttempts   = 3
	firstImageRetryDelay = 10 * time.Millisecond
)

// reportWriteError distinguishes a transport failure from an encoding error.
type reportWriteError struct{ cause error }

func (e *reportWriteError) Error() string { return e.cause.Error() }
func (e *reportWriteError) Unwrap() error { return e.cause }

// writeImage runs with writeMu held. Every attempt starts a fresh image
// transfer because a failed write can leave a partial image on the device.
func (d *Device) writeImage(upload func() error) error {
	delay := firstImageRetryDelay
	for attempt := 1; ; attempt++ {
		err := upload()
		if err == nil || errors.Is(err, os.ErrClosed) || errors.Is(err, os.ErrPermission) {
			return err
		}
		if _, ok := errors.AsType[*reportWriteError](err); !ok {
			return err
		}
		if attempt == imageWriteAttempts {
			return fmt.Errorf("streamdeck: image upload failed after %d attempts: %w", attempt, err)
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
