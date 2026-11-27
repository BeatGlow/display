package streamdeck

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/BeatGlow/display/internal/ioctl"
)

const usbBus = 0x0003

type hidrawDevice struct {
	*os.File
	raw syscall.RawConn
}

type hidrawInfo struct {
	bus     uint32
	vendor  uint16
	product uint16
}

func enumerateHID() ([]hidIdentity, error) {
	const sysfs = "/sys/class/hidraw"
	entries, err := os.ReadDir(sysfs)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var identities []hidIdentity
	prefix := fmt.Sprintf("HID_ID=%04X:%08X:", usbBus, elgatoVendorID)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "hidraw") {
			continue
		}
		properties, err := os.ReadFile(filepath.Join(sysfs, name, "device", "uevent"))
		if errors.Is(err, fs.ErrNotExist) {
			continue // The device may have been unplugged since ReadDir.
		}
		if err != nil {
			return nil, fmt.Errorf("identify %s: %w", name, err)
		}
		for line := range strings.SplitSeq(string(properties), "\n") {
			value, ok := strings.CutPrefix(line, prefix)
			if !ok {
				continue
			}
			if id, err := strconv.ParseUint(value, 16, 16); err == nil {
				identities = append(identities, hidIdentity{path: filepath.Join("/dev", name), productID: uint16(id)})
			}
			break
		}
	}
	return identities, nil
}

func resolvePath(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}

func openHID(path string, expectedProduct uint16) (transport, error) {
	file, err := os.OpenFile(path, os.O_RDWR|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}

	hid, err := identifyHID(file, expectedProduct)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return hid, nil
}

func identifyHID(file *os.File, expectedProduct uint16) (*hidrawDevice, error) {
	raw, err := file.SyscallConn()
	if err != nil {
		return nil, err
	}
	var info hidrawInfo
	var ioctlErr syscall.Errno
	const hidGetRawInfo = 'H'<<8 | 0x03 // HIDIOCGRAWINFO
	command := ioctl.Encode(ioctl.Read, uint16(unsafe.Sizeof(info)), hidGetRawInfo)
	err = raw.Control(func(fd uintptr) {
		_, _, ioctlErr = syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(command), uintptr(unsafe.Pointer(&info)))
	})
	if err != nil {
		return nil, err
	}
	if ioctlErr != 0 {
		return nil, fmt.Errorf("HIDIOCGRAWINFO: %w", ioctlErr)
	}
	if info.bus != usbBus || info.vendor != elgatoVendorID || info.product != expectedProduct {
		return nil, fmt.Errorf("unexpected HID device %04x:%04x on bus %04x", info.vendor, info.product, info.bus)
	}
	if err := file.SetReadDeadline(time.Time{}); err != nil {
		return nil, fmt.Errorf("enable interruptible input: %w", err)
	}
	return &hidrawDevice{File: file, raw: raw}, nil
}

func (d *hidrawDevice) readReport(ctx context.Context, report []byte) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if ctx.Done() == nil {
		return d.Read(report)
	}

	// Let the poller wake the read without closing the shared HID handle.
	// Join the callback before clearing the deadline for the next read.
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		_ = d.SetReadDeadline(time.Now())
		close(interrupted)
	})
	n, err := d.Read(report)
	if !stop() {
		<-interrupted
		_ = d.SetReadDeadline(time.Time{})
	}
	if err != nil && ctx.Err() != nil {
		return 0, ctx.Err()
	}
	return n, err
}

func (d *hidrawDevice) writeReport(report []byte) error {
	var n int
	var writeErr error
	// Retrying a partial write would send its remaining bytes as a separate report.
	err := d.raw.Write(func(fd uintptr) bool {
		for {
			n, writeErr = syscall.Write(int(fd), report)
			if writeErr != syscall.EINTR {
				return writeErr != syscall.EAGAIN
			}
		}
	})
	if err != nil {
		return err
	}
	if writeErr != nil {
		return writeErr
	}
	if n != len(report) {
		return io.ErrShortWrite
	}
	return nil
}
