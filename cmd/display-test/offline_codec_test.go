package main

import (
	"bytes"
	"image"
	"image/color"
	"reflect"
	"testing"

	"periph.io/x/conn/v3/gpio"

	"github.com/BeatGlow/display/pixel"
)

func TestOfflineFrameCodecsRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		driver string
		frame  image.Image
	}{
		{"mono-page-major", "ssd1306", codecMonoFrame()},
		{"mono-column-major-reversed", "gp1294", codecMonoFrame()},
		{"mono-padded-reversed", "gp1247ai", codecMonoFrame()},
		{"mono-data-prefix", "gu256x64d7000", codecMonoFrame()},
		{"mono-command-prefix", "gu3000", codecMonoFrame()},
		{"gray2", "ssd1306", codecGray2Frame()},
		{"gray4", "sh1122", codecGray4Frame()},
		{"rgb15", "st7735", codecRGB15Frame()},
		{"rgb16", "st7789", codecRGB16Frame()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := encodeFrame(tt.driver, tt.frame)
			if err != nil {
				t.Fatalf("encodeFrame: %v", err)
			}
			decoded, err := decodeFrame(tt.driver, tt.frame, encoded)
			if err != nil {
				t.Fatalf("decodeFrame: %v", err)
			}
			if !sameImage(tt.frame, decoded) {
				t.Fatal("decoded frame differs from source")
			}

			conn := &recordingConn{}
			config := offlineDriverConfigs[tt.driver]
			for page := 0; config.pageCommand != nil && page < (tt.frame.Bounds().Dy()+7)/8; page++ {
				conn.events = append(conn.events, busEvent{Type: "command", Bytes: config.pageCommand(page)})
			}
			if config.frameInCommand {
				conn.events = append(conn.events, busEvent{Type: "command", Bytes: append(config.frameCommandPrefix(tt.frame), encoded...)})
			} else {
				payload := append([]byte(nil), encoded...)
				if config.frameDataPrefix != nil {
					payload = append(append([]byte(nil), config.frameDataPrefix(tt.frame)...), payload...)
				}
				payload = append(payload, make([]byte, config.frameDataExtra)...)
				conn.events = append(conn.events, busEvent{Type: "data", Bytes: payload})
			}
			if _, err := conn.VerifyFrame(tt.driver, tt.frame, 0); err != nil {
				t.Fatalf("VerifyFrame: %v", err)
			}
		})
	}
}

func TestOfflineInitializationAndFrameErrors(t *testing.T) {
	config := offlineDriverConfigs["gp1247ai"]
	conn := &recordingConn{}
	for _, expected := range config.initialization {
		command := append([]byte(nil), expected...)
		for i := range command {
			command[i] = reverseBits(command[i])
		}
		conn.events = append(conn.events, busEvent{Type: "command", Bytes: command})
	}
	if err := conn.VerifyInitialization("GP1247AI"); err != nil {
		t.Fatalf("VerifyInitialization: %v", err)
	}
	if err := (&recordingConn{}).VerifyInitialization("unknown driver"); err != nil {
		t.Fatalf("unknown driver initialization: %v", err)
	}
	if err := (&recordingConn{}).VerifyInitialization("gp1247ai"); err == nil {
		t.Fatal("VerifyInitialization accepted missing commands")
	}

	frame := codecMonoFrame()
	encoded, err := encodeFrame("gp1247ai", frame)
	if err != nil {
		t.Fatal(err)
	}
	missingPages := &recordingConn{events: []busEvent{{Type: "data", Bytes: encoded}}}
	if _, err := missingPages.VerifyFrame("gp1247ai", frame, 0); err == nil {
		t.Fatal("VerifyFrame accepted missing page commands")
	}
	badPrefix := &recordingConn{events: []busEvent{{Type: "data", Bytes: encoded}}}
	if _, err := badPrefix.VerifyFrame("gu256x64d7000", codecMonoFrame(), 0); err == nil {
		t.Fatal("VerifyFrame accepted missing data prefix")
	}
	short := &recordingConn{events: []busEvent{{Type: "data", Bytes: []byte{1}}}}
	if _, err := short.VerifyFrame("ssd1306", codecMonoFrame(), 0); err == nil {
		t.Fatal("VerifyFrame accepted truncated pixel payload")
	}
	if _, err := encodeFrame("ssd1306", image.NewRGBA(image.Rect(0, 0, 2, 2))); err == nil {
		t.Fatal("encodeFrame accepted unsupported RGBA format")
	}
	if _, err := (&recordingConn{}).VerifyFrame("gp1294", frame, 0); err == nil {
		t.Fatal("VerifyFrame accepted missing frame command")
	}
	if set, err := (&recordingConn{}).VerifyContrast("unknown", 0, 1, nil); set || err != nil {
		t.Fatalf("unknown driver contrast = (%v, %v), want (false, nil)", set, err)
	}
}

