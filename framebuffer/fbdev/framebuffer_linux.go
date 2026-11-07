package fbdev

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"
	"syscall"
	"unsafe"

	"github.com/BeatGlow/display/v2"
	"github.com/BeatGlow/display/v2/pixel"
)

const (
	// From <linux/fb.h>
	fbioGetVScreenInfo = 0x4600
	fbioGetFScreenInfo = 0x4602
)

type linuxFrameBuffer struct {
	pixel.Buffer
	pixel.Image
	f          *os.File
	name       string
	mapped     []byte
	fd         uintptr
	info       linuxFrameBufferInfo
	screenInfo linuxVarScreenInfo
	model      color.Model
	order      binary.ByteOrder
}

// Open a Linux FrameBuffer device (fbdev) by name, typically /dev/fb[0..x].
func Open(name string) (display.Display, error) {
	if name == "" {
		name = "/dev/fb0"
	}
	f, err := os.OpenFile(name, os.O_RDWR, os.ModeDevice)
	if err != nil {
		return nil, fmt.Errorf("fbdev: open %s: %w", name, err)
	}

	fb := &linuxFrameBuffer{
		f:    f,
		name: name,
		fd:   f.Fd(),
	}
	if err = fb.ioctl(fbioGetFScreenInfo, unsafe.Pointer(&fb.info)); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("fbdev: query fixed screen info: %w", err)
	}

	// Request virtual screen info.
	if err = fb.ioctl(fbioGetVScreenInfo, unsafe.Pointer(&fb.screenInfo)); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("fbdev: query variable screen info: %w", err)
	}
	if fb.model, fb.order, err = linuxParseColorModel(&fb.screenInfo); err != nil {
		_ = f.Close()
		return nil, err
	}
	if err = fb.configureImage(); err != nil {
		_ = f.Close()
		return nil, err
	}

	// Map pixel buffer.
	maxInt := uint64(^uint(0) >> 1)
	if fb.info.SmemLen == 0 || uint64(fb.info.SmemLen) > maxInt {
		_ = f.Close()
		return nil, errors.New("fbdev: invalid framebuffer memory length")
	}
	if fb.mapped, err = syscall.Mmap(int(fb.fd), 0, int(fb.info.SmemLen), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("fbdev: map framebuffer memory: %w", err)
	}
	if err = fb.setMapping(fb.mapped); err != nil {
		_ = syscall.Munmap(fb.mapped)
		_ = f.Close()
		return nil, err
	}
	return fb, nil
}

func (fb *linuxFrameBuffer) configureImage() error {
	info := &fb.screenInfo
	if info.Xres == 0 || info.Yres == 0 {
		return errors.New("fbdev: invalid framebuffer dimensions")
	}
	bytesPerPixel := uint64((info.BitsPerPixel + 7) / 8)
	if bytesPerPixel == 0 || fb.info.LineLength == 0 || uint64(fb.info.LineLength) < (uint64(info.Xoffset)+uint64(info.Xres))*bytesPerPixel {
		return errors.New("fbdev: line length is shorter than the active pixel row")
	}
	if uint64(info.Xoffset)+uint64(info.Xres) > uint64(info.XresVirtual) || uint64(info.Yoffset)+uint64(info.Yres) > uint64(info.YresVirtual) {
		return errors.New("fbdev: active view exceeds virtual framebuffer dimensions")
	}
	lastRow := uint64(info.Yoffset) + uint64(info.Yres) - 1
	rowBytes := (uint64(info.Xoffset) + uint64(info.Xres)) * bytesPerPixel
	if fb.info.SmemLen == 0 || lastRow > uint64(fb.info.SmemLen)/uint64(fb.info.LineLength) {
		return errors.New("fbdev: active view exceeds framebuffer memory")
	}
	lastRowStart := lastRow * uint64(fb.info.LineLength)
	if rowBytes > uint64(fb.info.SmemLen)-lastRowStart {
		return errors.New("fbdev: active view exceeds framebuffer memory")
	}
	maxInt := uint64(^uint(0) >> 1)
	if uint64(info.Xres) > maxInt || uint64(info.Yres) > maxInt || uint64(fb.info.LineLength) > maxInt {
		return errors.New("fbdev: framebuffer dimensions exceed process limits")
	}
	return nil
}

