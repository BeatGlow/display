package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"strings"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/physic"

	"github.com/BeatGlow/display/v2"
	"github.com/BeatGlow/display/v2/draw"
	"github.com/BeatGlow/display/v2/pixel"
)

// recordingConn lets real display drivers initialize and refresh without
// physical hardware. It records traffic totals while the driver's framebuffer
// remains available for a PNG preview.
type recordingConn struct {
	commands  int
	dataBytes int
	events    []busEvent
}

type busEvent struct {
	Type  string `json:"type"`
	Bytes []byte `json:"bytes"`
}

func (c *recordingConn) VerifyInitialization(driver string) error {
	config, ok := offlineDriverConfigs[strings.ToLower(driver)]
	if !ok || len(config.initialization) == 0 {
		return nil
	}
	expected := make([][]byte, len(config.initialization))
	for i, command := range config.initialization {
		expected[i] = append([]byte(nil), command...)
		if config.reverseInitBits {
			for j := range expected[i] {
				expected[i][j] = reverseBits(expected[i][j])
			}
		}
	}
	index := 0
	for _, event := range c.events {
		if event.Type == "command" && bytes.HasPrefix(event.Bytes, expected[index]) {
			index++
			if index == len(expected) {
				return nil
			}
		}
	}
	return fmt.Errorf("%s initialization is missing expected command % x", driver, expected[index])
}

func (c *recordingConn) String() string { return "offline recording connection" }
func (c *recordingConn) Close() error   { return nil }
func (c *recordingConn) Reset(gpio.Level) error {
	return nil
}
func (c *recordingConn) Command(command byte, args ...byte) error {
	c.commands++
	c.dataBytes += 1 + len(args)
	c.events = append(c.events, busEvent{Type: "command", Bytes: append([]byte{command}, args...)})
	return nil
}
func (c *recordingConn) Data(data ...byte) error {
	c.dataBytes += len(data)
	c.events = append(c.events, busEvent{Type: "data", Bytes: append([]byte(nil), data...)})
	return nil
}
func (c *recordingConn) Interface() any { return c }

// VerifyFrame checks that Refresh emitted the framebuffer in the format the
// selected driver is expected to send. Controller command bytes stay in the
// event stream so this also catches missing or truncated transfers.
func (c *recordingConn) VerifyFrame(driver string, frame image.Image, from int) (image.Image, error) {
	driver = strings.ToLower(driver)
	config := offlineDriverConfigs[driver]
	want, err := encodeFrame(driver, frame)
	if err != nil {
		return nil, err
	}
	events := c.events[from:]
	if config.pageCommand != nil {
		pages := (frame.Bounds().Dy() + 7) / 8
		page := 0
		for _, event := range events {
			if event.Type != "command" || page == pages {
				continue
			}
			expected := config.pageCommand(page)
			if bytes.Equal(event.Bytes, expected) {
				page++
			}
		}
		if page != pages {
			return nil, fmt.Errorf("%s refresh sent %d valid page commands; expected %d", driver, page, pages)
		}
	}
	var captured []byte
	if config.frameInCommand {
		prefix := config.frameCommandPrefix(frame)
		for _, event := range events {
			if event.Type == "command" && bytes.HasPrefix(event.Bytes, prefix) && bytes.HasSuffix(event.Bytes, want) {
				captured = event.Bytes[len(prefix):]
			}
		}
	} else {
		var payload []byte
		for _, event := range events {
			if event.Type == "data" {
				payload = append(payload, event.Bytes...)
			}
		}
		if config.frameDataPrefix != nil {
			prefix := config.frameDataPrefix(frame)
			if !bytes.HasPrefix(payload, prefix) {
				return nil, fmt.Errorf("%s refresh has invalid data prefix: got % x, want % x", driver, payload[:min(len(payload), len(prefix))], prefix)
			}
			payload = payload[len(prefix):]
		}
		if len(payload) != len(want)+config.frameDataExtra {
			return nil, fmt.Errorf("%s refresh sent %d data bytes; expected %d pixel bytes plus %d framing bytes", driver, len(payload), len(want), config.frameDataExtra)
		}
		if index := bytes.LastIndex(payload, want); index >= 0 {
			captured = payload[index : index+len(want)]
		}
	}
	if captured == nil {
		return nil, fmt.Errorf("%s refresh traffic does not contain the expected %d-byte framebuffer (expected % x)", driver, len(want), want[:min(8, len(want))])
	}
	decoded, err := decodeFrame(driver, frame, captured)
	if err != nil {
		return nil, err
	}
	if !sameImage(frame, decoded) {
		return nil, fmt.Errorf("%s emulated display memory differs from the requested framebuffer", driver)
	}
	return decoded, nil
}

func sameImage(want, got image.Image) bool {
	if want.Bounds().Size() != got.Bounds().Size() {
		return false
	}
	for y := 0; y < want.Bounds().Dy(); y++ {
		for x := 0; x < want.Bounds().Dx(); x++ {
			wr, wg, wb, wa := want.At(want.Bounds().Min.X+x, want.Bounds().Min.Y+y).RGBA()
			gr, gg, gb, ga := got.At(x, y).RGBA()
			if wr != gr || wg != gg || wb != gb || wa != ga {
				return false
			}
		}
	}
	return true
}

