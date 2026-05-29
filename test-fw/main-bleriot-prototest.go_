//go:build tinygo

package main

import (
	"machine"
	"runtime"
	"time"

	"bleriot"

	"github.com/burgrp/tinygo-drivers/bb/spi"
	"github.com/burgrp/tinygo-drivers/pan211x"
)

const (
	pinLedRed   = machine.PB0
	pinLedGreen = machine.PB1

	// pinSpiSck  = machine.PA9  // SCK  → PAN211x pin 2
	// pinSpiData = machine.PA7  // DATA → PAN211x pin 3, bidirectional
	// pinSpiCsn  = machine.PA10 // CSN  → PAN211x pin 1, active-low
)

type Radio interface {
	SetChannel(channel uint8) error
	EnableRxAddress(index uint8, addr [4]byte) error
	Receive(buf []byte) (int, bool)
	Send(addr [4]byte, data []byte) error
}

type Device struct {
	myAddr     [4]byte
	peerAddr   [4]byte
	pinSpiSck  machine.Pin
	pinSpiData machine.Pin
	pinSpiCsn  machine.Pin
	led        machine.Pin
	radio      Radio
}

func (d *Device) initialize() {

	d.led.High()
	time.Sleep(500 * time.Millisecond)
	d.led.Low()

	radio := pan211x.NewDriverBLELongRange(pan211x.NewRegistersSPI(spi.NewMaster(d.pinSpiSck, d.pinSpiData), d.pinSpiCsn))

	must(radio.Init(pan211x.ConfigBLELongRange{
		PayloadLen:      bleriot.PacketLen,
		SerialInterface: pan211x.SerialInterfaceSPI3W,
		SpreadFactor:    pan211x.SpreadFactorS8,
	}))

	must(radio.SetChannel(11))
	must(radio.EnableRxAddress(0, d.myAddr))

	d.radio = radio

}

func (d *Device) startHub() {
	d.initialize()
}

type TestRegisters struct {
	registers map[RegisterId]Register
}

func (r *TestRegisters) GetRegisterById(id RegisterId) (Register, bool) {
	reg, ok := r.registers[id]
	return reg, ok
}

func (r *TestRegisters) PrintDescriptor(indent int) {
	for id, reg := range r.registers {
		println("Register ID:", id)
		reg.PrintDescriptor(indent + 2)
	}
}

type IntRegister struct {
	value int32
}

func (r *IntRegister) GetRawValue() int32 {
	return r.value
}

func (r *IntRegister) PrintDescriptor(indent int) {
	println("IntRegister with value", r.value)
}

func (d *Device) startNode() {
	d.initialize()

	registers := &TestRegisters{
		registers: map[RegisterId]Register{},
	}
	registers.registers[5] = &IntRegister{value: 42}

	node := NewNode(d.radio, registers, d.myAddr, [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10})

	must(node.run())
}

type Node struct {
	radio     Radio
	registers Registers
	address   [4]byte
	key       [4]uint32
}

type Register interface {
	GetRawValue() int32
	PrintDescriptor(indent int)
}

type RegisterId uint16

type Registers interface {
	GetRegisterById(id RegisterId) (Register, bool)
	PrintDescriptor(indent int)
}

func NewNode(radio Radio, registers Registers, address [4]byte, key [16]byte) *Node {

	n := &Node{
		radio:     radio,
		registers: registers,
		address:   address,
	}

	for i := range n.key {
		n.key[i] = uint32(key[i*4]) | uint32(key[i*4+1])<<8 |
			uint32(key[i*4+2])<<16 | uint32(key[i*4+3])<<24
	}

	return n
}

func (n *Node) run() error {

	println("--- bleriot-node")
	print("address: 0x")
	printAddress(n.address)
	println()

	print("key: \"")
	printKey(n.key)
	println("\"")

	n.registers.PrintDescriptor(0)

	println("...")
	return nil

}

func main() {
	println("BleRiot proto-test starting...")

	pinLedGreen.Configure(machine.PinConfig{Mode: machine.PinOutput})
	pinLedRed.Configure(machine.PinConfig{Mode: machine.PinOutput})

	hubAddr := [4]byte{0xCC, 0xA0, 0x00, 0x01}
	nodeAddr := [4]byte{0xCC, 0xA0, 0x00, 0x02}

	go (&Device{
		myAddr:     hubAddr,
		peerAddr:   nodeAddr,
		pinSpiSck:  machine.PA9,
		pinSpiData: machine.PA7,
		pinSpiCsn:  machine.PA10,
		led:        pinLedRed,
	}).startHub()

	go (&Device{
		myAddr:     nodeAddr,
		peerAddr:   hubAddr,
		pinSpiSck:  machine.PB4,
		pinSpiData: machine.PB3,
		pinSpiCsn:  machine.PB5,
		led:        pinLedGreen,
	}).startNode()

	for {
		printMemoryStats()
		time.Sleep(5000 * time.Millisecond)
	}

}

func printMemoryStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	println("RAM:", m.HeapAlloc, "/", m.HeapSys, "bytes, Mallocs:", m.Mallocs)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func printSpace(len int) {
	for i := 0; i < len; i++ {
		print(" ")
	}
}

var hexDigits = [16]string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "A", "B", "C", "D", "E", "F"}

func printAddress(addr [4]byte) {
	for _, b := range addr {
		h := b >> 4
		l := b & 0xF
		print(hexDigits[h])
		print(hexDigits[l])
	}
}

func printKey(key [4]uint32) {
	for _, k := range key {
		for i := 0; i < 4; i++ {
			b := byte(k >> (8 * uint(i)))
			h := b >> 4
			l := b & 0xF
			print(hexDigits[h])
			print(hexDigits[l])
		}
	}
}
