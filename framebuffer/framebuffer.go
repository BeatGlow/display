// Package framebuffer provides access to the operating system's native framebuffer
//
// This requires framebuffer device support in the operating system. The framebuffer
// can be opened with the [Open] call, and will otherwise function like a regular
// display.
//
// Note that not all framebuffers implement all methods, such as rotating or setting
// the contrast. On those implementations, these calls will be a no-op.
package framebuffer
