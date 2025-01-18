package pixel

import "image/color"

// Models for the standard color types.
var (
	MonoModel   color.Model = color.ModelFunc(monoModel)
	Gray2Model  color.Model = color.ModelFunc(gray2Model)
	Gray4Model  color.Model = color.ModelFunc(gray4Model)
	CRGB15Model color.Model = color.ModelFunc(crgb15Model)
	CRGB16Model color.Model = color.ModelFunc(crgb16Model)
)

var (
	Off = Mono{false}
	On  = Mono{true}
)

// Mono represents a 1-bit monochrome color.
type Mono struct {
	On bool
}

func (c Mono) RGBA() (r, g, b, a uint32) {
	if c.On {
		return 0xffff, 0xffff, 0xffff, 0xffff
	}
	return 0, 0, 0, 0xffff
}

func monoModel(c color.Color) color.Color {
	if _, ok := c.(Mono); ok {
		return c
	}
	r, g, b, _ := c.RGBA()

	// These coefficients (the fractions 0.299, 0.587 and 0.114) are the same
	// as those given by the JFIF specification and used by func RGBToYCbCr in
	// ycbcr.go.
	//
	// Note that 19595 + 38470 + 7471 equals 65536.
	//
	// The 31 is 16 + 15. The 16 is the same as used in RGBToYCbCr. The 15 is
	// because the return value is 1 bit color, not 16 bit color.
	y := (19595*r + 38470*g + 7471*b + 1<<15) >> 31

	return Mono{On: y != 0}
}

// Gray2 represents a 2-bit grayscale color.
type Gray2 struct {
	Y uint8
}

func (c Gray2) RGBA() (r, g, b, a uint32) {
	y := uint32(c.Y) >> 6
	y |= y << 2
	y |= y << 4
	y |= y << 8
	return y, y, y, 0xffff
}

func gray2Model(c color.Color) color.Color {
	if _, ok := c.(Gray2); ok {
		return c
	}
	r, g, b, _ := c.RGBA()
	y := (299*r + 587*g + 114*b + 500) / 1000
	y >>= 6
	y |= y << 2
	y |= y << 4
	y |= y << 8
	return color.Gray16{uint16(y)}
}

// Gray4 represents a 4-bit grayscale color.
type Gray4 struct {
	Y uint8
}

func (c Gray4) RGBA() (r, g, b, a uint32) {
	y := uint32(c.Y) >> 4
	y |= y << 4
	y |= y << 8
	return y, y, y, 0xffff
}

func gray4Model(c color.Color) color.Color {
	if _, ok := c.(Gray4); ok {
		return c
	}
	r, g, b, _ := c.RGBA()
	y := (299*r + 587*g + 114*b + 500) / 1000
	y >>= 4
	return Gray4{Y: uint8(y & 0xf)}
}

// CRGB15 represents a 15-bit 5-5-5 RGB color.
type CRGB15 struct {
	// CIgnore, 1, CRed, 5, CGreen, 5, CBlue, 5
	V uint16
}

func (c CRGB15) RGBA() (r, g, b, a uint32) {
	// Build a 5-bit value at the top of the low byte of each component.
	red := (c.V & 0x7C00) >> 7
	grn := (c.V & 0x03E0) >> 2
	blu := (c.V & 0x001F) << 3
	// Duplicate the high bits in the low bits.
	red |= red >> 5
	grn |= grn >> 5
	blu |= blu >> 5
	// Duplicate the whole value in the high byte.
	red |= red << 8
	grn |= grn << 8
	blu |= blu << 8
	return uint32(red), uint32(grn), uint32(blu), 0xffff
}

func crgb15Model(c color.Color) color.Color {
	if _, ok := c.(CRGB15); ok {
		return c
	}
	r, g, b, _ := c.RGBA()
	r = (r & 0xF800) >> 1
	g = (g & 0xF800) >> 6
	b = (b & 0xF800) >> 11
	return CRGB15{uint16(r | g | b)}
}

// CRGB16 represents a 16-bit 5-6-5 RGB color.
type CRGB16 struct {
	// CRed, 5, CGreen, 6, CBlue, 5
	V uint16
}

func (c CRGB16) RGBA() (r, g, b, a uint32) {
	// Build a 5- or 6-bit value at the top of the low byte of each component.
	red := (c.V & 0xF800) >> 8
	grn := (c.V & 0x07E0) >> 3
	blu := (c.V & 0x001F) << 3
	// Duplicate the high bits in the low bits.
	red |= red >> 5
	grn |= grn >> 6
	blu |= blu >> 5
	// Duplicate the whole value in the high byte.
	red |= red << 8
	grn |= grn << 8
	blu |= blu << 8
	return uint32(red), uint32(grn), uint32(blu), 0xffff
}

func crgb16Model(c color.Color) color.Color {
	switch c := c.(type) {
	case Mono:
		if c.On {
			return CRGB16{0xffff}
		}
		return CRGB16{}
	case Gray4:
		y := uint16(c.Y<<1 | c.Y)
		r := y
		g := y >> 5
		b := y >> 11
		return CRGB16{r | g | b}
	case CRGB16:
		return c
	default:
		r, g, b, _ := c.RGBA()
		r = (r & 0xF800)
		g = (g & 0xFC00) >> 5
		b = (b & 0xF800) >> 11
		return CRGB16{uint16(r | g | b)}
	}
}
