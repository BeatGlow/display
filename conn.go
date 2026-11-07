package display

import (
	"errors"
	"fmt"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/pin"
	"periph.io/x/conn/v3/uart"
	"periph.io/x/conn/v3/uart/uartreg"

	"periph.io/x/host/v3"

	"github.com/BeatGlow/display/v2/conn"
)

type InvalidPin struct {
	Name string
}

func (err InvalidPin) Error() string {
	return fmt.Sprintf("display: %s GPIO pin is invalid", err.Name)
}

// Conn is the connection interface for communicating with hardware.
type Conn interface {
	String() string

	// Close the connection.
	Close() error

	// Reset sets the reset pin to the provided level.
	Reset(gpio.Level) error

	// Command sends a command byte with optional arguments.
	Command(byte, ...byte) error

	// Data sends data bytes.
	Data(...byte) error

	// Interface is the underlying interface.
	Interface() any

	// Send data over the serial interface.
	//Send(data []byte, isCommand bool) error

	// SendByte sends one data byte over the serial interface.
	//SendByte(data byte, isCommand bool) error
}

type SPI interface {
	Conn

	// SetDataLow changes the data/command direction behaviour.
	SetDataLow(bool)

	// SetMode requests a SPI mode.
	SetMode(mode conn.SPIMode) error

	// SetMaxSpeed requests a SPI speed.
	SetMaxSpeed(hz int) error
}

type Parallel interface {
	Conn
}

// UARTConfig describes a UART connection.
type UARTConfig struct {
	// Port is the UART port name or alias. An empty name selects the first
	// available port.
	Port string
	// Baud is the configured baud rate. It must match the display's serial
	// interface setting and cannot exceed 115200.
	Baud int
	// Busy is an optional SBUSY input. The display drives it high while busy;
	// connect it through a suitable level shifter.
	Busy gpio.PinIn
}

// OpenUART opens a UART connection using 8 data bits, no parity, one stop
// bit, and no flow control.
func OpenUART(config *UARTConfig) (Conn, error) {
	if config == nil {
		return nil, errors.New("display: UART config can't be nil")
	}
	if config.Baud <= 0 || config.Baud > 115200 {
		return nil, fmt.Errorf("display: invalid UART baud rate %d", config.Baud)
	}
	if _, err := host.Init(); err != nil {
		return nil, err
	}
	port, err := uartreg.Open(config.Port)
	if err != nil {
		return nil, err
	}
	c, err := port.Connect(physic.Frequency(config.Baud)*physic.Hertz, uart.One, uart.NoParity, uart.NoFlow, 8)
	if err != nil {
		_ = port.Close()
		return nil, err
	}
	return &uartConn{port: port, conn: c, busy: config.Busy}, nil
}

type uartConn struct {
	port uart.PortCloser
	conn interface {
		Tx(w, r []byte) error
	}
	busy gpio.PinIn
}

func (c *uartConn) String() string {
	return fmt.Sprintf("UART %v", c.port)
}

func (c *uartConn) Close() error {
	return c.port.Close()
}

func (c *uartConn) Reset(gpio.Level) error {
	return nil
}

func (c *uartConn) Command(command byte, args ...byte) error {
	return c.Data(append([]byte{command}, args...)...)
}

func (c *uartConn) Data(data ...byte) error {
	if isValidPin(c.busy) {
		for _, b := range data {
			c.waitReady()
			if err := c.conn.Tx([]byte{b}, nil); err != nil {
				return err
			}
		}
		return nil
	}
	return c.conn.Tx(data, nil)
}

