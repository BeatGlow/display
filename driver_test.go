package display

import (
	"errors"
	"image/color"
	"testing"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/physic"
)

var errFakeConn = errors.New("fake connection failure")

type recordedCommand struct {
	command byte
	data    []byte
}

type recordingConn struct {
	commands []recordedCommand
	data     [][]byte
	resets   []gpio.Level
	calls    int
	failCall int
}

func (c *recordingConn) String() string { return "recording connection" }
func (c *recordingConn) Close() error   { return nil }
func (c *recordingConn) Reset(level gpio.Level) error {
	c.resets = append(c.resets, level)
	return nil
}
func (c *recordingConn) Command(command byte, data ...byte) error {
	c.commands = append(c.commands, recordedCommand{command: command, data: append([]byte(nil), data...)})
	return c.result()
}
func (c *recordingConn) Data(data ...byte) error {
	c.data = append(c.data, append([]byte(nil), data...))
	return c.result()
}
func (c *recordingConn) Interface() any { return nil }
func (c *recordingConn) result() error {
	c.calls++
	if c.failCall > 0 && c.calls == c.failCall {
		return errFakeConn
	}
	return nil
}

type fakeBacklight struct{}

func (fakeBacklight) String() string                        { return "fake backlight" }
func (fakeBacklight) Halt() error                           { return nil }
func (fakeBacklight) Name() string                          { return "fake" }
func (fakeBacklight) Number() int                           { return 0 }
func (fakeBacklight) Function() string                      { return "out" }
func (fakeBacklight) Out(gpio.Level) error                  { return nil }
func (fakeBacklight) PWM(gpio.Duty, physic.Frequency) error { return nil }

func TestDriversAgainstRecordingConnection(t *testing.T) {
	tests := []struct {
		name string
		new  func(*recordingConn) (Display, error)
	}{
		{"GP1247AI", func(c *recordingConn) (Display, error) { return GP1247AI(c, &Config{Width: 9, Height: 9}) }},
		{"GP1278", func(c *recordingConn) (Display, error) { return GP1278(c, &Config{Width: 13, Height: 9}) }},
		{"GP1294", func(c *recordingConn) (Display, error) {
			return GP1294(c, &Config{Width: 8, Height: 8, Backlight: fakeBacklight{}})
		}},
		{"GU256X64D7000", func(c *recordingConn) (Display, error) { return GU256X64D7000(c, &Config{Width: 16, Height: 8}) }},
		{"GU3000", func(c *recordingConn) (Display, error) { return GU3000(c, &Config{Width: 8, Height: 8}) }},
		{"SH1106", func(c *recordingConn) (Display, error) { return SH1106(c, &Config{Width: 128, Height: 32}) }},
		{"SH1122", func(c *recordingConn) (Display, error) { return SH1122(c, &Config{Width: 128, Height: 32}) }},
		{"SSD1305", func(c *recordingConn) (Display, error) { return SSD1305(c, &Config{Width: 128, Height: 32}) }},
		{"SSD1306", func(c *recordingConn) (Display, error) { return SSD1306(c, &Config{Width: 128, Height: 32}) }},
		{"SSD1322", func(c *recordingConn) (Display, error) { return SSD1322(c, &Config{Width: 64, Height: 32}) }},
		{"SSD1362", func(c *recordingConn) (Display, error) { return SSD1362(c, &Config{}) }},
		{"ST7735", func(c *recordingConn) (Display, error) { return ST7735(c, &Config{Width: 8, Height: 8}) }},
		{"ST7789", func(c *recordingConn) (Display, error) { return ST7789(c, &Config{Width: 8, Height: 8}) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failedConn := &recordingConn{failCall: 1}
			if d, err := tt.new(failedConn); err == nil || d != nil {
				t.Fatalf("constructor with connection failure = (%v, %v), want (nil, error)", d, err)
			}

			conn := &recordingConn{}
			d, err := tt.new(conn)
			if err != nil {
				t.Fatalf("construct display: %v", err)
			}
			if d.Bounds().Empty() {
				t.Fatalf("empty bounds: %v", d.Bounds())
			}
			if d.String() == "" {
				t.Fatal("empty display description")
			}
			if d.ColorModel() == nil {
				t.Fatal("nil color model")
			}
			if d.At(d.Bounds().Min.X, d.Bounds().Min.Y) == nil {
				t.Fatal("At returned nil color")
			}
			if len(conn.commands)+len(conn.data) == 0 {
				t.Fatal("constructor did not initialize the controller")
			}

			d.Fill(color.Black)
			d.Clear()
			d.Fill(color.White)
			d.Set(d.Bounds().Min.X, d.Bounds().Min.Y, color.White)
			if err := d.Refresh(); err != nil {
				t.Fatalf("refresh: %v", err)
			}
			if conn.payloadBytes() == 0 {
				t.Fatal("refresh did not transmit image data")
			}
			if err := d.SetContrast(127); err != nil {
				t.Errorf("set contrast: %v", err)
			}
			for _, rotation := range []Rotation{NoRotation, Rotate90, Rotate180, Rotate270, Rotation(4)} {
				if err := d.SetRotation(rotation); err != nil {
					t.Errorf("set rotation %v: %v", rotation, err)
				}
			}
			if err := d.Show(false); err != nil {
				t.Errorf("hide display: %v", err)
			}
			if err := d.Show(true); err != nil {
				t.Errorf("show display: %v", err)
			}

			conn.failCall = conn.calls + 1
			if err := d.Refresh(); !errors.Is(err, errFakeConn) {
				t.Errorf("refresh connection error = %v, want %v", err, errFakeConn)
			}
			conn.failCall = 0

			if err := d.Close(); err != nil {
				t.Errorf("close: %v", err)
			}
		})
	}
}

