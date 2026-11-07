//go:build !linux

package fbdev

import (
	"github.com/BeatGlow/display"
)

func Open(_ string) (display.Display, error) {
	return nil, ErrNotSupported
}
