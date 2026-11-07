package display

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"periph.io/x/conn/v3"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/i2c"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/uart"
)

type fakeUARTPort struct {
	closed   bool
	closeErr error
}

func (p *fakeUARTPort) String() string { return "fake UART port" }
func (p *fakeUARTPort) Close() error {
	p.closed = true
	return p.closeErr
}
func (p *fakeUARTPort) LimitSpeed(physic.Frequency) error { return nil }
func (p *fakeUARTPort) Connect(physic.Frequency, uart.Stop, uart.Parity, uart.Flow, int) (conn.Conn, error) {
	return nil, errors.New("not used")
}

type fakeUARTConn struct {
	transmissions [][]byte
	err           error
}

func (*fakeUARTConn) String() string      { return "fake UART connection" }
func (*fakeUARTConn) Duplex() conn.Duplex { return conn.Full }
func (c *fakeUARTConn) Tx(w, _ []byte) error {
	c.transmissions = append(c.transmissions, append([]byte(nil), w...))
	return c.err
}

type fakeGPIOPin struct {
	name        string
	levels      []gpio.Level
	readLevels  []gpio.Level
	defaultRead gpio.Level
	failOut     error
	failAt      int
	outCalls    int
	failHalt    error
}

func (p *fakeGPIOPin) String() string { return p.name }
func (p *fakeGPIOPin) Halt() error    { return p.failHalt }
func (p *fakeGPIOPin) Name() string   { return p.name }
func (*fakeGPIOPin) Number() int      { return 1 }
func (*fakeGPIOPin) Function() string { return "out" }
func (p *fakeGPIOPin) Out(level gpio.Level) error {
	p.outCalls++
	if p.failOut != nil && (p.failAt == 0 || p.failAt == p.outCalls) {
		return p.failOut
	}
	p.levels = append(p.levels, level)
	return nil
}
func (*fakeGPIOPin) PWM(gpio.Duty, physic.Frequency) error { return nil }
func (*fakeGPIOPin) In(gpio.Pull, gpio.Edge) error         { return nil }
func (p *fakeGPIOPin) Read() gpio.Level {
	if len(p.readLevels) == 0 {
		return p.defaultRead
	}
	level := p.readLevels[0]
	p.readLevels = p.readLevels[1:]
	return level
}
func (*fakeGPIOPin) WaitForEdge(time.Duration) bool { return false }
func (*fakeGPIOPin) Pull() gpio.Pull                { return gpio.PullNoChange }
func (*fakeGPIOPin) DefaultPull() gpio.Pull         { return gpio.PullNoChange }