func TestOfflineContrastVerification(t *testing.T) {
	tests := []struct {
		name    string
		driver  string
		level   uint8
		event   busEvent
		pin     *offlinePin
		wantSet bool
		wantErr bool
	}{
		{"command", "ssd1306", 91, busEvent{Type: "command", Bytes: []byte{0x81, 91}}, nil, true, false},
		{"data", "gu3000", 40, busEvent{Type: "data", Bytes: []byte{0x02, 0x44, 0xff, 0x58, 40}}, nil, true, false},
		{"pwm", "st7735", 128, busEvent{}, &offlinePin{duty: gpio.DutyMax / 0xff * 128, pwmSet: true}, true, false},
		{"no-contrast", "st7789", 128, busEvent{}, nil, false, false},
		{"missing-command", "ssd1306", 91, busEvent{Type: "data", Bytes: []byte{0x81, 91}}, nil, true, true},
		{"missing-pwm", "st7735", 128, busEvent{}, &offlinePin{}, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &recordingConn{events: []busEvent{tt.event}}
			gotSet, err := conn.VerifyContrast(tt.driver, 0, tt.level, tt.pin)
			if gotSet != tt.wantSet || (err != nil) != tt.wantErr {
				t.Fatalf("VerifyContrast = (%v, %v), want (%v, error=%v)", gotSet, err, tt.wantSet, tt.wantErr)
			}
		})
	}
}

