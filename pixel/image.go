package pixel

import (
	"encoding/binary"
	"image"
	"image/color"

	"github.com/BeatGlow/display/draw"
)

type Image interface {
	draw.Image

	// Clear the image.
	Clear()

	// Fill the image with a single color.
	Fill(color.Color)
}

// Buffer holds the pixel values and is a container that is used by most image formats in this package.
type Buffer struct {
	// Rect is the image bounding box.
	Rect image.Rectangle

	// Pix are the image pixels.
	Pix []byte

	// Stride is the Pix stride (in bytes) between vertically adjacent pixels.
	Stride int
}

func (p *Buffer) Bounds() image.Rectangle {
	return p.Rect
}

func (p *Buffer) Clear() {
	for i := range p.Pix {
		p.Pix[i] = 0x00
	}
}

func makeBuffer(w, h, stride, size int) Buffer {
	return Buffer{
		Rect:   image.Rect(0, 0, w, h),
		Pix:    make([]byte, size),
		Stride: stride,
	}
}

// MonoImage is a 1-bit per pixel monochrome image.
type MonoImage struct {
	Buffer
}

func NewMonoImage(w, h int) *MonoImage {
	stride := ((w + 7) & ^7) / 8 // round up to whole bytes
	return &MonoImage{
		Buffer: makeBuffer(w, h, stride, stride*h),
	}
}

func (p *MonoImage) ColorModel() color.Model {
	return MonoModel
}

func (p *MonoImage) PixOffset(x, y int) int {
	return y*p.Stride + x/8
}

func (p *MonoImage) At(x, y int) color.Color {
	if !(image.Point{x, y}).In(p.Rect) {
		return color.Transparent
	}

	index := y*p.Stride + x/8
	pixel := p.Pix[index] & (1 << uint(x%8))

	if pixel != 0 {
		return On
	}
	return Off
}

func (p *MonoImage) Set(x, y int, c color.Color) {
	if !(image.Point{x, y}).In(p.Rect) {
		return
	}

	index := y*p.Stride + x/8
	color := monoModel(c).(Mono)

	if color.On {
		p.Pix[index] |= (1 << uint(x%8))
	} else {
		p.Pix[index] &^= (1 << uint(x%8))
	}
}

func (p *MonoImage) Fill(c color.Color) {
	var value byte
	if monoModel(c).(Mono).On {
		value = 0xff
	}
	for i := range p.Pix {
		p.Pix[i] = value
	}
}

// MonoVerticalLSBImage is a 1-bit per pixel monochrome image.
//
// This is mostly used by SSD1xxx OLED displays.
type MonoVerticalLSBImage struct {
	Buffer
}

func NewMonoVerticalLSBImage(w, h int) *MonoVerticalLSBImage {
	bands := ((h + 7) & ^7) / 8 // round up to whole bytes
	return &MonoVerticalLSBImage{
		Buffer: makeBuffer(w, h, w, bands*w),
	}
}

func (p *MonoVerticalLSBImage) ColorModel() color.Model {
	return MonoModel
}

func (p *MonoVerticalLSBImage) At(x, y int) color.Color {
	if !(image.Point{X: x, Y: y}).In(p.Rect) {
		return color.Transparent
	}

	var (
		pos = y/8*p.Stride + x
		bit = byte(1) << uint(y&7)
	)
	return Mono{
		On: p.Pix[pos]&bit != 0,
	}
}

func (p *MonoVerticalLSBImage) Set(x, y int, c color.Color) {
	if !(image.Point{X: x, Y: y}).In(p.Rect) {
		return
	}

	var (
		pos = y/8*p.Stride + x
		bit = byte(1) << uint(y&7)
	)
	if monoModel(c).(Mono).On {
		p.Pix[pos] |= bit
	} else {
		p.Pix[pos] &^= bit
	}
}

func (p *MonoVerticalLSBImage) Fill(c color.Color) {
	var value byte
	if monoModel(c).(Mono).On {
		value = 0xff
	}
	for i := range p.Pix {
		p.Pix[i] = value
	}
}

// Gray2Image is a 2-bits per pixel gray scale image.
type Gray2Image struct {
	Buffer
}

func NewGray2Image(w, h int) *Gray2Image {
	return &Gray2Image{
		Buffer: makeBuffer(w, h, (w+3)>>2, h*(w+3)>>2),
	}
}

