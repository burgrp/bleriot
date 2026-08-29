package mcp2210

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

type ioResult struct {
	n   int
	err error
	buf []byte
}

type scriptedHID struct {
	writes     [][]byte
	writeSteps []ioResult
	readSteps  []ioResult
	closeCalls int
	closeErr   error
}

func (hid *scriptedHID) Write(p []byte) (int, error) {
	hid.writes = append(hid.writes, append([]byte(nil), p...))
	if len(hid.writeSteps) == 0 {
		return len(p), nil
	}
	step := hid.writeSteps[0]
	hid.writeSteps = hid.writeSteps[1:]
	return step.n, step.err
}

func (hid *scriptedHID) Read(p []byte) (int, error) {
	if len(hid.readSteps) == 0 {
		return 0, io.EOF
	}
	step := hid.readSteps[0]
	hid.readSteps = hid.readSteps[1:]
	copy(p, step.buf)
	return step.n, step.err
}

func (hid *scriptedHID) Close() error {
	hid.closeCalls++
	return hid.closeErr
}

func responseStep(opcode byte) ioResult {
	buf := make([]byte, reportLen)
	buf[0] = opcode
	return ioResult{n: reportLen, buf: buf}
}

func TestCommandWritesNumberedReportAndReadsMatchingResponse(t *testing.T) {
	hid := &scriptedHID{readSteps: []ioResult{responseStep(cmdSPITransfer)}}
	d := &Device{f: hid}
	var req [reportLen]byte
	req[0], req[1], req[63] = cmdSPITransfer, 2, 0xA5

	resp, err := d.command(req)
	if err != nil {
		t.Fatalf("command: %v", err)
	}
	if resp[0] != req[0] {
		t.Fatalf("response opcode = %#x, want %#x", resp[0], req[0])
	}
	if len(hid.writes) != 1 || len(hid.writes[0]) != reportLen+1 {
		t.Fatalf("writes = %d reports, first length %d; want one %d-byte report", len(hid.writes), len(hid.writes[0]), reportLen+1)
	}
	if hid.writes[0][0] != 0 || !bytes.Equal(hid.writes[0][1:], req[:]) {
		t.Fatalf("written report = % X, want report ID 0 followed by % X", hid.writes[0], req)
	}
}

func TestCommandDrainsStaleResponses(t *testing.T) {
	hid := &scriptedHID{readSteps: []ioResult{responseStep(cmdSetGPIOValues), responseStep(cmdSPITransfer)}}
	d := &Device{f: hid}
	var req [reportLen]byte
	req[0] = cmdSPITransfer
	if _, err := d.command(req); err != nil {
		t.Fatalf("command after stale response: %v", err)
	}
}

func TestCommandStaleDrainBoundary(t *testing.T) {
	var req [reportLen]byte
	req[0] = cmdSPITransfer

	t.Run("last permitted stale then match", func(t *testing.T) {
		steps := make([]ioResult, maxStaleDrain)
		for i := 0; i < maxStaleDrain-1; i++ {
			steps[i] = responseStep(cmdSetGPIOValues)
		}
		steps[len(steps)-1] = responseStep(cmdSPITransfer)
		d := &Device{f: &scriptedHID{readSteps: steps}}
		if _, err := d.command(req); err != nil {
			t.Fatalf("command at stale boundary: %v", err)
		}
	})

	t.Run("too many stale responses", func(t *testing.T) {
		steps := make([]ioResult, maxStaleDrain)
		for i := range steps {
			steps[i] = responseStep(cmdSetGPIOValues)
		}
		d := &Device{f: &scriptedHID{readSteps: steps}}
		_, err := d.command(req)
		if err == nil || !strings.Contains(err.Error(), "response opcode") {
			t.Fatalf("command error = %v, want stale opcode error", err)
		}
	})
}

func TestCommandIOErrors(t *testing.T) {
	writeErr := errors.New("write failed")
	readErr := errors.New("read failed")
	tests := []struct {
		name string
		hid  *scriptedHID
		want string
		is   error
	}{
		{name: "short write", hid: &scriptedHID{writeSteps: []ioResult{{n: reportLen}}}, want: "short write: 64 bytes"},
		{name: "write error", hid: &scriptedHID{writeSteps: []ioResult{{err: writeErr}}}, want: "write report", is: writeErr},
		{name: "short read", hid: &scriptedHID{readSteps: []ioResult{{n: reportLen - 1, buf: make([]byte, reportLen-1)}}}, want: "short response: 63 bytes"},
		{name: "read error", hid: &scriptedHID{readSteps: []ioResult{{err: readErr}}}, want: "read report", is: readErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Device{f: tt.hid}
			_, err := d.command([reportLen]byte{cmdSPITransfer})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("command error = %v, want containing %q", err, tt.want)
			}
			if tt.is != nil && !errors.Is(err, tt.is) {
				t.Fatalf("command error = %v, want wrapped %v", err, tt.is)
			}
		})
	}
}

