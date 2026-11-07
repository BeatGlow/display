//go:build !linux && !freebsd

package kms

import "github.com/BeatGlow/display"

func Open(*Config) (display.Display, error) { return nil, ErrNotSupported }