func (p *Gray2Image) ColorModel() color.Model {
	return Gray2Model
}

func (p *Gray2Image) At(x, y int) color.Color {
	if !(image.Point{x, y}).In(p.Rect) {
		return color.Transparent
	}

	index := y*p.Stride + x/4
	shift := (3 - x&3) << 1
	return Gray2{Y: (p.Pix[index] >> shift) & 3}
}

func (p *Gray2Image) Set(x, y int, c color.Color) {
	if !(image.Point{x, y}).In(p.Rect) {
		return
	}

	index := y*p.Stride + x/4
	shift := (3 - x&3) << 1
	color := gray2Model(c).(Gray2).Y & 3
	p.Pix[index] = (p.Pix[index] &^ (3 << shift)) | color<<shift
}

func (p *Gray2Image) Fill(c color.Color) {
	value := gray2Model(c).(Gray2).Y & 0x3
	value |= value << 2
	value |= value << 4
	for i := range p.Pix {
		p.Pix[i] = value
	}
}

// Gray4Image is a 4-bits per pixel gray scale image.
type Gray4Image struct {
	Buffer
}

func NewGray4Image(w, h int) *Gray4Image {
	return &Gray4Image{
		Buffer: makeBuffer(w, h, (w+1)/2, h*((w+1)/2)),
	}
}

func (p *Gray4Image) ColorModel() color.Model {
	return Gray4Model
}

func (p *Gray4Image) At(x, y int) color.Color {
	if !(image.Point{x, y}).In(p.Rect) {
		return color.Transparent
	}

	index := y*p.Stride + x>>1
	if x%2 == 0 {
		return Gray4{Y: p.Pix[index] >> 4}
	} else {
		return Gray4{Y: p.Pix[index] & 0xf}
	}
}

func (p *Gray4Image) Set(x, y int, c color.Color) {
	if !(image.Point{x, y}).In(p.Rect) {
		return
	}

	index := y*p.Stride + x>>1
	color := gray4Model(c).(Gray4).Y & 0xf
	if x%2 == 0 {
		p.Pix[index] = (p.Pix[index] & 0x0f) | color<<4
	} else {
		p.Pix[index] = (p.Pix[index] & 0xf0) | color
	}
}

func (p *Gray4Image) Fill(c color.Color) {
	value := gray4Model(c).(Gray4).Y & 0xf
	value |= value << 4
	for i := range p.Pix {
		p.Pix[i] = value
	}
}

// CBGR15Image is a 15-bits per pixel 5-5-5-bit BGR image.
type CBGR15Image struct {
	Buffer
	Order binary.ByteOrder
}

func NewCBGR15Image(w, h int) *CBGR15Image {
	return &CBGR15Image{
		Buffer: makeBuffer(w, h, w*2, w*2*h),
		Order:  binary.BigEndian,
	}
}

func (p *CBGR15Image) ColorModel() color.Model {
	return CBGR15Model
}

func (p *CBGR15Image) At(x, y int) color.Color {
	if !(image.Point{X: x, Y: y}).In(p.Rect) {
		return color.Transparent
	}

	v := p.Order.Uint16(p.Pix[x*2+y*p.Stride:])
	return CBGR15{v & 0x7fff}
}

func (p *CBGR15Image) Set(x, y int, c color.Color) {
	if !(image.Point{X: x, Y: y}).In(p.Rect) {
		return
	}

	v := cbgr15Model(c).(CBGR15).V
	p.Order.PutUint16(p.Pix[x*2+y*p.Stride:], v)
}

func (p *CBGR15Image) Fill(c color.Color) {
	value := cbgr15Model(c).(CBGR15).V
	bytes := make([]byte, 2)
	p.Order.PutUint16(bytes, value)
	for i, l := 0, len(p.Pix); i < l; i += 2 {
		copy(p.Pix[i:], bytes)
	}
}

// CBGR16Image is a 16-bits per pixel 5-6-5-bit BGR image.
type CBGR16Image struct {
	Buffer
	Order binary.ByteOrder
}

func NewCBGR16Image(w, h int) *CBGR16Image {
	return &CBGR16Image{
		Buffer: makeBuffer(w, h, w*2, w*2*h),
		Order:  binary.BigEndian,
	}
}