func (fb *linuxFrameBuffer) setMapping(mapped []byte) error {
	bytesPerPixel := uint64((fb.screenInfo.BitsPerPixel + 7) / 8)
	offset := uint64(fb.screenInfo.Yoffset)*uint64(fb.info.LineLength) + uint64(fb.screenInfo.Xoffset)*bytesPerPixel
	viewLen := uint64(fb.screenInfo.Yres-1)*uint64(fb.info.LineLength) + uint64(fb.screenInfo.Xres)*bytesPerPixel
	if offset > uint64(len(mapped)) || viewLen > uint64(len(mapped))-offset {
		return errors.New("fbdev: active framebuffer view exceeds mapped memory")
	}
	fb.mapped = mapped
	fb.Pix = mapped[int(offset):int(offset+viewLen)]
	fb.Rect = image.Rect(0, 0, int(fb.screenInfo.Xres), int(fb.screenInfo.Yres))
	fb.Stride = int(fb.info.LineLength)
	fb.setImage()
	return nil
}

func (fb *linuxFrameBuffer) setImage() {
	switch fb.model {
	case pixel.CBGR15Model:
		fb.Image = &pixel.CBGR15Image{Buffer: fb.Buffer, Order: fb.order}
	case pixel.CBGR16Model:
		fb.Image = &pixel.CBGR16Image{Buffer: fb.Buffer, Order: fb.order}
	case pixel.CRGB15Model:
		fb.Image = &pixel.CRGB15Image{
			Buffer: fb.Buffer,
			Order:  fb.order,
		}
	case pixel.CRGB16Model:
		fb.Image = &pixel.CRGB16Image{
			Buffer: fb.Buffer,
			Order:  fb.order,
		}
	}
}

func (fb *linuxFrameBuffer) Bounds() image.Rectangle {
	return fb.Rect
}

func (fb *linuxFrameBuffer) String() string {
	return fb.name
}

// Clear clears the mapped framebuffer.
func (fb *linuxFrameBuffer) Clear() {
	fb.Fill(color.Black)
}

// Close the framebuffer device
func (fb *linuxFrameBuffer) Close() error {
	var err error
	if fb.mapped != nil {
		err = syscall.Munmap(fb.mapped)
		fb.mapped = nil
	}
	if fb.f != nil {
		err = errors.Join(err, fb.f.Close())
		fb.f = nil
	}
	return err
}

func (fb *linuxFrameBuffer) Fill(c color.Color) {
	if fb.Image != nil {
		fb.Image.Fill(c)
	}
}

// Show toggles the display on or off.
func (fb *linuxFrameBuffer) Show(_ bool) error {
	return nil
}

// SetContrast adjusts the contrast level.
func (fb *linuxFrameBuffer) SetContrast(level uint8) error {
	return nil
}

// SetRotation adjusts the pixel rotation.
func (fb *linuxFrameBuffer) SetRotation(_ display.Rotation) error {
	return nil
}

// Refresh redraws the display.
func (fb *linuxFrameBuffer) Refresh() error {
	return nil
}

func (d *linuxFrameBuffer) ioctl(cmd uintptr, arg unsafe.Pointer) (err error) {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, d.fd, cmd, uintptr(arg)); errno != 0 {
		return &os.SyscallError{
			Syscall: "SYS_IOCTL",
			Err:     errno,
		}
	}
	return nil
}

