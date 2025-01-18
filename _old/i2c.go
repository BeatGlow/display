package conn

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unsafe"

	"github.com/beatglow/display/internal/ioctl"
)

const (
	i2cSlave = 0x0703
	i2cFuncs = 0x0705
	i2cRDWR  = 0x0707
	i2cSMBus = 0x0720

	// Modes <uapi/linux/i2c.h>
	i2cSMBusWrite = 0
	i2cSMBusRead  = 1
)

const (
	// Data sizes <uapi/linux/i2c.h>
	i2cSMBusQuick = iota
	i2cSMBusByte
	i2cSMBusByteData
	i2cSMBusWordData
	i2cSMBusProcCall
	i2cSMBusBlockData
	_
	i2cSMBusBlockProcCall
	i2cSMBusI2CBlockData
)

type I2C struct {
	addr  uint8
	f     *os.File
	fd    uintptr
	funcs uint32
}

func OpenI2C(device int, addr uint8) (*I2C, error) {
	if device < 0 {
		var err error
		if device, err = defaultI2CDevice(); err != nil {
			return nil, err
		}
	}

	f, err := os.OpenFile(fmt.Sprintf("%s-%d", i2cDevPath, device), os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	c := &I2C{
		addr: addr,
		f:    f,
		fd:   f.Fd(),
	}
	if err = ioctl.Call(c.fd, i2cFuncs, uintptr(unsafe.Pointer(&c.funcs))); err != nil {
		_ = f.Close()
		return nil, err
	}

	return c, nil
}

func (c *I2C) Close() error {
	return c.f.Close()
}

// Reset does nothing (for now?).
func (i *I2C) Reset() error {
	return nil
}

type i2cMsg struct {
	addr   uint16
	flags  uint16
	length uint16
	buf    uintptr
}

type rdwrIoctlData struct {
	msgs  uintptr // Pointer to i2cMsg
	nmsgs uint32
}

func (c *I2C) Send(p []byte, isCommand bool) error {
	if err := ioctl.Call(c.fd, i2cSlave, uintptr(c.addr)); err != nil {
		return err
	}

	var reg uint8
	if !isCommand {
		reg = 0x40
	}
	p = append([]byte{reg}, p...)

	var buf [2]i2cMsg
	msgs := buf[0:0]
	if len(p) != 0 {
		msgs = buf[:1]
		buf[0].addr = uint16(c.addr)
		buf[0].length = uint16(len(p))
		buf[0].buf = uintptr(unsafe.Pointer(&p[0]))
	}

	data := rdwrIoctlData{
		msgs:  uintptr(unsafe.Pointer(&msgs[0])),
		nmsgs: uint32(len(msgs)),
	}
	return ioctl.Call(c.fd, i2cRDWR, uintptr(unsafe.Pointer(&data)))
}

func defaultI2CDevice() (int, error) {
	infos, err := os.ReadDir(devPath)
	if err != nil {
		return -1, err
	}

	sort.Slice(infos, func(i, j int) bool {
		return strings.Compare(infos[i].Name(), infos[j].Name()) < 0
	})

	for _, info := range infos {
		name := filepath.Join(devPath, info.Name())
		if strings.HasPrefix(name, i2cDevPath+"-") {
			return strconv.Atoi(name[len(i2cDevPath+"-"):])
		}
	}

	return -1, errors.New("no default I2C device could be found; is the i2c kernel interface loaded?")
}