func (p *CBGR16Image) ColorModel() color.Model {
	return CBGR16Model
}

func (p *CBGR16Image) At(x, y int) color.Color {
	if !(image.Point{X: x, Y: y}).In(p.Rect) {
		return color.Transparent
	}

	v := p.Order.Uint16(p.Pix[x*2+y*p.Stride:])
	return CBGR16{v}
}

func (p *CBGR16Image) Set(x, y int, c color.Color) {
	if !(image.Point{X: x, Y: y}).In(p.Rect) {
		return
	}

	v := cbgr16Model(c).(CBGR16).V
	p.Order.PutUint16(p.Pix[x*2+y*p.Stride:], v)
}

func (p *CBGR16Image) Fill(c color.Color) {
	value := cbgr16Model(c).(CBGR16).V
	bytes := make([]byte, 2)
	p.Order.PutUint16(bytes, value)
	for i, l := 0, len(p.Pix); i < l; i += 2 {
		copy(p.Pix[i:], bytes)
	}
}

// CRGB15Image is a 15-bits per pixel 5-5-5-bit RGB image.
type CRGB15Image struct {
	Buffer
	Order binary.ByteOrder
}

func NewCRGB15Image(w, h int) *CRGB15Image {
	return &CRGB15Image{
		Buffer: makeBuffer(w, h, w*2, w*2*h),
		Order:  binary.BigEndian,
	}
}

func (p *CRGB15Image) ColorModel() color.Model {
	return CRGB15Model
}

func (p *CRGB15Image) At(x, y int) color.Color {
	if !(image.Point{X: x, Y: y}).In(p.Rect) {
		return color.Transparent
	}

	v := p.Order.Uint16(p.Pix[x*2+y*p.Stride:])
	return CRGB15{v & 0x7fff}
}

func (p *CRGB15Image) Set(x, y int, c color.Color) {
	if !(image.Point{X: x, Y: y}).In(p.Rect) {
		return
	}

	v := crgb15Model(c).(CRGB15).V
	p.Order.PutUint16(p.Pix[x*2+y*p.Stride:], v)
}

func (p *CRGB15Image) Fill(c color.Color) {
	value := crgb15Model(c).(CRGB15).V
	bytes := make([]byte, 2)
	p.Order.PutUint16(bytes, value)
	for i, l := 0, len(p.Pix); i < l; i += 2 {
		copy(p.Pix[i:], bytes)
	}
}

// CRGB16Image is a 16-bits per pixel 5-6-5-bit RGB image.
type CRGB16Image struct {
	Buffer
	Order binary.ByteOrder
}

func NewCRGB16Image(w, h int) *CRGB16Image {
	return &CRGB16Image{
		Buffer: makeBuffer(w, h, w*2, w*2*h),
		Order:  binary.BigEndian,
	}
}

func (p *CRGB16Image) ColorModel() color.Model {
	return CRGB16Model
}

func (p *CRGB16Image) At(x, y int) color.Color {
	if !(image.Point{X: x, Y: y}).In(p.Rect) {
		return color.Transparent
	}

	v := p.Order.Uint16(p.Pix[x*2+y*p.Stride:])
	return CRGB16{v}
}

func (p *CRGB16Image) Set(x, y int, c color.Color) {
	if !(image.Point{X: x, Y: y}).In(p.Rect) {
		return
	}

	v := crgb16Model(c).(CRGB16).V
	p.Order.PutUint16(p.Pix[x*2+y*p.Stride:], v)
}

func (p *CRGB16Image) Fill(c color.Color) {
	value := crgb16Model(c).(CRGB16).V
	bytes := make([]byte, 2)
	p.Order.PutUint16(bytes, value)
	for i, l := 0, len(p.Pix); i < l; i += 2 {
		copy(p.Pix[i:], bytes)
	}
}

// Interface checks.
var (
	_ Image = (*MonoImage)(nil)
	_ Image = (*MonoVerticalLSBImage)(nil)
	_ Image = (*Gray2Image)(nil)
	_ Image = (*Gray4Image)(nil)
	_ Image = (*CBGR15Image)(nil)
	_ Image = (*CBGR16Image)(nil)
	_ Image = (*CRGB15Image)(nil)
	_ Image = (*CRGB16Image)(nil)
)
