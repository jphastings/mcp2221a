// Demonstrates driving a TinyGo I²C device driver through an MCP2221A.
// The mcp.I2C module satisfies the tinygo.org/x/drivers I2C interface, so any
// I²C driver from that package can be used directly.
//
// This example reads an Adafruit I2C QT Rotary Encoder (seesaw, address 0x36)
// and prints its position whenever it changes.
package main

import (
	"fmt"
	"log"
	"time"

	mcp "github.com/ardnew/mcp2221a"
	"tinygo.org/x/drivers/seesaw"
)

func main() {

	m, err := mcp.New(0, mcp.VID, mcp.PID)
	if nil != err {
		log.Fatalf("Open(): %v", err)
	}
	defer m.Close()

	log.Print(mcp.PackageVersion())

	// reset device to default settings stored in flash memory
	if err := m.Reset(5 * time.Second); nil != err {
		log.Fatalf("Reset(): %v", err)
	}

	// configure I2C module to use default baud rate (optional)
	if err := m.I2C.SetConfig(mcp.I2CBaudRate); nil != err {
		log.Fatalf("I2C.SetConfig(): %v", err)
	}

	dev := seesaw.New(m.I2C)
	dev.Address = 0x36

	prev := int32(0)
	for {
		pos, err := dev.GetEncoderPosition(0, false)
		if nil != err {
			log.Fatalf("GetEncoderPosition(): %v", err)
		}

		if pos != prev {
			fmt.Println("Position:", pos)
			prev = pos
		} else {
			time.Sleep(20 * time.Millisecond)
		}
	}
}
