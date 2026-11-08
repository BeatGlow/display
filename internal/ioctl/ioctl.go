package ioctl

import (
	"fmt"
	"reflect"
	"syscall"
)

// Mode is the IOCTL mode.
type Mode uint8

const (
	// None indicates that an ioctl command transfers no data.
	None Mode = iota
	// Write indicates that the caller writes data to the device.
	Write
	// Read indicates that the caller reads data from the device.
	Read
)

// Command is an encoded ioctl request.
type Command uintptr

// String returns a readable representation of the ioctl request.
func (c Command) String() string {
	return fmt.Sprintf("ioctl (%d bytes) 0x%04x", c>>16&ioctlSizeMask, uintptr(c&0xffff))
}

// Do executes the ioctl [Command] using ptr as its argument.
func Do(fd uintptr, command Command, ptr any) error {
	var p uintptr

	if ptr != nil {
		v := reflect.ValueOf(ptr)
		p = v.Pointer()
	}

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(command), p)
	if errno != 0 {
		return fmt.Errorf("ioctl %s failed: %v", command, errno)
	}
	return nil
}

// Call does a plain ioctl system call.
func Call(fd, command, arg uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, command, arg)
	if errno != 0 {
		return fmt.Errorf("ioctl %s failed: %v", Command(command), errno)
	}
	return nil
}

// Pointer encodes an ioctl request with [Encode] for a value of the same type as ref.
func Pointer(mode Mode, ref any, cmd uintptr) Command {
	size := uint16(reflect.TypeOf(ref).Elem().Size())
	return Encode(mode, size, cmd)
}
