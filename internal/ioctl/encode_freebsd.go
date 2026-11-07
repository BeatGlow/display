//go:build freebsd

package ioctl

const ioctlSizeMask = 0x1fff

// Encode builds a FreeBSD _IOC request number using FreeBSD 15.1's
// sys/sys/ioccom.h definitions and the drm-kmod 6.12.110 DRM UAPI.
func Encode(mode Mode, size uint16, cmd uintptr) Command {
	return encodeFreeBSD(mode, size, cmd)
}
