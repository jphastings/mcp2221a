package mcp2221a

import (
	"bytes"
	"testing"
	"unicode/utf16"
)

// fakeDevice implements usb.Device, recording every HID report written and
// answering each command with a minimal success response. I²C get-data
// commands return readData as a completed read.
type fakeDevice struct {
	reports  [][]byte
	readData []byte
	next     []byte
}

func (d *fakeDevice) Close() error { return nil }

func (d *fakeDevice) Write(b []byte) (int, error) {
	report := append([]byte(nil), b...)
	d.reports = append(d.reports, report)

	rsp := make([]byte, MsgSz)
	rsp[0] = report[0]
	if report[0] == cmdI2CReadGetData {
		rsp[2] = i2cStateReadComplete
		rsp[3] = byte(len(d.readData))
		copy(rsp[4:], d.readData)
	}
	d.next = rsp
	return len(b), nil
}

func (d *fakeDevice) Read(b []byte) (int, error) {
	copy(b, d.next)
	return len(b), nil
}

func (d *fakeDevice) ReadTimeout(b []byte, timeout int) (int, error) {
	return d.Read(b)
}

func (d *fakeDevice) GetFeatureReport(b []byte) (int, error) { return len(b), nil }

func (d *fakeDevice) SendFeatureReport(b []byte) (int, error) { return len(b), nil }

// i2cCommands returns the sequence of I²C transfer command IDs issued to the
// device, ignoring status polling.
func (d *fakeDevice) i2cCommands() []byte {
	ids := []byte{}
	for _, r := range d.reports {
		if r[0] != cmdStatus {
			ids = append(ids, r[0])
		}
	}
	return ids
}

func newTestI2C(readData []byte) (*I2C, *fakeDevice) {
	dev := &fakeDevice{readData: readData}
	mcp := &MCP2221A{Device: dev}
	mcp.I2C = &I2C{mcp}
	return mcp.I2C, dev
}

func TestTxWriteThenRead(t *testing.T) {
	want := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	i2c, dev := newTestI2C(want)

	r := make([]byte, len(want))
	if err := i2c.Tx(0x36, []byte{0x19}, r); err != nil {
		t.Fatalf("Tx(): %v", err)
	}
	if !bytes.Equal(r, want) {
		t.Errorf("read %X, want %X", r, want)
	}

	cmds := dev.i2cCommands()
	wantCmds := []byte{cmdI2CWriteNoStop, cmdI2CReadRepStart, cmdI2CReadGetData}
	if !bytes.Equal(cmds, wantCmds) {
		t.Errorf("command sequence %X, want %X (write-no-stop, then repeated-START read)", cmds, wantCmds)
	}
}

func TestTxPureReadUsesPlainStart(t *testing.T) {
	i2c, dev := newTestI2C([]byte{0x42})

	r := make([]byte, 1)
	if err := i2c.Tx(0x36, nil, r); err != nil {
		t.Fatalf("Tx(): %v", err)
	}

	if cmds := dev.i2cCommands(); cmds[0] != cmdI2CRead {
		t.Errorf("bare read issued command 0x%02X, want plain-START read 0x%02X", cmds[0], cmdI2CRead)
	}
}

func TestTxPureWriteUsesStop(t *testing.T) {
	i2c, dev := newTestI2C(nil)

	if err := i2c.Tx(0x36, []byte{0x19, 0xFF}, nil); err != nil {
		t.Fatalf("Tx(): %v", err)
	}

	cmds := dev.i2cCommands()
	if !bytes.Equal(cmds, []byte{cmdI2CWrite}) {
		t.Errorf("command sequence %X, want a single write-with-STOP 0x%02X", cmds, cmdI2CWrite)
	}
}

func TestTxAddressing(t *testing.T) {
	i2c, dev := newTestI2C([]byte{0x00})

	if err := i2c.Tx(0x36, []byte{0x19}, make([]byte, 1)); err != nil {
		t.Fatalf("Tx(): %v", err)
	}

	for _, report := range dev.reports {
		switch report[0] {
		case cmdI2CWriteNoStop:
			if report[3] != 0x36<<1 {
				t.Errorf("write address byte 0x%02X, want 0x%02X", report[3], 0x36<<1)
			}
		case cmdI2CReadRepStart:
			if report[3] != 0x36<<1|1 {
				t.Errorf("read address byte 0x%02X, want 0x%02X", report[3], 0x36<<1|1)
			}
		}
	}
}

func TestTxRejects10BitAddress(t *testing.T) {
	i2c, dev := newTestI2C(nil)

	if err := i2c.Tx(0x1234, []byte{0x00}, nil); err == nil {
		t.Fatal("Tx() accepted a 10-bit address")
	}
	if len(dev.reports) != 0 {
		t.Errorf("bus traffic issued for invalid address: %d reports", len(dev.reports))
	}
}

func TestParseFlashString(t *testing.T) {
	msg := func(s string) []byte {
		b := make([]byte, MsgSz)
		u := utf16.Encode([]rune(s))
		b[2] = byte(2 + 2*len(u))
		for i, r := range u {
			b[4+2*i] = byte(r)
			b[4+2*i+1] = byte(r >> 8)
		}
		return b
	}

	if got := parseFlashString(msg("MCP2221 I²C ✓")); got != "MCP2221 I²C ✓" {
		t.Errorf("round-trip: %q", got)
	}
	if got := parseFlashString(msg("")); got != "" {
		t.Errorf("empty string: %q", got)
	}

	corrupt := make([]byte, MsgSz)
	corrupt[2] = 1 // length below the 2-byte header minimum
	if got := parseFlashString(corrupt); got != "" {
		t.Errorf("corrupt length byte: %q", got)
	}
}

func TestParseStatus(t *testing.T) {
	if parseStatus(nil) != nil || parseStatus(make([]byte, MsgSz-1)) != nil {
		t.Error("expected nil for missing or short message")
	}

	msg := make([]byte, MsgSz)
	msg[8] = i2cStateWritingNoStop
	msg[24] = 0x01                // interrupt flag
	msg[50], msg[51] = 0x34, 0x02 // ADC channel 0, little-endian

	stat := parseStatus(msg)
	if stat.i2cState != i2cStateWritingNoStop {
		t.Errorf("i2cState 0x%02X, want 0x%02X", stat.i2cState, i2cStateWritingNoStop)
	}
	if stat.interrupt == 0 {
		t.Error("interrupt flag not parsed")
	}
	if stat.adcChan[0] != 0x0234 {
		t.Errorf("adcChan[0] 0x%04X, want 0x0234", stat.adcChan[0])
	}
}

func TestUnlockFlags(t *testing.T) {
	for _, tc := range []struct {
		sec               ChipSecurity
		locked, writeable bool
	}{
		{SecUnsecured, false, true},
		{SecPassword, false, false},
		{SecLocked1, true, false},
		{SecLocked2, true, false},
	} {
		locked, writeable := unlockFlags(tc.sec)
		if locked != tc.locked || writeable != tc.writeable {
			t.Errorf("unlockFlags(%d) = (%t, %t), want (%t, %t)",
				tc.sec, locked, writeable, tc.locked, tc.writeable)
		}
	}
}
