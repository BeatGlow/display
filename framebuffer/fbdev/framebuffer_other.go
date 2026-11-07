//go:build !linux

package fbdev

import (
	"github.com/BeatGlow/display/v2"
)

func Open(_ string) (display.Display, error) {
	return nil, ErrNotSupported
}
