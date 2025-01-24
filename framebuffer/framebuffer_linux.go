package framebuffer

import (
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"os"
	"syscall"
	"unsafe"

	"github.com/BeatGlow/display"
	"github.com/BeatGlow/display/pixel"
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
	fd         uintptr
	info       linuxFrameBufferInfo
	screenInfo linuxVarScreenInfo
	model      color.Model
	order      binary.ByteOrder
}

// Open a Linux FrameBuffer device (fbdev) by name, typically /dev/fb[0..x].
func Open(name string) (display.Display, error) {
	f, err := os.OpenFile(name, os.O_RDWR, os.ModeDevice)
	if err != nil {
		return nil, err
	}

	fb := &linuxFrameBuffer{
		f:  f,
		fd: f.Fd(),
	}
	if err = fb.ioctl(fbioGetFScreenInfo, unsafe.Pointer(&fb.info)); err != nil {
		_ = f.Close()
		return nil, err
	}

	// Request virtual screen info.
	if err = fb.ioctl(fbioGetVScreenInfo, unsafe.Pointer(&fb.screenInfo)); err != nil {
		_ = f.Close()
		return nil, err
	}
	if fb.model, fb.order, err = linuxParseColorModel(&fb.screenInfo); err != nil {
		_ = f.Close()
		return nil, err
	}

	// Map pixel buffer.
	if fb.Buffer.Pix, err = syscall.Mmap(int(fb.fd), 0, int(fb.info.SmemLen), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED); err != nil {
		_ = f.Close()
		return nil, err
	}

	fb.Buffer.Rect = image.Rect(
		int(fb.screenInfo.Xoffset), int(fb.screenInfo.Yoffset),
		int(fb.screenInfo.Xres), int(fb.screenInfo.Yres),
	)

	fb.setImage()
	return fb, nil
}

func (fb *linuxFrameBuffer) setImage() {
	switch fb.model {
	case pixel.CBGR15Model:
		fb.Buffer.Stride = int(fb.screenInfo.Width) * 2
	case pixel.CBGR16Model:
		fb.Buffer.Stride = int(fb.screenInfo.Width) * 2
	case pixel.CRGB15Model:
		fb.Buffer.Stride = int(fb.screenInfo.Width) * 2
		fb.Image = &pixel.CRGB15Image{
			Buffer: fb.Buffer,
			Order:  fb.order,
		}
	case pixel.CRGB16Model:
		fb.Buffer.Stride = int(fb.screenInfo.Width) * 2
		fb.Image = &pixel.CRGB15Image{
			Buffer: fb.Buffer,
			Order:  fb.order,
		}
	}
}

func (fb *linuxFrameBuffer) Bounds() image.Rectangle {
	return fb.Buffer.Bounds()
}

// Close the framebuffer device
func (fb *linuxFrameBuffer) Close() error {
	if err := syscall.Munmap(fb.Buffer.Pix); err != nil {
		return err
	}
	return fb.f.Close()
}

func (fb *linuxFrameBuffer) Fill(c color.Color) {
	switch im := fb.Image.(type) {
	case *pixel.CBGR15Image:
		im.Fill(c)
	case *pixel.CBGR16Image:
		im.Fill(c)
	case *pixel.CRGB15Image:
		im.Fill(c)
	case *pixel.CRGB16Image:
		im.Fill(c)
	case *image.RGBA:
		r, g, b, a := c.RGBA()
		pix := []byte{
			byte(r >> 8),
			byte(g >> 8),
			byte(b >> 8),
			byte(a >> 8),
		}
		for i, l := 0, len(fb.Buffer.Pix); i < l; i += 4 {
			copy(fb.Buffer.Pix[i:], pix)
		}
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

// linuxPixelFormat of the framebuffer
type linuxPixelFormat int

const (
	// UnknownPixelFormat for when detection is not (yet) implemented
	linuxUnknownPixelFormat linuxPixelFormat = iota
	// BGR565 is a 16bit pixelformat
	linuxBGR565
)

func linuxParseColorModel(info *linuxVarScreenInfo) (color.Model, binary.ByteOrder, error) {
	if info == nil {
		return nil, nil, errors.New("invalid VarScreenInfo")
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
			return pixel.CBGR15Model, binary.BigEndian, nil

		case info.Red.Offset == 0 &&
			info.Red.Length == 5 &&
			info.Green.Offset == 5 &&
			info.Green.Length == 5 &&
			info.Blue.Offset == 10 &&
			info.Blue.Length == 5 &&
			info.Alpha.Length == 0:
			return pixel.CRGB15Model, binary.BigEndian, nil
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
			return pixel.CBGR16Model, binary.BigEndian, nil

		case info.Red.Offset == 0 &&
			info.Red.Length == 5 &&
			info.Green.Offset == 5 &&
			info.Green.Length == 6 &&
			info.Blue.Offset == 11 &&
			info.Blue.Length == 5 &&
			info.Alpha.Length == 0:
			return pixel.CRGB16Model, binary.BigEndian, nil
		}

	case 24:
		switch {
		case info.Red.Offset == 0 &&
			info.Red.Length == 8 &&
			info.Green.Offset == 8 &&
			info.Green.Length == 8 &&
			info.Blue.Offset == 16 &&
			info.Blue.Length == 8 &&
			info.Alpha.Length == 8:
			return color.RGBAModel, binary.BigEndian, nil
		}
	}

	return nil, nil, errors.New("framebuffer: unsupported color model")
}
