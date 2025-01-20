package display

import (
	"errors"
	"fmt"
	"log"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"

	"github.com/BeatGlow/display/conn"
)

// Conn errors.
var (
	ErrResetPin = errors.New("display: reset GPIO pin is invalid")
	ErrDCPin    = errors.New("display: data/command (DC) GPIO pin is invalid")
)

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

// I2CConfig describes the I²C bus configuration.
type I2CConfig struct {
	// Device is the I²C device, use -1 to use the first available device.
	Device int

	// Addr is the I²C address.
	Addr uint8

	// Reset pin.
	Reset gpio.PinOut
}

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

func (c *i2cConn) Command(cmnd byte, args ...byte) (err error) {
	_, err = c.I2C.Write(append([]byte{0x00, cmnd}, args...))
	return
}

func (c *i2cConn) Data(data ...byte) (err error) {
	_, err = c.I2C.Write(append([]byte{0x40}, data...))
	return
}

func (c *i2cConn) Reset(level gpio.Level) error {
	return c.reset.Out(level)
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

// DefaultSPIConfig are the default configuration values.
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
	debug     bool
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

	if config.Reset == nil || config.Reset == gpio.INVALID {
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
		//debug:        true,
	}, nil
}

func (c *spiConn) String() string {
	return fmt.Sprintf("SPI bus %s", c.bus)
}

func (c *spiConn) Close() error {
	return c.bus.Close()
}

func (c *spiConn) Reset(level gpio.Level) error {
	return c.reset.Out(level)
}

func (c *spiConn) updateDC(level gpio.Level) error {
	if c.dcLevel != level {
		if err := c.dc.Out(level); err != nil {
			return err
		}
		c.dcLevel = level
	}
	return nil
}

func (c *spiConn) updateCS(level gpio.Level) error {
	if c.cs == nil {
		return nil
	}
	return c.cs.Out(level)
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

func (c *spiConn) writeChunked(data []byte) (err error) {
	if len(data) < int(c.batchSize) {
		_, err = c.bus.Write(data)
		return
	}

	log.Printf("write %d bytes of data in %d chunks", len(data), (len(data)+int(c.batchSize)-1)/int(c.batchSize))
	buffer := data
	for len(buffer) > 0 {
		if len(buffer) > int(c.batchSize) {
			if _, err = c.bus.Write(buffer[:c.batchSize]); err != nil {
				return
			}
			buffer = buffer[c.batchSize:]
		} else {
			if _, err = c.bus.Write(buffer); err != nil {
				return
			}
			buffer = nil
		}
	}
	return
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