func decodeFrame(driver string, frame image.Image, encoded []byte) (*image.RGBA, error) {
	bounds := frame.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	decoded := image.NewRGBA(image.Rect(0, 0, w, h))
	config := offlineDriverConfigs[strings.ToLower(driver)]
	kind := frame.At(bounds.Min.X, bounds.Min.Y)
	if _, ok := kind.(pixel.Mono); ok {
		data := append([]byte(nil), encoded...)
		if config.reverseFrameBits {
			for i := range data {
				data[i] = reverseBits(data[i])
			}
		}
		pages := (h + 7) / 8
		for y := range h {
			for x := range w {
				index := (y/8)*w + x
				switch config.monoLayout {
				case monoColumnMajor:
					index = x*pages + y/8
				case monoPadded256:
					index = (y/8)*256 + x
				}
				value := uint8(0)
				if data[index]&(1<<uint(y&7)) != 0 {
					value = 0xff
				}
				decoded.SetRGBA(x, y, color.RGBA{R: value, G: value, B: value, A: 0xff})
			}
		}
		return decoded, nil
	}
	if _, ok := kind.(pixel.Gray2); ok {
		stride := (w + 3) / 4
		for y := range h {
			for x := range w {
				v := (encoded[y*stride+x/4] >> uint((3-x%4)*2)) & 3
				gray := v * 0x55
				decoded.SetRGBA(x, y, color.RGBA{R: gray, G: gray, B: gray, A: 0xff})
			}
		}
		return decoded, nil
	}
	if _, ok := kind.(pixel.Gray4); ok {
		stride := (w + 1) / 2
		for y := range h {
			for x := range w {
				v := encoded[y*stride+x/2]
				if x&1 == 0 {
					v >>= 4
				} else {
					v &= 0xf
				}
				gray := (v & 0xf) * 0x11
				decoded.SetRGBA(x, y, color.RGBA{R: gray, G: gray, B: gray, A: 0xff})
			}
		}
		return decoded, nil
	}
	for y := range h {
		for x := range w {
			i := (y*w + x) * 2
			v := uint16(encoded[i])<<8 | uint16(encoded[i+1])
			var r, g, b, a uint32
			if _, ok := kind.(pixel.CRGB15); ok {
				r, g, b, a = pixel.CRGB15{V: v}.RGBA()
			} else {
				r, g, b, a = pixel.CRGB16{V: v}.RGBA()
			}
			decoded.SetRGBA(x, y, color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)})
		}
	}
	return decoded, nil
}

func (c *recordingConn) VerifyContrast(driver string, from int, level uint8, pin *offlinePin) (bool, error) {
	config := offlineDriverConfigs[strings.ToLower(driver)].contrast
	if config.none {
		return false, nil
	}
	if config.pwm {
		expected := gpio.DutyMax / 0xff * gpio.Duty(level)
		if pin != nil && pin.pwmSet && pin.duty == expected {
			return true, nil
		}
		return true, fmt.Errorf("%s contrast did not set the expected backlight PWM duty", driver)
	}
	if config.data == nil {
		return false, nil
	}
	want := config.data(level)
	for _, event := range c.events[from:] {
		if event.Type == config.kind && bytes.Contains(event.Bytes, want) {
			return true, nil
		}
	}
	return true, fmt.Errorf("%s contrast call did not emit expected %s bytes % x", driver, config.kind, want)
}

func encodeFrame(driver string, frame image.Image) ([]byte, error) {
	bounds := frame.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	config := offlineDriverConfigs[strings.ToLower(driver)]
	var out []byte
	switch frame.At(bounds.Min.X, bounds.Min.Y).(type) {
	case pixel.Mono:
		pages := (height + 7) / 8
		for page := range pages {
			pageWidth := width
			if config.monoLayout == monoPadded256 {
				pageWidth = 256
			}
			for x := range pageWidth {
				if x < width {
					out = append(out, monoByte(frame, x, page*8))
				} else {
					out = append(out, 0)
				}
			}
		}
		if config.monoLayout == monoColumnMajor {
			out = out[:0]
			for x := range width {
				for page := range pages {
					out = append(out, monoByte(frame, x, page*8))
				}
			}
		}
	case pixel.Gray2:
		stride := (width + 3) / 4
		out = make([]byte, stride*height)
		for y := range height {
			for x := range width {
				v := pixel.Gray2Model.Convert(frame.At(bounds.Min.X+x, bounds.Min.Y+y)).(pixel.Gray2).Y & 3
				out[y*stride+x/4] |= v << uint((3-x%4)*2)
			}
		}
	case pixel.Gray4:
		stride := (width + 1) / 2
		out = make([]byte, stride*height)
		for y := range height {
			for x := range width {
				v := pixel.Gray4Model.Convert(frame.At(bounds.Min.X+x, bounds.Min.Y+y)).(pixel.Gray4).Y & 0xf
				if x&1 == 0 {
					out[y*stride+x/2] |= v << 4
				} else {
					out[y*stride+x/2] |= v
				}
			}
		}
	case pixel.CRGB15:
		for y := range height {
			for x := range width {
				v := pixel.CRGB15Model.Convert(frame.At(bounds.Min.X+x, bounds.Min.Y+y)).(pixel.CRGB15).V
				out = append(out, byte(v>>8), byte(v))
			}
		}
	case pixel.CRGB16:
		for y := range height {
			for x := range width {
				v := pixel.CRGB16Model.Convert(frame.At(bounds.Min.X+x, bounds.Min.Y+y)).(pixel.CRGB16).V
				out = append(out, byte(v>>8), byte(v))
			}
		}
	default:
		return nil, fmt.Errorf("no offline framebuffer encoder for color model %T", frame.ColorModel())
	}
	if config.reverseFrameBits {
		for i := range out {
			out[i] = reverseBits(out[i])
		}
	}
	return out, nil
}

