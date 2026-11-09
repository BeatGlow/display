package display

import (
	"image"
	"image/color"
	"testing"
)

func TestSSDHardwareRotation(t *testing.T) {
	tests := []struct {
		name           string
		new            func(*recordingConn) (Display, error)
		flipCommands   []recordedCommand
		normalCommands []recordedCommand
	}{
		{"SH1106", func(c *recordingConn) (Display, error) { return SH1106(c, &Config{Width: 128, Height: 32}) }, []recordedCommand{{ssd1xxxSetRemap, nil}, {ssd1xxxSetComScanDec, nil}}, []recordedCommand{{ssd1xxxSetSegmentRemap, nil}, {ssd1xxxSetComScanInc, nil}}},
		{"SSD1305", func(c *recordingConn) (Display, error) { return SSD1305(c, &Config{Width: 128, Height: 32}) }, []recordedCommand{{ssd1xxxSetRemap, nil}, {ssd1xxxSetComScanInc, nil}}, []recordedCommand{{ssd1xxxSetSegmentRemap, nil}, {ssd1xxxSetComScanDec, nil}}},
		{"SSD1306", func(c *recordingConn) (Display, error) { return SSD1306(c, &Config{Width: 128, Height: 32}) }, []recordedCommand{{ssd1xxxSetRemap, nil}, {ssd1xxxSetComScanInc, nil}}, []recordedCommand{{ssd1xxxSetSegmentRemap, nil}, {ssd1xxxSetComScanDec, nil}}},
		{"SH1122", func(c *recordingConn) (Display, error) { return SH1122(c, &Config{Width: 128, Height: 32}) }, []recordedCommand{{ssd1xxxSetSegmentRemap, nil}, {ssd1xxxSetComScanDec, nil}}, []recordedCommand{{ssd1xxxSetRemap, nil}, {ssd1xxxSetComScanInc, nil}}},
		{"SSD1322", func(c *recordingConn) (Display, error) { return SSD1322(c, &Config{Width: 64, Height: 32}) }, []recordedCommand{{ssd1322SetRemap, []byte{0x06, 0x11}}}, []recordedCommand{{ssd1322SetRemap, []byte{0x14, 0x11}}}},
		{"SSD1362", func(c *recordingConn) (Display, error) { return SSD1362(c, &Config{}) }, []recordedCommand{{0xA0, []byte{0xD2}}}, []recordedCommand{{0xA0, []byte{0xC3}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &recordingConn{}
			d, err := tt.new(c)
			if err != nil {
				t.Fatal(err)
			}
			nativeBounds := d.Bounds()
			for _, rotation := range []Rotation{NoRotation, Rotate90, Rotate180, Rotate270, Rotation(4)} {
				c.commands = nil
				if err := d.SetRotation(rotation); err != nil {
					t.Fatalf("SetRotation(%v): %v", rotation, err)
				}
				want := tt.normalCommands
				if rotation%4 == Rotate180 {
					want = tt.flipCommands
				}
				if len(c.commands) != len(want) {
					t.Fatalf("commands for %v = %#v, want %#v", rotation, c.commands, want)
				}
				for i := range want {
					if c.commands[i].command != want[i].command || string(c.commands[i].data) != string(want[i].data) {
						t.Fatalf("command[%d] for %v = %#v, want %#v", i, rotation, c.commands[i], want[i])
					}
				}
				quarter := rotation%4 == Rotate90 || rotation%4 == Rotate270
				bounds := d.Bounds()
				if quarter != (bounds.Dx() == nativeBounds.Dy() && bounds.Dy() == nativeBounds.Dx()) {
					t.Fatalf("Bounds for %v = %v", rotation, bounds)
				}
			}
		})
	}
}

func TestBaseDisplayRotationMapsLogicalPixels(t *testing.T) {
	d := &baseDisplay{Image: image.NewRGBA(image.Rect(0, 0, 3, 2))}
	if err := d.SetRotation(Rotate90); err != nil {
		t.Fatal(err)
	}
	if got, want := d.Bounds(), image.Rect(0, 0, 2, 3); got != want {
		t.Fatalf("Bounds() = %v, want %v", got, want)
	}
	d.Set(1, 2, color.White)
	if got := color.RGBAModel.Convert(d.Image.At(2, 0)); got != color.RGBAModel.Convert(color.White) {
		t.Fatalf("backing pixel at mapped coordinate = %v, want white", got)
	}
	if got := color.RGBAModel.Convert(d.At(1, 2)); got != color.RGBAModel.Convert(color.White) {
		t.Fatalf("logical pixel = %v, want white", got)
	}
}
