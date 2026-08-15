package mcp2210

import (
	"bytes"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestBuildChipSettings(t *testing.T) {
	req := buildChipSettings(0)
	if req[0] != cmdSetChipSettings {
		t.Fatalf("opcode = 0x%02X, want 0x%02X", req[0], cmdSetChipSettings)
	}
	if req[4] != gpDesignationChipSelect {
		t.Fatalf("GP0 designation = 0x%02X, want chip select 0x%02X", req[4], gpDesignationChipSelect)
	}
	for i := 5; i <= 12; i++ {
		if req[i] != 0 {
			t.Fatalf("GP%d designation = 0x%02X, want GPIO 0x00", i-4, req[i])
		}
	}
	if req[15] != 0xFF || req[16] != 0x01 {
		t.Fatalf("default direction = 0x%02X%02X, want all inputs 0x01FF", req[16], req[15])
	}
}

func TestBuildChipSettingsOutputs(t *testing.T) {
	// GP1 and GP2 as outputs: clear bits 1 and 2 from the all-inputs 0x01FF.
	req := buildChipSettings(1<<1 | 1<<2)
	if req[13] != 0 || req[14] != 0 {
		t.Fatalf("default output = 0x%02X%02X, want 0 (off)", req[14], req[13])
	}
	dir := uint16(req[15]) | uint16(req[16])<<8
	if dir != 0x01F9 {
		t.Fatalf("direction = 0x%04X, want 0x01F9 (GP1,GP2 outputs)", dir)
	}
}

func TestBuildGPIOValues(t *testing.T) {
	req := buildGPIOValues(1<<1 | 1<<2)
	if req[0] != cmdSetGPIOValues {
		t.Fatalf("opcode = 0x%02X, want 0x%02X", req[0], cmdSetGPIOValues)
	}
	if req[4] != 0x06 || req[5] != 0x00 {
		t.Fatalf("values LE = 0x%02X%02X, want 0x0006", req[5], req[4])
	}
}

func TestBuildSPISettings(t *testing.T) {
	req := buildSPISettings(SPIConfig{BitRateHz: 0x00112233, Mode: 2}, 0x0102)
	if req[0] != cmdSetSPISettings {
		t.Fatalf("opcode = 0x%02X", req[0])
	}
	if req[4] != 0x33 || req[5] != 0x22 || req[6] != 0x11 || req[7] != 0x00 {
		t.Fatalf("bit rate LE wrong: % X", req[4:8])
	}
	if req[8] != 0x01 || req[10] != 0x00 {
		t.Fatalf("CS idle/active = 0x%02X/0x%02X, want 0x01/0x00", req[8], req[10])
	}
	if req[18] != 0x02 || req[19] != 0x01 {
		t.Fatalf("bytes per tx LE = 0x%02X%02X, want 0x0102", req[19], req[18])
	}
	if req[20] != 2 {
		t.Fatalf("mode = %d, want 2", req[20])
	}
}

func TestBuildSPITransfer(t *testing.T) {
	req := buildSPITransfer([]byte{0xAA, 0xBB})
	if req[0] != cmdSPITransfer {
		t.Fatalf("opcode = 0x%02X", req[0])
	}
	if req[1] != 2 {
		t.Fatalf("length = %d, want 2", req[1])
	}
	if req[4] != 0xAA || req[5] != 0xBB {
		t.Fatalf("data = % X, want AA BB", req[4:6])
	}

	empty := buildSPITransfer(nil)
	if empty[1] != 0 {
		t.Fatalf("empty transfer length = %d, want 0", empty[1])
	}
}

func TestParseSPITransfer(t *testing.T) {
	var resp [reportLen]byte
	resp[0] = cmdSPITransfer
	resp[1] = spiStatusOK
	resp[2] = 3
	resp[4], resp[5], resp[6] = 0x01, 0x02, 0x03
	got, err := parseSPITransfer(resp)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !bytes.Equal(got, []byte{0x01, 0x02, 0x03}) {
		t.Fatalf("rx = % X, want 01 02 03", got)
	}
}

func TestParseSPITransferInProgress(t *testing.T) {
	var resp [reportLen]byte
	resp[1] = spiStatusInProgress
	if _, err := parseSPITransfer(resp); !errors.Is(err, errSPIInProgress) {
		t.Fatalf("err = %v, want errSPIInProgress", err)
	}
}

func TestParseSPITransferBusBusy(t *testing.T) {
	var resp [reportLen]byte
	resp[1] = spiStatusBusBusy
	if _, err := parseSPITransfer(resp); !errors.Is(err, ErrBusBusy) {
		t.Fatalf("err = %v, want ErrBusBusy", err)
	}
}

func TestTransferStalled(t *testing.T) {
	d := &Device{lastTxBytes: 1}
	commands := 0
	d.commandHook = func(req [reportLen]byte) ([reportLen]byte, error) {
		commands++
		var resp [reportLen]byte
		resp[0] = req[0]
		resp[1] = spiStatusInProgress
		return resp, nil
	}

	if _, err := d.Transfer([]byte{0x42}); !errors.Is(err, ErrTransferStalled) {
		t.Fatalf("Transfer error = %v, want ErrTransferStalled", err)
	}
	if commands != maxNoProgressPolls {
		t.Fatalf("commands = %d, want %d", commands, maxNoProgressPolls)
	}
}

func TestTransferResetsNoProgressAfterData(t *testing.T) {
	d := &Device{lastTxBytes: 2}
	commands := 0
	d.commandHook = func(req [reportLen]byte) ([reportLen]byte, error) {
		commands++
		var resp [reportLen]byte
		resp[0] = req[0]
		if commands == maxNoProgressPolls {
			resp[1] = spiStatusOK
			resp[2] = 2
			resp[4], resp[5] = 0xAA, 0xBB
		} else {
			resp[1] = spiStatusInProgress
		}
		return resp, nil
	}

	got, err := d.Transfer([]byte{1, 2})
	if err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	if !bytes.Equal(got, []byte{0xAA, 0xBB}) {
		t.Fatalf("Transfer = % X, want AA BB", got)
	}
}

type silentHID struct{}

func (silentHID) Write(p []byte) (int, error) { return len(p), nil }
func (silentHID) Read([]byte) (int, error)    { return 0, syscall.EAGAIN }
func (silentHID) Close() error                { return nil }

func TestCommandTimesOutWhenDeviceStopsResponding(t *testing.T) {
	d := &Device{f: silentHID{}, commandTimeout: 2 * time.Millisecond}
	var req [reportLen]byte
	req[0] = cmdSPITransfer
	if _, err := d.command(req); !errors.Is(err, ErrCommandTimeout) {
		t.Fatalf("command error = %v, want ErrCommandTimeout", err)
	}
}

func TestUeventMatches(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/uevent"
	content := "DRIVER=hidraw\nHID_ID=0003:000004D8:000000DE\nHID_NAME=MCP2210\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if !ueventMatches(path, VendorID, ProductID) {
		t.Fatal("expected match for MCP2210 uevent")
	}
	if ueventMatches(path, 0x1234, 0x5678) {
		t.Fatal("did not expect match for wrong VID/PID")
	}
}
