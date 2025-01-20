package conn

import (
	"fmt"
	"strconv"

	"periph.io/x/conn/v3"
	"periph.io/x/conn/v3/i2c"
	"periph.io/x/conn/v3/i2c/i2creg"
)

type I2C struct {
	bus  i2c.BusCloser
	conn conn.Conn
}

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

func (c *I2C) String() string {
	return fmt.Sprintf("I²C bus %s", c.bus)
}

func (c *I2C) Close() error {
	return c.bus.Close()
}

// Reset does nothing (for now?).
func (i *I2C) Reset() error {
	return nil
}

func (c *I2C) Read(p []byte) (int, error) {
	return len(p), c.conn.Tx(p, p)
}

func (c *I2C) Write(p []byte) (int, error) {
	return len(p), c.conn.Tx(p, nil)
}
