package conn

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSPIValuesAndFileIO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spidev")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	c := &SPI{f: f, fd: f.Fd(), mode: SPIMode2, bitsPerWord: 8, maxSpeedHz: 4_000_000}
	t.Cleanup(func() { _ = c.Close() })

	if c.Mode() != SPIMode2 || c.BitsPerWord() != 8 || c.MaxSpeed() != 4_000_000 {
		t.Fatalf("SPI getters = mode %v, bits %d, speed %d", c.Mode(), c.BitsPerWord(), c.MaxSpeed())
	}
	if got := c.String(); got == "" {
		t.Fatal("String() returned empty")
	}
	if err := c.SetBitsPerWord(7); err == nil {
		t.Fatal("SetBitsPerWord accepted fewer than 8 bits")
	}
	if err := c.SetBitsPerWord(33); err == nil {
		t.Fatal("SetBitsPerWord accepted more than 32 bits")
	}
	if err := c.SetBitsPerWord(8); err != nil {
		t.Fatalf("SetBitsPerWord unchanged value: %v", err)
	}
	if err := c.SetMaxSpeed(-1); err != nil {
		t.Fatalf("SetMaxSpeed negative no-op: %v", err)
	}
	if err := c.SetMaxSpeed(4_000_000); err != nil {
		t.Fatalf("SetMaxSpeed unchanged value: %v", err)
	}
	if err := c.SetMode(SPIMode0); err == nil {
		t.Fatal("SetMode unexpectedly succeeded on a regular file")
	}
	if err := c.SetBitsPerWord(16); err == nil {
		t.Fatal("SetBitsPerWord unexpectedly succeeded on a regular file")
	}
	if err := c.SetMaxSpeed(8_000_000); err == nil {
		t.Fatal("SetMaxSpeed unexpectedly succeeded on a regular file")
	}

	if n, err := c.Write([]byte{1, 2, 3}); err != nil || n != 3 {
		t.Fatalf("Write() = (%d, %v), want (3, nil)", n, err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 3)
	if n, err := c.Read(buf); err != nil || n != 3 {
		t.Fatalf("Read() = (%d, %v), want (3, nil)", n, err)
	}
	if string(buf) != string([]byte{1, 2, 3}) {
		t.Fatalf("Read bytes = %v", buf)
	}
}

func TestSPIOpenReportsUnavailableDevice(t *testing.T) {
	if _, err := OpenSPI(999, 999); err == nil {
		t.Fatal("OpenSPI unexpectedly opened an unavailable device")
	}
}
