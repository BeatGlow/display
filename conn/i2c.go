package conn

import (
	"fmt"
	"strconv"

	"periph.io/x/conn/v3"
	"periph.io/x/conn/v3/i2c"
	"periph.io/x/conn/v3/i2c/i2creg"
)

// I2C is an open I²C device connection.
type I2C struct {
	bus  i2c.BusCloser
	conn conn.Conn
}

// OpenI2C opens the numbered I²C device at addr. A negative device selects
// the first available bus.
func OpenI2C(device int, addr uint8) (*I2C, error) {
	var (
		bus i2c.BusCloser
		err error
	)
	if device < 0 {
		bus, err = i2creg.Open("")
	} else {
		bus, err = i2creg.Open(strconv.FormatInt(int64(device), 10))
	}
	if err != nil {
		return nil, err
	}

	return &I2C{
		bus:  bus,
		conn: &i2c.Dev{Bus: bus, Addr: uint16(addr)},
	}, nil
}

// String returns a human-readable description.
func (c *I2C) String() string {
	return fmt.Sprintf("I²C bus %s", c.bus)
}

// Close releases the I²C device connection.
func (c *I2C) Close() error {
	return c.bus.Close()
}

// Reset is a no-op because this connection has no reset pin.
func (i *I2C) Reset() error {
	return nil
}

// Read reads bytes from the connection.
func (c *I2C) Read(p []byte) (int, error) {
	return len(p), c.conn.Tx(p, p)
}

// Write writes bytes to the connection.
func (c *I2C) Write(p []byte) (int, error) {
	return len(p), c.conn.Tx(p, nil)
}