func monoByte(frame image.Image, x, y int) byte {
	var b byte
	for bit := 0; bit < 8 && y+bit < frame.Bounds().Max.Y; bit++ {
		if pixel.MonoModel.Convert(frame.At(x, y+bit)).(pixel.Mono).On {
			b |= 1 << uint(bit)
		}
	}
	return b
}

func reverseBits(b byte) byte {
	b = (b&0x55)<<1 | (b&0xaa)>>1
	b = (b&0x33)<<2 | (b&0xcc)>>2
	return b<<4 | b>>4
}

type brightnessPreview struct {
	image.Image
	scale uint8
}

func atBrightness(src image.Image, driver string, level uint8) image.Image {
	scale := level
	if fn := offlineDriverConfigs[strings.ToLower(driver)].brightnessScale; fn != nil {
		scale = fn(level)
	}
	return brightnessPreview{Image: src, scale: scale}
}

func (p brightnessPreview) At(x, y int) color.Color {
	r, g, b, a := color.RGBAModel.Convert(p.Image.At(x, y)).RGBA()
	return color.RGBA{
		R: uint8((r * uint32(p.scale)) / 0xffff),
		G: uint8((g * uint32(p.scale)) / 0xffff),
		B: uint8((b * uint32(p.scale)) / 0xffff),
		A: uint8(a >> 8),
	}
}

type offlinePin struct {
	duty   gpio.Duty
	pwmSet bool
}

func (*offlinePin) String() string       { return "offline GPIO" }
func (*offlinePin) Halt() error          { return nil }
func (*offlinePin) Name() string         { return "offline" }
func (*offlinePin) Number() int          { return -1 }
func (*offlinePin) Function() string     { return "out" }
func (*offlinePin) Out(gpio.Level) error { return nil }
func (p *offlinePin) PWM(duty gpio.Duty, _ physic.Frequency) error {
	p.duty, p.pwmSet = duty, true
	return nil
}

var _ gpio.PinOut = (*offlinePin)(nil)

type virtualDisplay struct {
	draw.Image
	model color.Model
}

func newVirtualDisplay(width, height int, modelName string) (display.Display, error) {
	if width == 0 {
		width = 128
	}
	if height == 0 {
		height = 64
	}
	if width < 1 || height < 1 {
		return nil, fmt.Errorf("offline display dimensions must be positive")
	}

	var (
		img   draw.Image
		model color.Model
	)
	switch modelName {
	case "mono":
		img, model = pixel.NewMonoImage(width, height), pixel.MonoModel
	case "gray2":
		img, model = pixel.NewGray2Image(width, height), pixel.Gray2Model
	case "gray4":
		img, model = pixel.NewGray4Image(width, height), pixel.Gray4Model
	case "rgb15":
		img, model = pixel.NewCRGB15Image(width, height), pixel.CRGB15Model
	case "rgb16":
		img, model = pixel.NewCRGB16Image(width, height), pixel.CRGB16Model
	default:
		return nil, fmt.Errorf("unsupported offline color model %q (choose mono, gray2, gray4, rgb15, or rgb16)", modelName)
	}
	return &virtualDisplay{Image: img, model: model}, nil
}

func (d *virtualDisplay) String() string {
	return fmt.Sprintf("offline virtual display %dx%d", d.Bounds().Dx(), d.Bounds().Dy())
}

func (d *virtualDisplay) ColorModel() color.Model            { return d.model }
func (d *virtualDisplay) Close() error                       { return nil }
func (d *virtualDisplay) Show(bool) error                    { return nil }
func (d *virtualDisplay) SetContrast(uint8) error            { return nil }
func (d *virtualDisplay) SetRotation(display.Rotation) error { return nil }
func (d *virtualDisplay) Refresh() error                     { return nil }

func (d *virtualDisplay) Clear() {
	d.Fill(color.Black)
}

func (d *virtualDisplay) Fill(c color.Color) {
	bounds := d.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			d.Set(x, y, c)
		}
	}
}

var _ display.Display = (*virtualDisplay)(nil)
