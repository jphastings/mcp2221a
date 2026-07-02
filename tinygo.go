package mcp2221a

import (
	"errors"
)

var errNot7BitI2CAddress = errors.New("10-bit I²C addresses are not supported")

// Tx performs an I²C transaction with the 7-bit address addr: the bytes in w
// are written (if any), then bytes are read into r (if any). When both
// buffers are given, the read is issued with a repeated-START while the bus
// is still held, matching the write-then-read register convention used by
// most devices:
//
//	bus.Tx(addr, []byte{reg}, buf) // Reads register reg into buf.
//	bus.Tx(addr, append([]byte{reg}, buf...), nil) // Writes buf into register reg.
//
// Tx implements the tinygo.org/x/drivers I2C interface, so an MCP2221A can
// host TinyGo device drivers directly. See examples/tinygo-drivers for usage.
func (mod *I2C) Tx(addr uint16, w, r []byte) error {

	unlock, err := mod.lock()
	if nil != err {
		return err
	}
	defer unlock()

	return mod.tx(addr, w, r)
}

// tx implements Tx. The caller must hold mcp.mu.
func (mod *I2C) tx(addr uint16, w, r []byte) error {

	if addr > 0x7F {
		return errNot7BitI2CAddress
	}

	if len(w) > 0 {
		// suppress the STOP condition when a read follows, so that the read can
		// be issued with a repeated-START.
		if err := mod.write(len(r) == 0, uint8(addr), w, uint16(len(w))); nil != err {
			return err
		}
	}

	if len(r) > 0 {
		// a repeated-START is only valid while the bus is held open by a
		// preceding write; a bare read must begin with a plain START.
		in, err := mod.read(len(w) > 0, uint8(addr), uint16(len(r)))
		if nil != err {
			return err
		}
		copy(r, in)
	}

	return nil
}

// WriteRegister writes the given data to a device register. Along with Tx and
// ReadRegister, it satisfies the register-based I2C interface used by older
// releases of tinygo.org/x/drivers (and mirrored by TinyGo's machine.I2C).
func (mod *I2C) WriteRegister(addr uint8, reg uint8, data []byte) error {

	unlock, err := mod.lock()
	if nil != err {
		return err
	}
	defer unlock()

	buf := make([]byte, len(data)+1)
	buf[0] = reg
	copy(buf[1:], data)
	return mod.tx(uint16(addr), buf, nil)
}

// ReadRegister reads len(data) bytes from a device register into data. Along
// with Tx and WriteRegister, it satisfies the register-based I2C interface
// used by older releases of tinygo.org/x/drivers (and mirrored by TinyGo's
// machine.I2C).
func (mod *I2C) ReadRegister(addr uint8, reg uint8, data []byte) error {

	unlock, err := mod.lock()
	if nil != err {
		return err
	}
	defer unlock()

	return mod.tx(uint16(addr), []byte{reg}, data)
}