func TestDriverConstructorRejectsUnsupportedSizes(t *testing.T) {
	tests := []struct {
		name string
		new  func(*recordingConn) (Display, error)
	}{
		{"GP1247AI-width", func(c *recordingConn) (Display, error) { return GP1247AI(c, &Config{Width: 257, Height: 1}) }},
		{"GP1247AI-height", func(c *recordingConn) (Display, error) { return GP1247AI(c, &Config{Width: 1, Height: 65}) }},
		{"SH1106", func(c *recordingConn) (Display, error) { return SH1106(c, &Config{Width: 17, Height: 9}) }},
		{"SH1122", func(c *recordingConn) (Display, error) { return SH1122(c, &Config{Width: 17, Height: 9}) }},
		{"SSD1305", func(c *recordingConn) (Display, error) { return SSD1305(c, &Config{Width: 17, Height: 9}) }},
		{"SSD1306", func(c *recordingConn) (Display, error) { return SSD1306(c, &Config{Width: 17, Height: 9}) }},
		{"SSD1322", func(c *recordingConn) (Display, error) { return SSD1322(c, &Config{Width: 17, Height: 9}) }},
		{"SSD1362", func(c *recordingConn) (Display, error) { return SSD1362(c, &Config{Width: 17, Height: 9}) }},
		{"ST7735-width", func(c *recordingConn) (Display, error) { return ST7735(c, &Config{Width: 241, Height: 1}) }},
		{"ST7735-rotated-width", func(c *recordingConn) (Display, error) {
			return ST7735(c, &Config{Width: 321, Height: 1, Rotation: Rotate90})
		}},
		{"ST7789-width", func(c *recordingConn) (Display, error) { return ST7789(c, &Config{Width: 241, Height: 1}) }},
		{"ST7789-rotated-height", func(c *recordingConn) (Display, error) {
			return ST7789(c, &Config{Width: 1, Height: 241, Rotation: Rotate90})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if d, err := tt.new(&recordingConn{}); err == nil || d != nil {
				t.Fatalf("constructor = (%v, %v), want (nil, error)", d, err)
			}
		})
	}
}

func TestRotationString(t *testing.T) {
	for rotation, want := range map[Rotation]string{
		NoRotation: "0°", Rotate90: "90°", Rotate180: "180°", Rotate270: "270°",
		Rotation(4): "0°",
	} {
		if got := rotation.String(); got != want {
			t.Errorf("Rotation(%d).String() = %q, want %q", rotation, got, want)
		}
	}
}

func (c *recordingConn) payloadBytes() int {
	n := 0
	for _, command := range c.commands {
		n += len(command.data)
	}
	for _, data := range c.data {
		n += len(data)
	}
	return n
}
