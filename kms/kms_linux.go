//go:build linux

package kms

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	stdDraw "image/draw"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/BeatGlow/display"
	"github.com/BeatGlow/display/internal/buffer"
	"github.com/BeatGlow/display/internal/ioctl"
)

const (
	drmIOSetMaster  = 0x641e
	drmIODropMaster = 0x641f
	// DRM ioctl numbers combine type 'd' (0x64) with the command number.
	drmModeGetResources = 0x64A0
	drmModeGetCRTC      = 0x64A1
	drmModeSetCRTC      = 0x64A2
	drmModeGetEncoder   = 0x64A6
	drmModeGetConnector = 0x64A7
	drmModePageFlip     = 0x64B0
	drmModeCreateDumb   = 0x64B2
	drmModeMapDumb      = 0x64B3
	drmModeDestroyDumb  = 0x64B4
	drmModeAddFB        = 0x64AE
	drmModeRMFB         = 0x64AF

	drmModeConnected            = 1
	drmModeTypePreferred        = 1 << 3
	drmModePageFlipEvent        = 1
	drmEventFlipComplete        = 2
	drmModeConnectorHDMIA       = 11
	drmModeConnectorHDMIB       = 12
	drmModeConnectorDSI         = 16
	drmModeConnectorDisplayPort = 10
	drmModeConnectorLVDS        = 7
	drmModeConnectorVGA         = 1
	drmModeConnectorDVIi        = 2
	drmModeConnectorDVId        = 3
	drmModeConnectorDVIa        = 4
	drmModeConnectorComposite   = 5
	drmModeConnectorSVideo      = 6
	drmModeConnectorComponent   = 8
	drmModeConnectorDIN         = 9
	drmModeConnectorTV          = 13
	drmModeConnectorEDP         = 14
	drmModeConnectorVirtual     = 15
	drmModeConnectorDPI         = 17
	drmModeConnectorWriteback   = 18
	drmModeConnectorSPI         = 19
	drmModeConnectorUSB         = 20
)

// Open opens and modesets a DRM/KMS output. The process must be able to acquire
// DRM master; an active compositor normally owns that access.
func Open(config *Config) (display.Display, error) {
	if config == nil {
		config = &Config{}
	}
	if (config.Width == 0) != (config.Height == 0) || config.Width < 0 || config.Height < 0 {
		return nil, errors.New("kms: width and height must both be zero or positive")
	}
	device := config.Device
	if device == "" {
		device = "/dev/dri/card0"
	}
	f, err := os.OpenFile(device, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("kms: open %s: %w", device, err)
	}
	d := &displayDevice{file: f}
	if err = ioctl.Call(f.Fd(), drmIOSetMaster, 0); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("kms: acquire DRM master: %w", err)
	}
	d.master = true
	if err = d.init(config); err != nil {
		return nil, errors.Join(err, d.Close())
	}
	return d, nil
}

var _ display.Display = (*displayDevice)(nil)

type displayDevice struct {
	mu          sync.Mutex
	file        *os.File
	master      bool
	closed      bool
	active      bool
	modeSet     bool
	shadow      *image.RGBA
	mode        modeInfo
	connectorID uint32
	crtcID      uint32
	front       int
	back        int
	buffers     [2]scanoutBuffer
	saved       crtcState
}

type scanoutBuffer struct {
	handle uint32
	fbID   uint32
	pitch  uint32
	size   uint64
	pix    []byte
}

type crtcState struct {
	fbID         uint32
	x, y         uint32
	modeValid    uint32
	mode         modeInfo
	connectorIDs []uint32
}

