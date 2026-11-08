//go:build !linux && !freebsd

package kms

import "github.com/BeatGlow/display"

// Open reports that DRM/KMS is unavailable on this platform.
func Open(*Config) (display.Display, error) { return nil, ErrNotSupported }