type linuxFrameBufferInfo struct {
	ID         [16]byte  // Identification string eg "TT Builtin"
	SmemStart  uintptr   // Start of frame buffer mem
	SmemLen    uint32    // Length of frame buffer mem
	Type       uint32    // FB_TYPE_
	TypeAux    uint32    // Interleave for interleaved Planes
	Visual     uint32    // FB_VISUAL_
	Xpanstep   uint16    // Zero if no hardware panning
	Ypanstep   uint16    // Zero if no hardware panning
	Ywrapstep  uint16    // Zero if no hardware ywrap
	LineLength uint32    // Length of a line in bytes
	MmioStart  uintptr   // Start of Memory Mapped I/O (physical address)
	MmioLen    uint32    // Length of Memory Mapped I/O
	Accel      uint32    // Type of acceleration available
	Reserved   [3]uint16 // Reserved for future compatibility
}

// linuxBitField for the color
type linuxBitField struct {
	Offset   uint32 // Beginning of bitfield
	Length   uint32 // Length of bitfield
	MsbRight uint32 // != 0 : Most significant bit is right
}

// linuxVarScreenInfo contains device independent changeable information about a frame buffer device and a specific video mode.
type linuxVarScreenInfo struct {
	Xres                    uint32
	Yres                    uint32
	XresVirtual             uint32
	YresVirtual             uint32
	Xoffset                 uint32
	Yoffset                 uint32
	BitsPerPixel            uint32
	Grayscale               uint32
	Red, Green, Blue, Alpha linuxBitField
	Nonstd                  uint32
	Activate                uint32
	Height                  uint32
	Width                   uint32
	AccelFlags              uint32
	Pixclock                uint32
	LeftMargin              uint32
	RightMargin             uint32
	UpperMargin             uint32
	LowerMargin             uint32
	HsyncLen                uint32
	VsyncLen                uint32
	Sync                    uint32
	Vmode                   uint32
	Rotate                  uint32
	Colorspace              uint32
	Reserved                [4]uint32
}

func linuxParseColorModel(info *linuxVarScreenInfo) (color.Model, binary.ByteOrder, error) {
	if info == nil {
		return nil, nil, errors.New("fbdev: invalid variable screen info")
	}
	if info.Red.MsbRight != 0 || info.Green.MsbRight != 0 || info.Blue.MsbRight != 0 || info.Alpha.MsbRight != 0 {
		return nil, nil, errors.New("fbdev: reversed color bitfields are unsupported")
	}

	switch info.BitsPerPixel {
	case 15:
		switch {
		case info.Blue.Offset == 0 &&
			info.Blue.Length == 5 &&
			info.Green.Offset == 5 &&
			info.Green.Length == 5 &&
			info.Red.Offset == 10 &&
			info.Red.Length == 5 &&
			info.Alpha.Length == 0:
			return pixel.CRGB15Model, binary.NativeEndian, nil

		case info.Red.Offset == 0 &&
			info.Red.Length == 5 &&
			info.Green.Offset == 5 &&
			info.Green.Length == 5 &&
			info.Blue.Offset == 10 &&
			info.Blue.Length == 5 &&
			info.Alpha.Length == 0:
			return pixel.CBGR15Model, binary.NativeEndian, nil
		}

	case 16:
		switch {
		case info.Blue.Offset == 0 &&
			info.Blue.Length == 5 &&
			info.Green.Offset == 5 &&
			info.Green.Length == 6 &&
			info.Red.Offset == 11 &&
			info.Red.Length == 5 &&
			info.Alpha.Length == 0:
			return pixel.CRGB16Model, binary.NativeEndian, nil

		case info.Red.Offset == 0 &&
			info.Red.Length == 5 &&
			info.Green.Offset == 5 &&
			info.Green.Length == 6 &&
			info.Blue.Offset == 11 &&
			info.Blue.Length == 5 &&
			info.Alpha.Length == 0:
			return pixel.CBGR16Model, binary.NativeEndian, nil
		}

	}

	return nil, nil, errors.New("fbdev: unsupported color model")
}