func (d *displayDevice) init(config *Config) error {
	resources, err := d.resources()
	if err != nil {
		return err
	}
	connectors, err := d.connectors(resources.connectorIDs)
	if err != nil {
		return err
	}
	connector, mode, err := chooseConnector(connectors, config)
	if err != nil {
		return err
	}
	d.connectorID = connector.kernel.connectorID
	d.mode = mode
	d.shadow = image.NewRGBA(image.Rect(0, 0, int(mode.hDisplay), int(mode.vDisplay)))
	crtc, err := d.chooseCRTC(connector, resources)
	if err != nil {
		return err
	}
	d.crtcID = crtc
	d.saved, err = d.getCRTC(crtc)
	if err != nil {
		return fmt.Errorf("kms: read CRTC %d: %w", crtc, err)
	}
	for _, c := range connectors {
		if c.kernel.encoderID == 0 {
			continue
		}
		enc, e := d.getEncoder(c.kernel.encoderID)
		if e != nil {
			return fmt.Errorf("kms: inspect connector %s route: %w", c.name, e)
		}
		if enc.crtcID == crtc && d.saved.fbID != 0 {
			d.saved.connectorIDs = append(d.saved.connectorIDs, c.kernel.connectorID)
		}
	}
	for i := range d.buffers {
		if err = d.createBuffer(&d.buffers[i], uint32(mode.hDisplay), uint32(mode.vDisplay)); err != nil {
			return err
		}
		if err = buffer.CopyToXRGB(d.buffers[i].pix, int(d.buffers[i].pitch), d.shadow); err != nil {
			return err
		}
	}
	d.front, d.back = 0, 1
	if err = d.setCRTC(d.buffers[d.front].fbID, mode, []uint32{d.connectorID}); err != nil {
		return fmt.Errorf("kms: modeset connector %s to %dx%d: %w", connector.name, mode.hDisplay, mode.vDisplay, err)
	}
	d.active = true
	d.modeSet = true
	return nil
}

func (d *displayDevice) resources() (cardResources, error) {
	var r modeResources
	if err := ioctl.Do(d.file.Fd(), ioctl.Pointer(ioctl.Read|ioctl.Write, &r, drmModeGetResources), &r); err != nil {
		return cardResources{}, fmt.Errorf("kms: query resources: %w", err)
	}
	fbIDs := make([]uint32, r.countFBs)
	crtcIDs := make([]uint32, r.countCRTCs)
	connectorIDs := make([]uint32, r.countConnectors)
	encoderIDs := make([]uint32, r.countEncoders)
	r.fbPtr = slicePointer(fbIDs)
	r.crtcPtr = slicePointer(crtcIDs)
	r.connectorPtr = slicePointer(connectorIDs)
	r.encoderPtr = slicePointer(encoderIDs)
	if err := ioctl.Do(d.file.Fd(), ioctl.Pointer(ioctl.Read|ioctl.Write, &r, drmModeGetResources), &r); err != nil {
		return cardResources{}, fmt.Errorf("kms: read resources: %w", err)
	}
	runtime.KeepAlive(fbIDs)
	runtime.KeepAlive(crtcIDs)
	runtime.KeepAlive(connectorIDs)
	runtime.KeepAlive(encoderIDs)
	return cardResources{crtcIDs: crtcIDs, connectorIDs: connectorIDs}, nil
}

