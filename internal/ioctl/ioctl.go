package ioctl

import (
	"fmt"
	"reflect"
	"syscall"
)

// Mode is the IOCTL mode.
type Mode uint8

// Modes
const (
	None Mode = iota
	Write
	Read
)

// Command to be sent over ioctl.
type Command uintptr

func (c Command) String() string {
	var (
		mode = Mode(c >> 30 & 0x03)
		size = c >> 16 & 0x3fff
		cmd  = c & 0xffff
		str  string
	)
	if mode&Write > 0 {
		str += " write"
	}
	if mode&Read > 0 {
		str += " read "
	}
	return fmt.Sprintf("ioctl%s (%d bytes) 0x%04x", str, size, uintptr(cmd))
}

// Do executes the ioctl call.
func Do(fd uintptr, command Command, ptr interface{}) error {
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

// Encode an ioctl command.
func Encode(mode Mode, size uint16, cmd uintptr) Command {
	return Command(mode)<<30 | Command(size)<<16 | Command(cmd)
}

// Pointer to a value.
func Pointer(mode Mode, ref interface{}, cmd uintptr) Command {
	size := uint16(reflect.TypeOf(ref).Elem().Size())
	return Encode(mode, size, cmd)
}
