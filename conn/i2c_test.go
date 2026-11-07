package conn

import (
	"errors"
	"reflect"
	"testing"

	"periph.io/x/conn/v3/i2c"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/conn/v3/physic"
)

type testI2CBus struct {
	closed bool
	addr   uint16
	writes [][]byte
	reads  [][]byte
	err    error
}

func (b *testI2CBus) String() string { return "test I2C bus" }
func (b *testI2CBus) Close() error {
	b.closed = true
	return nil
}
func (*testI2CBus) SetSpeed(physic.Frequency) error { return nil }
func (b *testI2CBus) Tx(addr uint16, w, r []byte) error {
	b.addr = addr
	b.writes = append(b.writes, append([]byte(nil), w...))
	b.reads = append(b.reads, append([]byte(nil), r...))
	if b.err != nil {
		return b.err
	}
	for i := range r {
		r[i] = byte(i + 10)
	}
	return nil
}

func TestI2COpenAndTransactions(t *testing.T) {
	const busNumber = 572913
	bus := &testI2CBus{}
	name := "display-test-i2c-572913"
	if err := i2creg.Register(name, nil, busNumber, func() (i2c.BusCloser, error) { return bus, nil }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = i2creg.Unregister(name) })

	device, err := OpenI2C(busNumber, 0x3c)
	if err != nil {
		t.Fatalf("OpenI2C: %v", err)
	}
	if got := device.String(); got != "I²C bus test I2C bus" {
		t.Fatalf("String() = %q", got)
	}
	if err := device.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if _, err := device.Write([]byte{1, 2}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	read := make([]byte, 3)
	if _, err := device.Read(read); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !reflect.DeepEqual(read, []byte{10, 11, 12}) {
		t.Fatalf("Read bytes = %v", read)
	}
	if bus.addr != 0x3c || !reflect.DeepEqual(bus.writes, [][]byte{{1, 2}, {0, 0, 0}}) || !reflect.DeepEqual(bus.reads, [][]byte{nil, {0, 0, 0}}) {
		t.Fatalf("I2C transactions address=%#x writes=%v reads=%v", bus.addr, bus.writes, bus.reads)
	}
	if err := device.Close(); err != nil || !bus.closed {
		t.Fatalf("Close() = %v, bus closed=%v", err, bus.closed)
	}

	writeErr := errors.New("I2C transaction failed")
	failedBus := &testI2CBus{err: writeErr}
	failed := &I2C{bus: failedBus, conn: &i2c.Dev{Bus: failedBus, Addr: 0x3d}}
	if _, err := failed.Write([]byte{4}); !errors.Is(err, writeErr) {
		t.Fatalf("Write error = %v, want %v", err, writeErr)
	}
	if _, err := failed.Read(make([]byte, 1)); !errors.Is(err, writeErr) {
		t.Fatalf("Read error = %v, want %v", err, writeErr)
	}
	if _, err := OpenI2C(busNumber+1, 0x3c); err == nil {
		t.Fatal("OpenI2C accepted an unregistered bus")
	}
}
