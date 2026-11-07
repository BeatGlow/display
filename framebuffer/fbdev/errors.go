package fbdev

import "errors"

// ErrNotSupported indicates that fbdev is unavailable on this operating system.
var ErrNotSupported = errors.New("fbdev: not supported")
