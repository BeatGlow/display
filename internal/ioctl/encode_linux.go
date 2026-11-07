//go:build linux

package ioctl

const ioctlSizeMask = 0x3fff

// Encode builds a Linux _IOC request number.
func Encode(mode Mode, size uint16, cmd uintptr) Command {
	return Command(mode)<<30 | Command(size)<<16 | Command(cmd)
}
