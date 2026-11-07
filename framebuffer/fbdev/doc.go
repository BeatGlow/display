// Package fbdev provides access to the Linux fbdev framebuffer device.
//
// This requires framebuffer device support in the operating system. The framebuffer
// can be opened with [Open] and used like a regular display.
//
// Note that not all framebuffers implement all methods, such as rotating or setting
// the contrast. On those implementations, these calls will be a no-op.
package fbdev