func TestUARTConnCommandsAndBusyHandshake(t *testing.T) {
	port := &fakeUARTPort{}
	underlying := &fakeUARTConn{}
	c := &uartConn{port: port, conn: underlying}
	if err := c.Command(0xA0, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := c.Data(3, 4); err != nil {
		t.Fatal(err)
	}
	want := [][]byte{{0xA0, 1, 2}, {3, 4}}
	if !reflect.DeepEqual(underlying.transmissions, want) {
		t.Fatalf("UART transmissions = %v, want %v", underlying.transmissions, want)
	}
	if got := c.Interface(); got != underlying {
		t.Fatalf("Interface() = %v, want underlying UART connection", got)
	}
	if c.String() != "UART fake UART port" {
		t.Fatalf("String() = %q", c.String())
	}
	if err := c.Reset(gpio.High); err != nil {
		t.Fatalf("Reset(): %v", err)
	}
	if err := c.Close(); err != nil || !port.closed {
		t.Fatalf("Close() = %v, closed = %v", err, port.closed)
	}

	busyPort := &fakeGPIOPin{name: "busy", readLevels: []gpio.Level{gpio.High, gpio.Low}}
	busyUART := &fakeUARTConn{}
	busy := &uartConn{port: &fakeUARTPort{}, conn: busyUART, busy: busyPort}
	if err := busy.Data(7, 8); err != nil {
		t.Fatal(err)
	}
	want = [][]byte{{7}, {8}}
	if !reflect.DeepEqual(busyUART.transmissions, want) {
		t.Fatalf("busy UART transmissions = %v, want %v", busyUART.transmissions, want)
	}

	writeErr := errors.New("UART write failed")
	failing := &uartConn{port: &fakeUARTPort{}, conn: &fakeUARTConn{err: writeErr}}
	if err := failing.Data(1); !errors.Is(err, writeErr) {
		t.Fatalf("Data error = %v, want %v", err, writeErr)
	}
}

type recordingI2CBus struct {
	closed bool
	addr   uint16
	write  [][]byte
}

func (*recordingI2CBus) String() string                  { return "recording I2C bus" }
func (b *recordingI2CBus) Close() error                  { b.closed = true; return nil }
func (*recordingI2CBus) SetSpeed(physic.Frequency) error { return nil }
func (b *recordingI2CBus) Tx(addr uint16, write, _ []byte) error {
	b.addr = addr
	b.write = append(b.write, append([]byte(nil), write...))
	return nil
}

func TestRootI2CConnectionProtocol(t *testing.T) {
	const number = 572914
	name := "display-test-root-i2c-572914"
	bus := &recordingI2CBus{}
	if err := i2creg.Register(name, nil, number, func() (i2c.BusCloser, error) { return bus, nil }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = i2creg.Unregister(name) })
	reset := &fakeGPIOPin{name: "reset"}
	c, err := OpenI2C(&I2CConfig{Device: number, Addr: 0x3c, Reset: reset})
	if err != nil {
		t.Fatalf("OpenI2C: %v", err)
	}
	if c.Interface() == nil {
		t.Fatal("Interface returned nil")
	}
	if err := c.Command(0xA0, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := c.Data(3, 4); err != nil {
		t.Fatal(err)
	}
	if err := c.Reset(gpio.High); err != nil {
		t.Fatal(err)
	}
	if bus.addr != 0x3c || !reflect.DeepEqual(bus.write, [][]byte{{0, 0xA0, 1, 2}, {0x40, 3, 4}}) {
		t.Fatalf("I2C protocol address=%#x writes=%v", bus.addr, bus.write)
	}
	if err := c.Close(); err != nil || !bus.closed {
		t.Fatalf("Close() = %v, bus closed=%v", err, bus.closed)
	}

	resetErr := errors.New("reset pin failed")
	failedReset := &i2cConn{reset: &fakeGPIOPin{name: "reset", failOut: resetErr}}
	if err := failedReset.Reset(gpio.Low); !errors.Is(err, resetErr) {
		t.Fatalf("reset error = %v, want %v", err, resetErr)
	}
}

func TestParallelConnWritesAndErrors(t *testing.T) {
	pins := make([]*fakeGPIOPin, 8)
	outs := make([]gpio.PinOut, 8)
	for i := range pins {
		pins[i] = &fakeGPIOPin{name: "D"}
		outs[i] = pins[i]
	}
	writePin := &fakeGPIOPin{name: "write"}
	config := &ParallelConfig{
		D0: outs[0], D1: outs[1], D2: outs[2], D3: outs[3],
		D4: outs[4], D5: outs[5], D6: outs[6], D7: outs[7],
		Write: writePin, Ready: &fakeGPIOPin{name: "ready", defaultRead: gpio.High},
	}
	c, err := OpenParallel(config)
	if err != nil {
		t.Fatalf("OpenParallel: %v", err)
	}
	parallel := c.(*parallelConn)
	t.Cleanup(func() { parallel.rateLimiter.Stop() })

	if err := parallel.Command(0xA5, 0x5A); err != nil {
		t.Fatalf("Command: %v", err)
	}
	if got, want := writePin.levels[1:], []gpio.Level{gpio.Low, gpio.High, gpio.Low, gpio.High}; !reflect.DeepEqual(got, want) {
		t.Fatalf("write strobe levels = %v, want %v", got, want)
	}
	for bit, pin := range pins {
		want := gpio.Low
		if 0x5A&(1<<bit) != 0 {
			want = gpio.High
		}
		if got := pin.levels[len(pin.levels)-1]; got != want {
			t.Errorf("data pin %d level = %v, want %v", bit, got, want)
		}
	}
	if got, err := parallel.Write([]byte{0x12, 0x34}); err != nil || got != 2 {
		t.Fatalf("Write() = (%d, %v), want (2, nil)", got, err)
	}
	if err := parallel.WriteWord(0xABCD); err != nil {
		t.Fatalf("WriteWord: %v", err)
	}
	if err := parallel.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	haltErr := errors.New("GPIO halt failed")
	failingClose := &parallelConn{write: &fakeGPIOPin{name: "write", failHalt: haltErr}}
	if err := failingClose.Close(); !errors.Is(err, haltErr) {
		t.Fatalf("Close error = %v, want %v", err, haltErr)
	}

	writeErr := errors.New("GPIO write failed")
	failingPins := make([]*fakeGPIOPin, 8)
	for i := range failingPins {
		failingPins[i] = &fakeGPIOPin{name: "D"}
	}
	failingPins[3].failOut = writeErr
	failing := &parallelConn{
		d0: failingPins[0], d1: failingPins[1], d2: failingPins[2], d3: failingPins[3],
		d4: failingPins[4], d5: failingPins[5], d6: failingPins[6], d7: failingPins[7],
		write: &fakeGPIOPin{name: "write"}, rateLimiter: time.NewTicker(time.Nanosecond),
	}
	defer failing.rateLimiter.Stop()
	if err := failing.WriteByte(0xFF); !errors.Is(err, writeErr) {
		t.Fatalf("WriteByte error = %v, want %v", err, writeErr)
	}

	for _, tc := range []struct {
		name string
		pin  int
		call int
	}{
		{"write-low", -1, 1},
		{"d0", 0, 0}, {"d1", 1, 0}, {"d2", 2, 0}, {"d3", 3, 0},
		{"d4", 4, 0}, {"d5", 5, 0}, {"d6", 6, 0}, {"d7", 7, 0},
		{"write-high", -1, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pins := make([]*fakeGPIOPin, 8)
			for i := range pins {
				pins[i] = &fakeGPIOPin{name: "D"}
			}
			write := &fakeGPIOPin{name: "write"}
			if tc.pin < 0 {
				write.failOut, write.failAt = writeErr, tc.call
			} else {
				pins[tc.pin].failOut = writeErr
			}
			parallel := &parallelConn{
				d0: pins[0], d1: pins[1], d2: pins[2], d3: pins[3],
				d4: pins[4], d5: pins[5], d6: pins[6], d7: pins[7],
				write: write, rateLimiter: time.NewTicker(time.Nanosecond),
			}
			defer parallel.rateLimiter.Stop()
			if err := parallel.WriteByte(0xA5); !errors.Is(err, writeErr) {
				t.Fatalf("WriteByte error = %v, want %v", err, writeErr)
			}
		})
	}
}

func TestOpenParallelRejectsEachMissingDataPin(t *testing.T) {
	if _, err := OpenParallel(nil); err == nil {
		t.Fatal("OpenParallel(nil) succeeded")
	}
	for pinIndex, name := range []string{"D0", "D1", "D2", "D3", "D4", "D5", "D6", "D7"} {
		t.Run(name, func(t *testing.T) {
			pins := make([]gpio.PinOut, 8)
			for i := range pins {
				pins[i] = &fakeGPIOPin{name: "D"}
			}
			pins[pinIndex] = nil
			_, err := OpenParallel(&ParallelConfig{
				D0: pins[0], D1: pins[1], D2: pins[2], D3: pins[3],
				D4: pins[4], D5: pins[5], D6: pins[6], D7: pins[7],
				Write: &fakeGPIOPin{name: "write"},
			})
			var invalid InvalidPin
			if !errors.As(err, &invalid) || invalid.Name != name {
				t.Fatalf("OpenParallel error = %v, want invalid pin %s", err, name)
			}
		})
	}
}