func TestCommandTimesOutOnWouldBlock(t *testing.T) {
	tests := []struct {
		name string
		hid  *scriptedHID
	}{
		{name: "write", hid: &scriptedHID{writeSteps: []ioResult{{err: syscall.EAGAIN}}}},
		{name: "read", hid: &scriptedHID{readSteps: []ioResult{{err: syscall.EAGAIN}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Device{f: tt.hid, commandTimeout: time.Nanosecond}
			if _, err := d.command([reportLen]byte{cmdSPITransfer}); !errors.Is(err, ErrCommandTimeout) {
				t.Fatalf("command error = %v, want ErrCommandTimeout", err)
			}
		})
	}
}

func TestClose(t *testing.T) {
	d := &Device{}
	if err := d.Close(); err != nil {
		t.Fatalf("Close with nil handle: %v", err)
	}
	closeErr := errors.New("close failed")
	hid := &scriptedHID{closeErr: closeErr}
	d.f = hid
	if err := d.Close(); !errors.Is(err, closeErr) {
		t.Fatalf("Close error = %v, want %v", err, closeErr)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if hid.closeCalls != 1 {
		t.Fatalf("underlying Close calls = %d, want 1", hid.closeCalls)
	}
}

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

func TestTransferProgramsSizeOnlyWhenItChangesAndUsesDefaultConfig(t *testing.T) {
	d := &Device{lastTxBytes: -1}
	var requests [][reportLen]byte
	d.commandHook = func(req [reportLen]byte) ([reportLen]byte, error) {
		requests = append(requests, req)
		var resp [reportLen]byte
		resp[0] = req[0]
		if req[0] == cmdSPITransfer {
			resp[2] = req[1]
			copy(resp[4:], req[4:4+int(req[1])])
		}
		return resp, nil
	}

	for range 2 {
		got, err := d.Transfer([]byte{0x11, 0x22})
		if err != nil {
			t.Fatalf("Transfer: %v", err)
		}
		if !bytes.Equal(got, []byte{0x11, 0x22}) {
			t.Fatalf("Transfer = % X, want 11 22", got)
		}
	}
	if len(requests) != 3 || requests[0][0] != cmdSetSPISettings || requests[1][0] != cmdSPITransfer || requests[2][0] != cmdSPITransfer {
		t.Fatalf("request opcodes = %#x %#x %#x, want settings then two transfers", requests[0][0], requests[1][0], requests[2][0])
	}
	wantSettings := buildSPISettings(DefaultSPIConfig, 2)
	if requests[0] != wantSettings {
		t.Fatalf("fallback SPI settings differ: got % X want % X", requests[0], wantSettings)
	}
}

func TestTransferAccumulatesReportsWithDrainRequest(t *testing.T) {
	d := &Device{lastTxBytes: 4}
	var requests [][reportLen]byte
	d.commandHook = func(req [reportLen]byte) ([reportLen]byte, error) {
		requests = append(requests, req)
		var resp [reportLen]byte
		resp[0] = req[0]
		resp[2] = 2
		if len(requests) == 1 {
			resp[4], resp[5] = 1, 2
		} else {
			resp[4], resp[5] = 3, 4
		}
		return resp, nil
	}

	got, err := d.Transfer([]byte{9, 8, 7, 6})
	if err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	if !bytes.Equal(got, []byte{1, 2, 3, 4}) {
		t.Fatalf("Transfer = %v, want [1 2 3 4]", got)
	}
	if len(requests) != 2 || requests[0][1] != 4 || requests[1][1] != 0 {
		t.Fatalf("transfer request lengths = %d, %d; want 4 then drain 0", requests[0][1], requests[1][1])
	}
}

func TestTransferOKWithNoDataStalls(t *testing.T) {
	d := &Device{lastTxBytes: 1}
	commands := 0
	d.commandHook = func(req [reportLen]byte) ([reportLen]byte, error) {
		commands++
		return [reportLen]byte{cmdSPITransfer, spiStatusOK}, nil
	}
	if _, err := d.Transfer([]byte{1}); !errors.Is(err, ErrTransferStalled) {
		t.Fatalf("Transfer error = %v, want ErrTransferStalled", err)
	}
	if commands != maxNoProgressPolls {
		t.Fatalf("commands = %d, want %d", commands, maxNoProgressPolls)
	}
}

func TestTransferErrors(t *testing.T) {
	issueErr := errors.New("issue failed")
	tests := []struct {
		name        string
		lastTxBytes int
		response    [reportLen]byte
		issueErr    error
		want        string
		is          error
	}{
		{name: "settings issue", lastTxBytes: -1, issueErr: issueErr, is: issueErr},
		{name: "transfer issue", lastTxBytes: 1, issueErr: issueErr, is: issueErr},
		{name: "invalid status", lastTxBytes: 1, response: [reportLen]byte{cmdSPITransfer, 0x55}, want: "status 0x55"},
		{name: "invalid count", lastTxBytes: 1, response: [reportLen]byte{cmdSPITransfer, spiStatusOK, maxTransfer + 1}, want: "reports 61 received bytes"},
		{name: "bus busy", lastTxBytes: 1, response: [reportLen]byte{cmdSPITransfer, spiStatusBusBusy}, is: ErrBusBusy},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Device{lastTxBytes: tt.lastTxBytes}
			d.commandHook = func(req [reportLen]byte) ([reportLen]byte, error) {
				if tt.issueErr != nil {
					return [reportLen]byte{}, tt.issueErr
				}
				return tt.response, nil
			}
			_, err := d.Transfer([]byte{1})
			if tt.is != nil && !errors.Is(err, tt.is) {
				t.Fatalf("Transfer error = %v, want %v", err, tt.is)
			}
			if tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
				t.Fatalf("Transfer error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestTransferRejectsBoundsWithoutIssuing(t *testing.T) {
	d := &Device{commandHook: func([reportLen]byte) ([reportLen]byte, error) {
		t.Fatal("invalid transfer issued a command")
		return [reportLen]byte{}, nil
	}}
	for _, size := range []int{0, maxTransfer + 1} {
		if _, err := d.Transfer(make([]byte, size)); err == nil {
			t.Errorf("Transfer length %d succeeded", size)
		}
	}
}

func TestConfigureDefaultsAndInitializesState(t *testing.T) {
	d := &Device{lastTxBytes: -1, gpioValues: 0xFFFF}
	var requests [][reportLen]byte
	d.commandHook = func(req [reportLen]byte) ([reportLen]byte, error) {
		requests = append(requests, req)
		return [reportLen]byte{req[0]}, nil
	}
	if err := d.Configure(SPIConfig{}, 1, 8); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if len(requests) != 2 || requests[0] != buildChipSettings(1<<1|1<<8) || requests[1] != buildSPISettings(DefaultSPIConfig, 1) {
		t.Fatalf("Configure requests = % X", requests)
	}
	if d.cfg != DefaultSPIConfig || d.lastTxBytes != 1 || d.gpioValues != 0 {
		t.Fatalf("configured state = cfg %+v size %d GPIO %#x", d.cfg, d.lastTxBytes, d.gpioValues)
	}
}

func TestConfigureRejectsPinsAndPropagatesFailures(t *testing.T) {
	for _, pin := range []uint8{0, 9} {
		t.Run(fmt.Sprintf("pin_%d", pin), func(t *testing.T) {
			d := &Device{commandHook: func([reportLen]byte) ([reportLen]byte, error) {
				t.Fatal("invalid pin issued a command")
				return [reportLen]byte{}, nil
			}}
			if err := d.Configure(DefaultSPIConfig, pin); err == nil {
				t.Fatalf("Configure pin %d succeeded", pin)
			}
		})
	}
	issueErr := errors.New("issue failed")
	for _, failCall := range []int{1, 2} {
		t.Run(fmt.Sprintf("issue_%d", failCall), func(t *testing.T) {
			calls := 0
			d := &Device{commandHook: func(req [reportLen]byte) ([reportLen]byte, error) {
				calls++
				if calls == failCall {
					return [reportLen]byte{}, issueErr
				}
				return [reportLen]byte{req[0]}, nil
			}}
			if err := d.Configure(DefaultSPIConfig); !errors.Is(err, issueErr) {
				t.Fatalf("Configure error = %v, want %v", err, issueErr)
			}
			if calls != failCall {
				t.Fatalf("issue calls = %d, want %d", calls, failCall)
			}
		})
	}
}

func TestSetGPIOPreservesPinsAndPropagatesErrors(t *testing.T) {
	d := &Device{}
	var requests [][reportLen]byte
	issueErr := errors.New("issue failed")
	d.commandHook = func(req [reportLen]byte) ([reportLen]byte, error) {
		requests = append(requests, req)
		if len(requests) == 3 {
			return [reportLen]byte{}, issueErr
		}
		return [reportLen]byte{req[0]}, nil
	}
	if err := d.SetGPIO(1, true); err != nil {
		t.Fatal(err)
	}
	if err := d.SetGPIO(2, true); err != nil {
		t.Fatal(err)
	}
	if err := d.SetGPIO(1, false); !errors.Is(err, issueErr) {
		t.Fatalf("SetGPIO error = %v, want %v", err, issueErr)
	}
	if requests[0] != buildGPIOValues(1<<1) || requests[1] != buildGPIOValues(1<<1|1<<2) || requests[2] != buildGPIOValues(1<<2) {
		t.Fatalf("GPIO requests did not preserve other pin values: % X", requests)
	}
	for _, pin := range []uint8{0, 9} {
		if err := d.SetGPIO(pin, true); err == nil {
			t.Errorf("SetGPIO pin %d succeeded", pin)
		}
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
