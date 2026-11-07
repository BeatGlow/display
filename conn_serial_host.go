//go:build !freebsd

package display

// The pinned periph host serial backend references baud constants unavailable
// in its FreeBSD build, so register it only on supported operating systems.
import _ "periph.io/x/host/v3/serial"
