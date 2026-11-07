//go:build !linux && !freebsd

package kms

import (
	"errors"
	"testing"
)

func TestOpenUnsupportedPlatform(t *testing.T) {
	if display, err := Open(&Config{}); display != nil || !errors.Is(err, ErrNotSupported) {
		t.Fatalf("Open() = (%v, %v), want (nil, ErrNotSupported)", display, err)
	}
}
