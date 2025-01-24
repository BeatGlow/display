//go:build !linux

package framebuffer

import (
	"errors"

	"github.com/BeatGlow/display"
)

var ErrNotSupported = errors.New("framebuffer: not supported")

func Open(_ string) (display.Display, error) {
	return nil, ErrNotSupported
}
