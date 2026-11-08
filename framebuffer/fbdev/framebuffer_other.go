//go:build !linux

package fbdev

import (
	"github.com/BeatGlow/display"
)

// Open reports that framebuffer devices are unavailable on this platform.
func Open(_ string) (display.Display, error) {
	return nil, ErrNotSupported
}