func TestRecordingConnectionAndVirtualDisplays(t *testing.T) {
	c := &recordingConn{}
	if c.String() != "offline recording connection" || c.Interface() != c {
		t.Fatal("recording connection identity methods returned unexpected values")
	}
	if err := c.Reset(gpio.High); err != nil || c.Close() != nil {
		t.Fatalf("recording connection reset/close failed: %v", err)
	}
	if err := c.Command(0xA0, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := c.Data(3, 4, 5); err != nil {
		t.Fatal(err)
	}
	if c.commands != 1 || c.dataBytes != 6 || !reflect.DeepEqual(c.events[0].Bytes, []byte{0xA0, 1, 2}) || !bytes.Equal(c.events[1].Bytes, []byte{3, 4, 5}) {
		t.Fatalf("recorded traffic = %#v; counts commands=%d bytes=%d", c.events, c.commands, c.dataBytes)
	}

	for _, model := range []string{"mono", "gray2", "gray4", "rgb15", "rgb16"} {
		t.Run(model, func(t *testing.T) {
			d, err := newVirtualDisplay(4, 3, model)
			if err != nil {
				t.Fatalf("newVirtualDisplay: %v", err)
			}
			d.Fill(color.White)
			if d.Bounds() != image.Rect(0, 0, 4, 3) || d.ColorModel() == nil || d.String() == "" {
				t.Fatalf("virtual display metadata invalid: bounds=%v model=%v name=%q", d.Bounds(), d.ColorModel(), d.String())
			}
			if d.At(1, 1) == nil {
				t.Fatal("At returned nil color")
			}
			d.Clear()
			if err := d.Show(false); err != nil {
				t.Fatal(err)
			}
			if err := d.SetContrast(0); err != nil {
				t.Fatal(err)
			}
			if err := d.SetRotation(0); err != nil {
				t.Fatal(err)
			}
			if err := d.Refresh(); err != nil {
				t.Fatal(err)
			}
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
	if _, err := newVirtualDisplay(0, 0, "mono"); err != nil {
		t.Fatalf("default virtual display dimensions rejected: %v", err)
	}
	for _, tc := range []struct {
		w, h  int
		model string
	}{{-1, 4, "mono"}, {4, -1, "mono"}, {4, 4, "unknown"}} {
		if _, err := newVirtualDisplay(tc.w, tc.h, tc.model); err == nil {
			t.Errorf("newVirtualDisplay(%d,%d,%q) succeeded", tc.w, tc.h, tc.model)
		}
	}
	if sameImage(codecMonoFrame(), codecMonoFrame()) != true || sameImage(codecMonoFrame(), codecGray4Frame()) {
		t.Fatal("sameImage returned an unexpected result")
	}
	other := codecMonoFrame()
	other.Set(0, 0, pixel.Off)
	if sameImage(codecMonoFrame(), other) {
		t.Fatal("sameImage accepted different pixel content")
	}
	if reverseBits(reverseBits(0x96)) != 0x96 {
		t.Fatal("reverseBits is not an involution")
	}
	if monoByte(codecMonoFrame(), 0, 8) == 0 {
		t.Fatal("monoByte did not encode partial-page pixels")
	}

	pin := &offlinePin{}
	if pin.String() == "" || pin.Name() == "" || pin.Function() == "" || pin.Number() != -1 {
		t.Fatal("offline pin metadata invalid")
	}
	if err := pin.Out(gpio.High); err != nil || pin.Halt() != nil {
		t.Fatalf("offline pin lifecycle failed: %v", err)
	}
}

func TestBrightnessScaleOverridesAndBounds(t *testing.T) {
	white := image.NewUniform(color.White)
	bright := color.RGBAModel.Convert(atBrightness(white, "st7789", 1).At(0, 0)).(color.RGBA)
	if bright != (color.RGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Fatalf("ST7789 brightness override = %#v", bright)
	}
	if got := atBrightness(white, "ssd1306", 0).Bounds(); got != white.Bounds() {
		t.Fatalf("brightness preview bounds = %v, want %v", got, white.Bounds())
	}
	pin := &offlinePin{}
	if pin.PWM(gpio.DutyMax, 0) != nil || !pin.pwmSet || pin.duty != gpio.DutyMax {
		t.Fatalf("offline PWM state = %#v", pin)
	}
}

func codecMonoFrame() *pixel.MonoVerticalLSBImage {
	img := pixel.NewMonoVerticalLSBImage(9, 9)
	for y := 0; y < 9; y++ {
		for x := 0; x < 9; x++ {
			if (x*3+y*5)%4 == 0 {
				img.Set(x, y, pixel.On)
			}
		}
	}
	return img
}

func codecGray2Frame() *pixel.Gray2Image {
	img := pixel.NewGray2Image(7, 3)
	for y := 0; y < 3; y++ {
		for x := 0; x < 7; x++ {
			img.Set(x, y, pixel.Gray2{Y: uint8((x + y*2) % 4)})
		}
	}
	return img
}

func codecGray4Frame() *pixel.Gray4Image {
	img := pixel.NewGray4Image(7, 3)
	for y := 0; y < 3; y++ {
		for x := 0; x < 7; x++ {
			img.Set(x, y, pixel.Gray4{Y: uint8((x*2 + y*3) % 16)})
		}
	}
	return img
}

func codecRGB15Frame() *pixel.CRGB15Image {
	img := pixel.NewCRGB15Image(5, 3)
	for y := 0; y < 3; y++ {
		for x := 0; x < 5; x++ {
			img.Set(x, y, pixel.CRGB15{V: uint16(x*409 + y*187)})
		}
	}
	return img
}

func codecRGB16Frame() *pixel.CRGB16Image {
	img := pixel.NewCRGB16Image(5, 3)
	for y := 0; y < 3; y++ {
		for x := 0; x < 5; x++ {
			img.Set(x, y, pixel.CRGB16{V: uint16(x*2033 + y*821)})
		}
	}
	return img
}
