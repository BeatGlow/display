//go:build !linux && !freebsd

package ioctl

const ioctlSizeMask = 0x3fff

// Encode builds the Linux-compatible _IOC request number used by other
// supported ioctl targets.
func Encode(mode Mode, size uint16, cmd uintptr) Command {
	return Command(mode)<<30 | Command(size)<<16 | Command(cmd)
}