func (d *displayDevice) connectors(ids []uint32) ([]connectorInfo, error) {
	var result []connectorInfo
	for _, id := range ids {
		c, err := d.connector(id)
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, nil
}

func (d *displayDevice) connector(id uint32) (connectorInfo, error) {
	c := connectorInfo{kernel: modeConnector{connectorID: id}}
	var modeProbe [1]modeInfo
	c.kernel.countModes = uint32(len(modeProbe))
	c.kernel.modesPtr = slicePointer(modeProbe[:])
	if err := ioctl.Do(d.file.Fd(), ioctl.Pointer(ioctl.Read|ioctl.Write, &c.kernel, drmModeGetConnector), &c.kernel); err != nil {
		return connectorInfo{}, fmt.Errorf("kms: query connector %d: %w", id, err)
	}
	runtime.KeepAlive(modeProbe)
	if c.kernel.countModes == 0 {
		c.name = connectorName(c.kernel.connectorType, c.kernel.connectorTypeID)
		return c, nil
	}

	for attempt := 0; attempt < 3; attempt++ {
		modes := make([]modeInfo, c.kernel.countModes)
		encoders := make([]uint32, c.kernel.countEncoders)
		properties := make([]uint32, c.kernel.countProps)
		propertyValues := make([]uint64, c.kernel.countProps)
		c.kernel.countModes = uint32(len(modes))
		c.kernel.countEncoders = uint32(len(encoders))
		c.kernel.countProps = uint32(len(properties))
		c.kernel.modesPtr = slicePointer(modes)
		c.kernel.encodersPtr = slicePointer(encoders)
		c.kernel.propsPtr = slicePointer(properties)
		c.kernel.propValuesPtr = slicePointer(propertyValues)
		if err := ioctl.Do(d.file.Fd(), ioctl.Pointer(ioctl.Read|ioctl.Write, &c.kernel, drmModeGetConnector), &c.kernel); err != nil {
			return connectorInfo{}, fmt.Errorf("kms: read connector %d data: %w", id, err)
		}
		runtime.KeepAlive(modes)
		runtime.KeepAlive(encoders)
		runtime.KeepAlive(properties)
		runtime.KeepAlive(propertyValues)
		if c.kernel.countModes == 0 {
			c.modes = nil
			c.encoders = nil
			break
		}
		if c.kernel.countModes == uint32(len(modes)) &&
			c.kernel.countEncoders == uint32(len(encoders)) &&
			c.kernel.countProps == uint32(len(properties)) {
			c.modes = modes
			c.encoders = encoders
			break
		}
		if attempt == 2 {
			return connectorInfo{}, fmt.Errorf("kms: connector %d changed while reading its modes", id)
		}
	}
	c.name = connectorName(c.kernel.connectorType, c.kernel.connectorTypeID)
	return c, nil
}

func chooseConnector(connectors []connectorInfo, config *Config) (connectorInfo, modeInfo, error) {
	var candidates []connectorInfo
	for _, c := range connectors {
		if c.kernel.connection != drmModeConnected || len(c.modes) == 0 {
			continue
		}
		if config.Connector != "" && !strings.EqualFold(config.Connector, c.name) {
			continue
		}
		candidates = append(candidates, c)
	}
	if len(candidates) == 0 {
		return connectorInfo{}, modeInfo{}, fmt.Errorf("%w: %s", ErrNoConnector, config.Connector)
	}
	if config.Connector == "" && len(candidates) != 1 {
		return connectorInfo{}, modeInfo{}, fmt.Errorf("kms: %d connected outputs; select one with Config.Connector", len(candidates))
	}
	c := candidates[0]
	if config.Width > 0 {
		for _, m := range c.modes {
			if int(m.hDisplay) == config.Width && int(m.vDisplay) == config.Height {
				return c, m, nil
			}
		}
		return connectorInfo{}, modeInfo{}, fmt.Errorf("kms: connector %s has no %dx%d mode", c.name, config.Width, config.Height)
	}
	for _, m := range c.modes {
		if m.typ&drmModeTypePreferred != 0 {
			return c, m, nil
		}
	}
	return c, c.modes[0], nil
}

func (d *displayDevice) chooseCRTC(c connectorInfo, r cardResources) (uint32, error) {
	encoders := append([]uint32(nil), c.encoders...)
	if c.kernel.encoderID != 0 {
		encoders = append([]uint32{c.kernel.encoderID}, encoders...)
	}
	seen := make(map[uint32]bool)
	var currentCRTC, unusedCRTC uint32
	for _, id := range encoders {
		if seen[id] {
			continue
		}
		seen[id] = true
		e, err := d.getEncoder(id)
		if err != nil {
			return 0, fmt.Errorf("kms: inspect encoder %d: %w", id, err)
		}
		for i, id := range r.crtcIDs {
			if i >= 32 || e.possibleCRTCs&(1<<uint(i)) == 0 {
				continue
			}
			if id == e.crtcID {
				currentCRTC = id
				continue
			}
			state, err := d.getCRTC(id)
			if err != nil {
				return 0, fmt.Errorf("kms: inspect CRTC %d: %w", id, err)
			}
			if unusedCRTC == 0 && (state.fbID == 0 || state.modeValid == 0) {
				unusedCRTC = id
			}
		}
	}
	if currentCRTC != 0 {
		return currentCRTC, nil
	}
	if unusedCRTC != 0 {
		return unusedCRTC, nil
	}
	return 0, errors.New("kms: no unused compatible CRTC")
}

func (d *displayDevice) getEncoder(id uint32) (modeEncoder, error) {
	e := modeEncoder{encoderID: id}
	err := ioctl.Do(d.file.Fd(), ioctl.Pointer(ioctl.Read|ioctl.Write, &e, drmModeGetEncoder), &e)
	return e, err
}

func (d *displayDevice) getCRTC(id uint32) (crtcState, error) {
	c := modeCRTC{crtcID: id}
	if err := ioctl.Do(d.file.Fd(), ioctl.Pointer(ioctl.Read|ioctl.Write, &c, drmModeGetCRTC), &c); err != nil {
		return crtcState{}, err
	}
	return crtcState{fbID: c.fbID, x: c.x, y: c.y, modeValid: c.modeValid, mode: c.mode}, nil
}

func (d *displayDevice) setCRTC(fb uint32, mode modeInfo, connectors []uint32) error {
	return d.setCRTCAt(fb, mode, connectors, 0, 0)
}

func (d *displayDevice) setCRTCAt(fb uint32, mode modeInfo, connectors []uint32, x, y uint32) error {
	c := modeCRTC{crtcID: d.crtcID, fbID: fb, x: x, y: y, modeValid: 1, mode: mode}
	if fb == 0 {
		c.modeValid = 0
	}
	c.countConnectors = uint32(len(connectors))
	c.connectorPtr = slicePointer(connectors)
	err := ioctl.Do(d.file.Fd(), ioctl.Pointer(ioctl.Read|ioctl.Write, &c, drmModeSetCRTC), &c)
	runtime.KeepAlive(connectors)
	if err != nil {
		return fmt.Errorf("kms: set CRTC: %w", err)
	}
	return nil
}

func (d *displayDevice) createBuffer(b *scanoutBuffer, width, height uint32) error {
	create := dumbBuffer{width: width, height: height, bpp: 32}
	if err := ioctl.Do(d.file.Fd(), ioctl.Pointer(ioctl.Read|ioctl.Write, &create, drmModeCreateDumb), &create); err != nil {
		return fmt.Errorf("kms: create dumb buffer: %w", err)
	}
	b.handle, b.pitch, b.size = create.handle, create.pitch, create.size
	if b.size > uint64(^uint(0)>>1) {
		return errors.New("kms: scanout buffer is too large for this process")
	}
	add := framebufferAdd{width: width, height: height, pitch: create.pitch, bpp: 32, depth: 24, handle: create.handle}
	if err := ioctl.Do(d.file.Fd(), ioctl.Pointer(ioctl.Read|ioctl.Write, &add, drmModeAddFB), &add); err != nil {
		return fmt.Errorf("kms: add framebuffer: %w", err)
	}
	b.fbID = add.fbID
	mapReq := dumbMap{handle: b.handle}
	if err := ioctl.Do(d.file.Fd(), ioctl.Pointer(ioctl.Read|ioctl.Write, &mapReq, drmModeMapDumb), &mapReq); err != nil {
		return fmt.Errorf("kms: map dumb buffer: %w", err)
	}
	pix, err := syscall.Mmap(int(d.file.Fd()), int64(mapReq.offset), int(b.size), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		return fmt.Errorf("kms: mmap dumb buffer: %w", err)
	}
	b.pix = pix
	return nil
}

func (d *displayDevice) destroyBuffer(b *scanoutBuffer) error {
	var first error
	if len(b.pix) != 0 {
		if err := syscall.Munmap(b.pix); err != nil {
			first = err
		}
		b.pix = nil
	}
	if b.fbID != 0 {
		if err := ioctl.Do(d.file.Fd(), ioctl.Pointer(ioctl.Write, &b.fbID, drmModeRMFB), &b.fbID); err != nil && first == nil {
			first = err
		}
		b.fbID = 0
	}
	if b.handle != 0 {
		request := dumbDestroy{handle: b.handle}
		if err := ioctl.Do(d.file.Fd(), ioctl.Pointer(ioctl.Write, &request, drmModeDestroyDumb), &request); err != nil && first == nil {
			first = err
		}
		b.handle = 0
	}
	return first
}

func (d *displayDevice) Bounds() image.Rectangle     { return d.shadow.Bounds() }
func (d *displayDevice) ColorModel() color.Model     { return color.RGBAModel }
func (d *displayDevice) At(x, y int) color.Color     { return d.shadow.At(x, y) }
func (d *displayDevice) Set(x, y int, c color.Color) { d.shadow.Set(x, y, c) }

// CopyFrom replaces the software framebuffer from an image. This is useful for
// renderers that can provide a complete frame at once and avoids per-pixel
// interface calls when the source is an *image.RGBA.
func (d *displayDevice) CopyFrom(src image.Image) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return os.ErrClosed
	}
	if src == nil || src.Bounds().Size() != d.shadow.Bounds().Size() {
		return errors.New("kms: source image size does not match the display")
	}
	stdDraw.Draw(d.shadow, d.shadow.Bounds(), src, src.Bounds().Min, stdDraw.Src)
	return nil
}

