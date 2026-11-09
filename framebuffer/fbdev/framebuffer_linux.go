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

	"github.com/BeatGlow/display"
	"github.com/BeatGlow/display/internal/rotation"
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
	f           *os.File
	name        string
	mapped      []byte
	fd          uintptr
	info        linuxFrameBufferInfo
	screenInfo  linuxVarScreenInfo
	model       color.Model
	order       binary.ByteOrder
	nativeImage pixel.Image
	rotation    display.Rotation
	scratch     []byte
	row         []byte
}

// Open opens a Linux framebuffer device by name, typically /dev/fb0.
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
		fb.nativeImage = &pixel.CBGR15Image{Buffer: fb.Buffer, Order: fb.order}
	case pixel.CBGR16Model:
		fb.nativeImage = &pixel.CBGR16Image{Buffer: fb.Buffer, Order: fb.order}
	case pixel.CRGB15Model:
		fb.nativeImage = &pixel.CRGB15Image{
			Buffer: fb.Buffer,
			Order:  fb.order,
		}
	case pixel.CRGB16Model:
		fb.nativeImage = &pixel.CRGB16Image{
			Buffer: fb.Buffer,
			Order:  fb.order,
		}
	}
	fb.applyRotation()
}

func (fb *linuxFrameBuffer) Bounds() image.Rectangle { return fb.Image.Bounds() }

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

// CopyFrom replaces the framebuffer from an image. RGBA sources use a direct
// packed-pixel copy that applies the current rotation without image interface
// calls per pixel.
func (fb *linuxFrameBuffer) CopyFrom(src image.Image) error {
	if src == nil || fb.nativeImage == nil {
		return errors.New("fbdev: source image and framebuffer must be available")
	}
	if src.Bounds().Size() != fb.Bounds().Size() {
		return errors.New("fbdev: source image size does not match the display")
	}
	rgba, ok := src.(*image.RGBA)
	if !ok {
		bounds := src.Bounds()
		for y := 0; y < bounds.Dy(); y++ {
			for x := 0; x < bounds.Dx(); x++ {
				fb.Set(x, y, src.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
		return nil
	}
	view := fb.nativeImage.(pixel.FastImage).PixelView()
	sourceBounds := rgba.Bounds()
	nativeBounds := fb.nativeImage.Bounds()
	logicalWidth, logicalHeight := fb.Bounds().Dx(), fb.Bounds().Dy()
	scratchLen := logicalWidth * logicalHeight * 2
	if cap(fb.scratch) < scratchLen {
		fb.scratch = make([]byte, scratchLen)
	} else {
		fb.scratch = fb.scratch[:scratchLen]
	}
	rowLen := nativeBounds.Dx() * 2
	if cap(fb.row) < rowLen {
		fb.row = make([]byte, rowLen)
	} else {
		fb.row = fb.row[:rowLen]
	}
	packRGBA16(fb.scratch, rgba, sourceBounds, view.Format, isLittleEndian(fb.order))
	copyPackedRotation(view.Pix, view.Stride, fb.scratch, logicalWidth, nativeBounds, fb.rotation, fb.row)
	return nil
}

func isLittleEndian(order binary.ByteOrder) bool {
	var probe [2]byte
	order.PutUint16(probe[:], 0x0102)
	return probe[0] == 0x02
}

func packRGBA16(dst []byte, src *image.RGBA, sourceBounds image.Rectangle, format pixel.PixelFormat, littleEndian bool) {
	redShift, greenShift, blueShift := uint(11), uint(5), uint(0)
	greenBits := uint(6)
	switch format {
	case pixel.FormatCRGB15:
		redShift, greenShift, greenBits = 10, 5, 5
	case pixel.FormatCBGR15:
		redShift, greenShift, blueShift, greenBits = 0, 5, 10, 5
	case pixel.FormatCBGR16:
		redShift, blueShift = 0, 11
	}
	for y := 0; y < sourceBounds.Dy(); y++ {
		destination := dst[y*sourceBounds.Dx()*2:]
		source := src.Pix[src.PixOffset(sourceBounds.Min.X, sourceBounds.Min.Y+y):]
		for x := 0; x < sourceBounds.Dx(); x++ {
			si := x * 4
			red, green, blue := source[si], source[si+1], source[si+2]
			value := uint16(red>>3)<<redShift | uint16(green>>(8-greenBits))<<greenShift | uint16(blue>>3)<<blueShift
			di := x * 2
			if littleEndian {
				destination[di], destination[di+1] = byte(value), byte(value>>8)
			} else {
				destination[di], destination[di+1] = byte(value>>8), byte(value)
			}
		}
	}
}

func copyPackedRotation(dst []byte, dstStride int, src []byte, logicalWidth int, nativeBounds image.Rectangle, r display.Rotation, row []byte) {
	w, h := nativeBounds.Dx(), nativeBounds.Dy()
	logicalStride := logicalWidth * 2
	for y := 0; y < h; y++ {
		destination := dst[y*dstStride:]
		switch r {
		case display.NoRotation:
			copy(destination[:w*2], src[y*logicalStride:y*logicalStride+w*2])
		case display.Rotate180:
			source := src[(h-1-y)*logicalStride:]
			for x := 0; x < w; x++ {
				row[x*2], row[x*2+1] = source[(w-1-x)*2], source[(w-1-x)*2+1]
			}
			copy(destination[:w*2], row[:w*2])
		case display.Rotate90:
			for x := 0; x < w; x++ {
				offset := x*logicalStride + (h-1-y)*2
				row[x*2], row[x*2+1] = src[offset], src[offset+1]
			}
			copy(destination[:w*2], row[:w*2])
		case display.Rotate270:
			for x := 0; x < w; x++ {
				offset := (w-1-x)*logicalStride + y*2
				row[x*2], row[x*2+1] = src[offset], src[offset+1]
			}
			copy(destination[:w*2], row[:w*2])
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

func (fb *linuxFrameBuffer) SetRotation(rotation display.Rotation) error {
	fb.rotation = rotation % 4
	fb.applyRotation()
	return nil
}

func (fb *linuxFrameBuffer) applyRotation() {
	if fb.nativeImage == nil {
		return
	}
	if fb.rotation == display.NoRotation {
		fb.Image = fb.nativeImage
		return
	}
	fb.Image = &rotatedImage{Image: fb.nativeImage, rotation: fb.rotation}
}

// rotatedImage maps logical display coordinates onto the native framebuffer.
type rotatedImage struct {
	Image    pixel.Image
	rotation display.Rotation
}

func (i *rotatedImage) Bounds() image.Rectangle {
	return rotation.Bounds(i.Image.Bounds(), i.rotation)
}

func (i *rotatedImage) ColorModel() color.Model {
	return i.Image.(interface{ ColorModel() color.Model }).ColorModel()
}
func (i *rotatedImage) At(x, y int) color.Color {
	if !image.Pt(x, y).In(i.Bounds()) {
		return color.Transparent
	}
	x, y = rotation.Point(i.Image.Bounds(), i.rotation, x, y)
	return i.Image.At(x, y)
}
func (i *rotatedImage) Set(x, y int, c color.Color) {
	if !image.Pt(x, y).In(i.Bounds()) {
		return
	}
	x, y = rotation.Point(i.Image.Bounds(), i.rotation, x, y)
	i.Image.Set(x, y, c)
}
func (i *rotatedImage) Fill(c color.Color) { i.Image.Fill(c) }
func (i *rotatedImage) Clear()             { i.Image.Clear() }

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
