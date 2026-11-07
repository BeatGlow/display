//go:build !linux

package fbdev

import (
	"errors"
	"testing"
)

func TestOpenUnsupportedPlatform(t *testing.T) {
	if display, err := Open("/dev/fb0"); display != nil || !errors.Is(err, ErrNotSupported) {
		t.Fatalf("Open() = (%v, %v), want (nil, ErrNotSupported)", display, err)
	}
}