// DrawBuffer returns the mutable RGBA framebuffer used by this display.
// Callers must finish drawing before calling Refresh and must not modify the
// buffer concurrently with Refresh or other display operations. The buffer is
// valid until Close.
func (d *displayDevice) DrawBuffer() *image.RGBA { return d.shadow }

func (d *displayDevice) String() string {
	return fmt.Sprintf("DRM/KMS %dx%d", d.shadow.Bounds().Dx(), d.shadow.Bounds().Dy())
}
func (d *displayDevice) Clear() { d.Fill(color.Black) }
func (d *displayDevice) Fill(c color.Color) {
	cc := color.RGBAModel.Convert(c).(color.RGBA)
	for i := 0; i < len(d.shadow.Pix); i += 4 {
		d.shadow.Pix[i], d.shadow.Pix[i+1], d.shadow.Pix[i+2], d.shadow.Pix[i+3] = cc.R, cc.G, cc.B, cc.A
	}
}

// Refresh page-flips to a completed back buffer and waits for the DRM vblank event.
func (d *displayDevice) Refresh() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return os.ErrClosed
	}
	if !d.active {
		return errors.New("kms: display is off")
	}
	back := &d.buffers[d.back]
	if err := buffer.CopyToXRGB(back.pix, int(back.pitch), d.shadow); err != nil {
		return err
	}
	flip := pageFlip{crtcID: d.crtcID, fbID: back.fbID, flags: drmModePageFlipEvent}
	if err := ioctl.Do(d.file.Fd(), ioctl.Pointer(ioctl.Read|ioctl.Write, &flip, drmModePageFlip), &flip); err != nil {
		return fmt.Errorf("kms: queue page flip: %w", err)
	}
	var event [32]byte
	for {
		n, err := d.file.Read(event[:])
		if err != nil {
			return fmt.Errorf("kms: wait for page flip: %w", err)
		}
		if n < 8 {
			return fmt.Errorf("kms: short DRM event (%d bytes)", n)
		}
		if binary.LittleEndian.Uint32(event[0:4]) == drmEventFlipComplete && n >= 32 {
			d.front, d.back = d.back, d.front
			return nil
		}
	}
}

