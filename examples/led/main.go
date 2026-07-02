// Blinks an LED on an I²C device using the register-based WriteRegister
// convenience method (here, the brightness register of a device at 0x6F).
package main

import (
	"log"
	"time"

	mcp "github.com/ardnew/mcp2221a"
)

const (
	ledAddr       = uint8(0x6F)
	brightnessReg = uint8(0x19)
)

func main() {

	m, err := mcp.New(0, mcp.VID, mcp.PID)
	if nil != err {
		log.Fatalf("Open(): %v", err)
	}
	defer func() { _ = m.Close() }()

	log.Print(mcp.PackageVersion())

	// reset device to default settings stored in flash memory
	if err := m.Reset(5 * time.Second); nil != err {
		log.Fatalf("Reset(): %v", err)
	}

	// configure I2C module to use default baud rate (optional)
	if err := m.I2C.SetConfig(mcp.I2CBaudRate); nil != err {
		log.Fatalf("I2C.SetConfig(): %v", err)
	}

	for {
		check(ledSet(m.I2C, true))
		time.Sleep(40 * time.Millisecond)
		check(ledSet(m.I2C, false))
		time.Sleep(20 * time.Millisecond)
		check(ledSet(m.I2C, true))
		time.Sleep(40 * time.Millisecond)
		check(ledSet(m.I2C, false))
		time.Sleep(200 * time.Millisecond)
	}
}

func ledSet(i2c *mcp.I2C, on bool) error {
	brightness := uint8(0)
	if on {
		brightness = 0xFF
	}
	return i2c.WriteRegister(ledAddr, brightnessReg, []byte{brightness})
}

func check(err error) {
	if nil != err {
		panic(err)
	}
}
