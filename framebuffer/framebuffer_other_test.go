//go:build !linux && !freebsd

package framebuffer

import (
	"errors"
	"testing"

	"github.com/BeatGlow/display/framebuffer/fbdev"
	"github.com/BeatGlow/display/framebuffer/kms"
)

func TestOpenReportsUnsupportedBackends(t *testing.T) {
	if display, err := Open(nil); display != nil || !errors.Is(err, kms.ErrNotSupported) || !errors.Is(err, fbdev.ErrNotSupported) {
		t.Fatalf("Open(nil) = (%v, %v), want both backend unsupported errors", display, err)
	}
}