func (d *displayDevice) Show(show bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return os.ErrClosed
	}
	if show == d.active {
		return nil
	}
	if show {
		if err := d.setCRTC(d.buffers[d.front].fbID, d.mode, []uint32{d.connectorID}); err != nil {
			return err
		}
	} else if err := d.setCRTC(0, modeInfo{}, nil); err != nil {
		return err
	}
	d.active = show
	return nil
}

func (d *displayDevice) SetContrast(uint8) error { return ErrNotSupported }
func (d *displayDevice) SetRotation(r display.Rotation) error {
	if r != display.NoRotation {
		return ErrNotSupported
	}
	return nil
}

func (d *displayDevice) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil
	}
	var first error
	if d.modeSet && d.master {
		if d.saved.modeValid != 0 && d.saved.fbID != 0 {
			if err := d.setCRTCAt(d.saved.fbID, d.saved.mode, d.saved.connectorIDs, d.saved.x, d.saved.y); err != nil {
				first = err
			}
		} else if d.active {
			if err := d.setCRTC(0, modeInfo{}, nil); err != nil {
				first = err
			}
		}
	}
	for i := range d.buffers {
		if err := d.destroyBuffer(&d.buffers[i]); err != nil && first == nil {
			first = err
		}
	}
	if d.master {
		if err := ioctl.Call(d.file.Fd(), drmIODropMaster, 0); err != nil && first == nil {
			first = err
		}
		d.master = false
	}
	if err := d.file.Close(); err != nil && first == nil {
		first = err
	}
	d.closed = true
	return first
}

