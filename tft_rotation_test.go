package display

import "testing"

func TestTFTControllerRotation(t *testing.T) {
	tests := []struct {
		name          string
		madctlCommand byte
		caseCommand   byte
		rowCommand    byte
		writeCommand  byte
		new           func(*recordingConn) (setRotation func(Rotation) error, setWindow func(int, int, int, int) error, rotation func() Rotation)
	}{
		{
			name:          "ST7735",
			madctlCommand: st7735MADCTL,
			caseCommand:   st7735CASET,
			rowCommand:    st7735RASET,
			writeCommand:  st7735RAMWR,
			new: func(c *recordingConn) (func(Rotation) error, func(int, int, int, int) error, func() Rotation) {
				d := &st7735{crgb16Display: crgb16Display{baseDisplay: baseDisplay{
					c: c, width: 240, height: 320, colOffset: 5, rowOffset: 7,
				}}}
				return d.SetRotation, d.SetWindow, func() Rotation { return d.rotation }
			},
		},
		{
			name:          "ST7789",
			madctlCommand: st7789MADCTL,
			caseCommand:   st7789CASET,
			rowCommand:    st7789RASET,
			writeCommand:  st7789RAMWR,
			new: func(c *recordingConn) (func(Rotation) error, func(int, int, int, int) error, func() Rotation) {
				d := &st7789{crgb16Display: crgb16Display{baseDisplay: baseDisplay{
					c: c, width: 240, height: 320, colOffset: 5, rowOffset: 7,
				}}}
				return d.SetRotation, d.SetWindow, func() Rotation { return d.rotation }
			},
		},
	}

	rotations := []struct {
		rotation Rotation
		madctl   byte
	}{
		{NoRotation, 0},
		{Rotate90, st7735ColumnAddressOrder | st7735PageColumnOrder},
		{Rotate180, st7735ColumnAddressOrder | st7735PageAddressOrder},
		{Rotate270, st7735PageAddressOrder | st7735PageColumnOrder},
		{Rotation(4), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &recordingConn{}
			setRotation, setWindow, getRotation := tt.new(c)
			for _, rotation := range rotations {
				c.commands = nil
				c.data = nil
				if err := setRotation(rotation.rotation); err != nil {
					t.Fatalf("SetRotation(%v): %v", rotation.rotation, err)
				}
				if got, want := getRotation(), rotation.rotation%4; got != want {
					t.Fatalf("stored rotation = %v, want %v", got, want)
				}
				if got := c.commands[len(c.commands)-1].command; got != tt.madctlCommand {
					t.Fatalf("rotation command = %#x, want MADCTL %#x", got, tt.madctlCommand)
				}
				if got := c.data[len(c.data)-1][0]; got != rotation.madctl {
					t.Fatalf("MADCTL value = %#x, want %#x", got, rotation.madctl)
				}

				c.commands = nil
				c.data = nil
				if err := setWindow(1, 2, 3, 4); err != nil {
					t.Fatalf("SetWindow after %v: %v", rotation.rotation, err)
				}
				if got, want := []byte{
					c.commands[0].command,
					c.commands[1].command,
					c.commands[2].command,
				}, []byte{tt.caseCommand, tt.rowCommand, tt.writeCommand}; string(got) != string(want) {
					t.Fatalf("window commands = % x, want % x", got, want)
				}
				var addresses []byte
				for _, data := range c.data {
					addresses = append(addresses, data...)
				}
				wantAddresses := []byte{0, 6, 0, 8, 0, 9, 0, 11}
				if rotation.rotation%4 == Rotate90 || rotation.rotation%4 == Rotate270 {
					wantAddresses = []byte{0, 8, 0, 10, 0, 7, 0, 9}
				}
				if string(addresses) != string(wantAddresses) {
					t.Fatalf("window address data = % x, want % x", addresses, wantAddresses)
				}
			}
		})
	}
}