func (c *uartConn) waitReady() {
	if isValidPin(c.busy) {
		for c.busy.Read() {
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func (c *uartConn) Interface() any {
	return c.conn
}

// I2CConfig describes the I²C bus configuration.
type I2CConfig struct {
	// Device is the I²C device, use -1 to use the first available device.
	Device int

	// Addr is the I²C address.
	Addr uint8

	// Reset pin.
	Reset gpio.PinOut
}

// DefaultI2CConfig are the default configuration values for I²C connections.
var DefaultI2CConfig = I2CConfig{
	Device: -1,
	Addr:   0x3c,
}

type i2cConn struct {
	*conn.I2C
	reset gpio.PinOut
}

func OpenI2C(config *I2CConfig) (Conn, error) {
	if config == nil {
		config = new(I2CConfig)
		*config = DefaultI2CConfig
	}

	c, err := conn.OpenI2C(config.Device, config.Addr)
	if err != nil {
		return nil, err
	}

	return &i2cConn{
		I2C:   c,
		reset: config.Reset,
	}, nil
}

func (c *i2cConn) Interface() any {
	return c.I2C
}

func (c *i2cConn) Command(cmnd byte, args ...byte) (err error) {
	_, err = c.Write(append([]byte{0x00, cmnd}, args...))
	return
}

func (c *i2cConn) Data(data ...byte) (err error) {
	_, err = c.Write(append([]byte{0x40}, data...))
	return
}

func (c *i2cConn) Reset(level gpio.Level) error {
	if err := c.reset.Out(level); err != nil {
		return fmt.Errorf("display: error setting I2C reset pin level: %w", err)
	}
	return nil
}

// SPIConfig describes the SPI bus configuration.
type SPIConfig struct {
	Bus       int
	Device    int
	Mode      uint8
	SpeedHz   uint32
	DataLow   bool
	BatchSize uint
	Reset     gpio.PinOut
	DC        gpio.PinOut
	CE        gpio.PinOut
}

// DefaultSPIConfig are the default configuration values for SPI connections.
var DefaultSPIConfig = SPIConfig{
	Bus:       0,
	Device:    0,
	Mode:      0,
	SpeedHz:   8_000_000,
	BatchSize: 4096,
	Reset:     gpioreg.ByName("GPIO25"),
	DC:        gpioreg.ByName("GPIO24"),
}

// ValidSPISpeeds are common valid SPI bus speeds.
var ValidSPISpeeds = []uint32{
	500_000,
	1_000_000,
	2_000_000,
	4_000_000,
	8_000_000,
	16_000_000,
	20_000_000,
	24_000_000,
	28_000_000,
	32_000_000,
	36_000_000,
	40_000_000,
	48_000_000,
	50_000_000,
	52_000_000,
}

type spiConn struct {
	bus       *conn.SPI
	reset     gpio.PinOut
	dc        gpio.PinOut
	dcLevel   gpio.Level
	cs        gpio.PinOut
	dataLow   bool
	batchSize uint
}

func OpenSPI(config *SPIConfig) (Conn, error) {
	if config == nil {
		config = new(SPIConfig)
		*config = DefaultSPIConfig
	}

	if !isValidPin(config.Reset) {
		return nil, ErrResetPin
	}

	if config.SpeedHz == 0 {
		config.SpeedHz = DefaultSPIConfig.SpeedHz
	}
	if config.BatchSize == 0 {
		config.BatchSize = DefaultSPIConfig.BatchSize
	}

	c, err := conn.OpenSPI(config.Bus, config.Device)
	if err != nil {
		return nil, err
	}

	if config.SpeedHz > 0 {
		var valid bool
		for _, speed := range ValidSPISpeeds {
			if valid = speed == config.SpeedHz; valid {
				break
			}
		}
		if !valid {
			_ = c.Close()
			return nil, fmt.Errorf("display: invalid SPI speed %dHz", config.SpeedHz)
		}

		if err = c.SetMaxSpeed(int(config.SpeedHz)); err != nil {
			_ = c.Close()
			return nil, err
		}
	}

	return &spiConn{
		bus:       c,
		batchSize: config.BatchSize,
		dataLow:   config.DataLow,
		reset:     config.Reset,
		dc:        config.DC,
		cs:        config.CE,
	}, nil
}

func (c *spiConn) Interface() any {
	return c.bus
}

func (c *spiConn) String() string {
	return fmt.Sprintf("SPI bus %s", c.bus)
}

func (c *spiConn) Close() error {
	return c.bus.Close()
}

func (c *spiConn) Reset(level gpio.Level) error {
	if err := c.reset.Out(level); err != nil {
		return fmt.Errorf("display: error setting SPI reset pin level: %w", err)
	}
	return nil
}

func (c *spiConn) updateDC(level gpio.Level) error {
	if c.dcLevel != level {
		if err := c.dc.Out(level); err != nil {
			return fmt.Errorf("display: error setting SPI data/control pin level: %w", err)
		}
		c.dcLevel = level
	}
	return nil
}

func (c *spiConn) updateCS(level gpio.Level) error {
	if c.cs == nil {
		return nil
	}
	if err := c.cs.Out(level); err != nil {
		return fmt.Errorf("display: error setting SPI chip select pin level: %w", err)
	}
	return nil
}

func (c *spiConn) Command(cmnd byte, data ...byte) (err error) {
	if err = c.updateCS(gpio.Low); err != nil {
		return
	}
	if err = c.updateDC(gpio.Level(c.dataLow)); err != nil {
		return
	}
	if _, err = c.bus.Write([]byte{cmnd}); err != nil {
		return
	}
	if len(data) > 0 {
		if err = c.updateDC(gpio.Level(!c.dataLow)); err != nil {
			return
		}
		if err = c.writeChunked(data); err != nil {
			return
		}
	}
	if err = c.updateCS(gpio.High); err != nil {
		return
	}
	return
}

func (c *spiConn) Data(data ...byte) (err error) {
	if len(data) == 0 {
		return
	}
	if err = c.updateDC(gpio.Level(!c.dataLow)); err != nil {
		return
	}
	if err = c.updateCS(gpio.Low); err != nil {
		return
	}
	if err = c.writeChunked(data); err != nil {
		return
	}
	if err = c.updateCS(gpio.High); err != nil {
		return
	}
	return
}

func (c *spiConn) writeChunked(data []byte) error {
	return writeChunkedTo(c.bus, c.batchSize, data)
}

func writeChunkedTo[T interface{ Write([]byte) (int, error) }](writer T, batchSize uint, data []byte) error {
	if len(data) < int(batchSize) {
		_, err := writer.Write(data)
		return err
	}

	for len(data) > 0 {
		chunk := len(data)
		if chunk > int(batchSize) {
			chunk = int(batchSize)
		}
		if _, err := writer.Write(data[:chunk]); err != nil {
			return err
		}
		data = data[chunk:]
	}
	return nil
}

func (c *spiConn) SetDataLow(v bool) {
	c.dataLow = v
}

func (c *spiConn) SetMode(mode conn.SPIMode) error {
	return c.bus.SetMode(mode)
}

func (c *spiConn) SetMaxSpeed(hz int) error {
	return c.bus.SetMaxSpeed(hz)
}

// ParallelConfig is the configuration for a parallel (aka 8080) connection.
type ParallelConfig struct {
	// Data pins.
	D0, D1, D2, D3, D4, D5, D6, D7 gpio.PinOut

	// Write pin.
	Write gpio.PinOut

	// Ready pin.
	Ready gpio.PinIn

	// UpdateFrequency is the maximum update frequency.
	UpdateFrequency physic.Frequency
}

type parallelConn struct {
	d0, d1, d2, d3, d4, d5, d6, d7 gpio.PinOut
	write                          gpio.PinOut
	ready                          gpio.PinIn
	rateLimiter                    *time.Ticker
}

// OpenParallel opens a parallel connection (also known as 8080 connection).
func OpenParallel(config *ParallelConfig) (Conn, error) {
	if config == nil {
		return nil, errors.New("display: config can't be nil")
	}
	if config.UpdateFrequency == 0 {
		config.UpdateFrequency = 2 * physic.MegaHertz
	}
	if !isValidPin(config.D0) {
		return nil, InvalidPin{"D0"}
	}
	if !isValidPin(config.D1) {
		return nil, InvalidPin{"D1"}
	}
	if !isValidPin(config.D0) {
		return nil, InvalidPin{"D2"}
	}
	if !isValidPin(config.D3) {
		return nil, InvalidPin{"D3"}
	}
	if !isValidPin(config.D4) {
		return nil, InvalidPin{"D4"}
	}
	if !isValidPin(config.D5) {
		return nil, InvalidPin{"D5"}
	}
	if !isValidPin(config.D6) {
		return nil, InvalidPin{"D6"}
	}
	if !isValidPin(config.D7) {
		return nil, InvalidPin{"D7"}
	}
	if !isValidPin(config.Write) {
		return nil, InvalidPin{"write"}
	}

	if err := config.Write.Out(gpio.High); err != nil {
		return nil, err
	}
	if isValidPin(config.Ready) {
		time.Sleep(100 * time.Millisecond)
		if ready := config.Ready.Read(); !ready {
			time.Sleep(100 * time.Millisecond)
			if ready = config.Ready.Read(); !ready {
				return nil, ErrNotReady
			}
		}
	}

	return &parallelConn{
		d0:          config.D0,
		d1:          config.D1,
		d2:          config.D2,
		d3:          config.D3,
		d4:          config.D4,
		d5:          config.D5,
		d6:          config.D6,
		d7:          config.D7,
		write:       config.Write,
		ready:       config.Ready,
		rateLimiter: time.NewTicker(config.UpdateFrequency.Period()),
	}, nil
}

func (c *parallelConn) Interface() any {
	return c
}

func (parallelConn) String() string {
	return "parallel"
}

func (c *parallelConn) Close() error {
	return c.write.Halt()
}

func (c *parallelConn) Command(command byte, args ...byte) error {
	if err := c.WriteByte(command); err != nil {
		return err
	}
	_, err := c.Write(args)
	return err
}

func (c *parallelConn) Data(args ...byte) error {
	_, err := c.Write(args)
	return err
}

func (c *parallelConn) waitReady() {
	if isValidPin(c.ready) {
		for !c.ready.Read() {
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func (c *parallelConn) Reset(_ gpio.Level) error {
	return nil
}

func (c *parallelConn) Write(p []byte) (n int, err error) {
	for ; n < len(p); n++ {
		if err = c.WriteByte(p[n]); err != nil {
			return
		}
	}
	return
}

func (c *parallelConn) WriteByte(b byte) error {
	c.waitReady()
	if err := c.write.Out(gpio.Low); err != nil {
		return err
	}
	if err := c.d0.Out(gpio.Level(b&0x01 != 0)); err != nil {
		return err
	}
	if err := c.d1.Out(gpio.Level(b&0x02 != 0)); err != nil {
		return err
	}
	if err := c.d2.Out(gpio.Level(b&0x04 != 0)); err != nil {
		return err
	}
	if err := c.d3.Out(gpio.Level(b&0x08 != 0)); err != nil {
		return err
	}
	if err := c.d4.Out(gpio.Level(b&0x10 != 0)); err != nil {
		return err
	}
	if err := c.d5.Out(gpio.Level(b&0x20 != 0)); err != nil {
		return err
	}
	if err := c.d6.Out(gpio.Level(b&0x40 != 0)); err != nil {
		return err
	}
	if err := c.d7.Out(gpio.Level(b&0x80 != 0)); err != nil {
		return err
	}
	if err := c.write.Out(gpio.High); err != nil {
		return err
	}
	<-c.rateLimiter.C
	return nil
}

func (c *parallelConn) WriteWord(w uint16) error {
	if err := c.WriteByte(byte(w & 0xFF)); err != nil {
		return err
	}
	return c.WriteByte(byte(w >> 8))
}

func isValidPin(pin pin.Pin) bool {
	return pin != nil && pin != gpio.INVALID
}

var (
	_ Conn = (*i2cConn)(nil)
	_ Conn = (*spiConn)(nil)
	_ Conn = (*parallelConn)(nil)
	_ Conn = (*uartConn)(nil)
)