func slicePointer[T any](s []T) uint64 {
	if len(s) == 0 {
		return 0
	}
	return uint64(uintptr(unsafe.Pointer(&s[0])))
}

type cardResources struct{ crtcIDs, connectorIDs []uint32 }

type modeResources struct {
	fbPtr, crtcPtr, connectorPtr, encoderPtr             uint64
	countFBs, countCRTCs, countConnectors, countEncoders uint32
	_, _, _, _                                           uint32
}

type connectorInfo struct {
	kernel   modeConnector
	modes    []modeInfo
	encoders []uint32
	name     string
}

type modeConnector struct {
	encodersPtr, modesPtr, propsPtr, propValuesPtr          uint64
	countModes, countProps, countEncoders, encoderID        uint32
	connectorID, connectorType, connectorTypeID, connection uint32
	_, _, _, _                                              uint32
}

type modeEncoder struct {
	encoderID, _, crtcID, possibleCRTCs, possibleClones uint32
}

type modeCRTC struct {
	connectorPtr                        uint64
	countConnectors, crtcID, fbID, x, y uint32
	_                                   uint32
	modeValid                           uint32
	mode                                modeInfo
}

type modeInfo struct {
	_          uint32
	hDisplay   uint16
	_, _, _, _ uint16
	vDisplay   uint16
	_          uint16
	vSyncEnd   uint16
	vTotal     uint16
	vScan      uint16
	_, _       uint32
	typ        uint32
	_          [32]byte
}

type dumbBuffer struct {
	height, width, bpp uint32
	_                  uint32
	handle, pitch      uint32
	size               uint64
}
type dumbMap struct {
	handle uint32
	_      uint32
	offset uint64
}
type dumbDestroy struct{ handle uint32 }
type framebufferAdd struct{ fbID, width, height, pitch, bpp, depth, handle uint32 }
type pageFlip struct {
	crtcID, fbID, flags uint32
	_                   uint32
	_                   uint64
}

func connectorName(typ, id uint32) string {
	name := map[uint32]string{
		drmModeConnectorVGA: "VGA", drmModeConnectorDVIi: "DVI-I",
		drmModeConnectorDVId: "DVI-D", drmModeConnectorDVIa: "DVI-A",
		drmModeConnectorComposite: "Composite", drmModeConnectorSVideo: "SVIDEO",
		drmModeConnectorLVDS: "LVDS", drmModeConnectorComponent: "Component",
		drmModeConnectorDIN: "DIN", drmModeConnectorDisplayPort: "DP",
		drmModeConnectorHDMIA: "HDMI-A", drmModeConnectorHDMIB: "HDMI-B",
		drmModeConnectorTV: "TV", drmModeConnectorEDP: "eDP",
		drmModeConnectorVirtual: "Virtual", drmModeConnectorDSI: "DSI",
		drmModeConnectorDPI: "DPI", drmModeConnectorWriteback: "Writeback",
		drmModeConnectorSPI: "SPI", drmModeConnectorUSB: "USB",
	}[typ]
	if name == "" {
		name = fmt.Sprintf("Connector-%d", typ)
	}
	return fmt.Sprintf("%s-%d", name, id)
}
