// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Programs a STM32 device using the NAEUSB firmware.
// Based on chipwhisperer/software/chipwhisperer/hardware/naeusb/programmer_stm32fserial.py.
package stm32f

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"gocw"

	"github.com/golang/glog"
)

// Implements programmer.ProgrammerInterface
type Programmer struct {
	dev      gocw.UsbDeviceInterface
	adc      gocw.AdcInterface
	ser      gocw.UsartInterface
	commands map[byte]bool // supported commands.
	chip     *ChipProperties
}

type ChipProperties struct {
	Name      string
	Signature [2]byte
}

var SupportedChips = map[string]ChipProperties{
	"STM32F303cBC": ChipProperties{
		"STM32F303cBC",      // name
		[2]byte{0x04, 0x22}, // signature
	},
}

//go:generate stringer -type Command
type Command uint8

const (
	CmdGetAvailableCommands Command = 0x00
	CmdGetId                Command = 0x02
	CmdReadMemory           Command = 0x11
	CmdWriteMemory          Command = 0x31
	CmdEraseMemory          Command = 0x43
	CmdExtendedEraseMemory  Command = 0x44
)

func (p *Programmer) setBoot(enterBootLoader bool) {
	if enterBootLoader {
		p.adc.SetPDIC(gocw.GpioHigh)
	} else {
		p.adc.SetPDIC(gocw.GpioLow)
	}
}

func (p *Programmer) reset() {
	p.adc.SetNRST(gocw.GpioLow)
	time.Sleep(10 * time.Millisecond)
	p.adc.SetNRST(gocw.GpioHigh)
	time.Sleep(25 * time.Millisecond)
}

func (p *Programmer) waitForAck() error {
	res := make([]byte, 1)
	n, err := p.ser.Read(res)
	if err != nil {
		return fmt.Errorf("Read failed with %v", err)
	}
	if n == 0 {
		return fmt.Errorf("Read ack timed out")
	}
	switch res[0] {
	case 0x79:
		// ACK
		return nil
	case 0x1F:
		// NACK
		return fmt.Errorf("Target returned NACK")
	default:
		return fmt.Errorf("Unknown response %02x", res[0])
	}
}

func (p *Programmer) initChip() error {
	glog.V(1).Info("Initializing chip")
	p.setBoot(true)
	for fails := 0; fails < 5; fails++ {
		// First 2-times, try resetting. After that don't in case reset is causing garbage on lines.
		if fails < 2 {
			p.reset()
		}

		p.ser.Flush()
		p.ser.Write([]byte{'\x7F'})
		err := p.waitForAck()
		if err == nil {
			return nil
		}
		glog.Warningf("Sync failed with err: %v", err)
	}

	return fmt.Errorf("Could not detect STM32F")
}

func (p *Programmer) releaseChip() {
	glog.V(1).Info("Releasing chip")
	p.setBoot(false)
	p.reset()
}

func (p *Programmer) cmdGeneric(cmd Command) error {
	glog.V(2).Infof("Executing command %v", cmd)
	p.ser.Write([]byte{byte(cmd)})
	p.ser.Write([]byte{byte(cmd) ^ 0xFF}) // control byte
	return p.waitForAck()
}

func (p *Programmer) cmdGetAvailableCommands() error {
	var err error
	if err = p.cmdGeneric(CmdGetAvailableCommands); err != nil {
		return fmt.Errorf("CmdGetAvailableCommands failed: %v", err)
	}
	glog.V(1).Infof("*** Get command")
	l := make([]byte, 1)
	if _, err = p.ser.Read(l); err != nil {
		return fmt.Errorf("Failed reading len %v", err)
	}
	ver := make([]byte, 1)
	if _, err = p.ser.Read(ver); err != nil {
		return fmt.Errorf("Failed reading version %v", err)
	}
	commands := make([]byte, l[0])
	if _, err = p.ser.Read(commands); err != nil {
		return fmt.Errorf("Failed reading commands %v", err)
	}
	if err = p.waitForAck(); err != nil {
		return fmt.Errorf("Ack failed %v", err)
	}
	for _, c := range commands {
		p.commands[c] = true
	}
	glog.V(1).Infof("Bootloader version: %v", ver[0])
	glog.V(1).Infof("Available commands: %v", commands)
	return nil
}

func (p *Programmer) cmdGetId() ([]byte, error) {
	var err error
	if err = p.cmdGeneric(CmdGetId); err != nil {
		return nil, fmt.Errorf("CmdGetId failed: %v", err)
	}
	glog.V(1).Infof("*** GetID command")
	l := make([]byte, 1)
	if _, err = p.ser.Read(l); err != nil {
		return nil, fmt.Errorf("Failed reading len %v", err)
	}
	id := make([]byte, l[0]+1)
	if _, err = p.ser.Read(id); err != nil {
		return nil, fmt.Errorf("Failed reading id %v", err)
	}
	if err = p.waitForAck(); err != nil {
		return nil, fmt.Errorf("Ack failed %v", err)
	}
	return id, nil
}

func (p *Programmer) cmdExtendedEraseMemory() error {
	var err error
	if err = p.cmdGeneric(CmdExtendedEraseMemory); err != nil {
		return fmt.Errorf("CmdExtendedEraseMemory failed: %v", err)
	}
	glog.V(1).Infof("*** Extended erase memory command")
	// Global mass erase
	p.ser.Write([]byte{0xff, 0xff})
	// Checksum
	p.ser.Write([]byte{0x00})

	t := p.ser.Timeout()
	defer p.ser.SetTimeout(t)

	glog.Infof("Extended erase, this can take a few seconds...")
	p.ser.SetTimeout(30 * time.Second)
	return p.waitForAck()
}

func (p *Programmer) cmdEraseMemory() error {
	if _, ok := p.commands[0x44]; ok {
		return p.cmdExtendedEraseMemory()
	}
	var err error
	if err = p.cmdGeneric(CmdEraseMemory); err != nil {
		return fmt.Errorf("CmdEraseMemory failed: %v", err)
	}
	glog.V(1).Infof("*** Extended memory command")
	// Global erase
	p.ser.Write([]byte{0xff, 0x00})
	return p.waitForAck()
}

func encodeAddr(addr uint32) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, addr)

	var crc byte
	for _, b := range buf.Bytes() {
		crc ^= b
	}

	buf.WriteByte(crc)
	return buf.Bytes()
}

func (p *Programmer) cmdWriteMemory(addr uint32, data []byte) error {
	var toWrite []byte
	if len(data)%4 > 0 {
		// Copy of data, with padding bytes.
		toWrite = make([]byte, len(data))
		copy(toWrite, data)
		for len(toWrite)%4 > 0 {
			toWrite = append(toWrite, 0xff)
		}
	} else {
		toWrite = data
	}
	var err error
	if err = p.cmdGeneric(CmdWriteMemory); err != nil {
		return fmt.Errorf("CmdWriteMemory failed: %v", err)
	}
	glog.V(2).Infof("*** Write memory command")
	p.ser.Write(encodeAddr(addr))
	if err = p.waitForAck(); err != nil {
		return fmt.Errorf("Write addr failed: %v", err)
	}
	n := byte(len(toWrite) - 1)
	p.ser.Write([]byte{n})

	crc := n
	for _, c := range toWrite {
		p.ser.Write([]byte{c})
		crc ^= c
	}
	p.ser.Write([]byte{crc})
	if err = p.waitForAck(); err != nil {
		return fmt.Errorf("Write failed: %v", err)
	}
	return nil
}

func (p *Programmer) cmdReadMemory(addr uint32, data []byte) error {
	var err error
	if err = p.cmdGeneric(CmdReadMemory); err != nil {
		return fmt.Errorf("CmdReadMemory failed: %v", err)
	}
	glog.V(2).Infof("*** Read memory command")
	p.ser.Write(encodeAddr(addr))
	if err = p.waitForAck(); err != nil {
		return fmt.Errorf("Read addr failed: %v", err)
	}
	n := byte(len(data) - 1)
	crc := n ^ 0xff
	p.ser.Write([]byte{n, crc})
	if err = p.waitForAck(); err != nil {
		return fmt.Errorf("Read len failed: %v", err)
	}
	if _, err = p.ser.Read(data); err != nil {
		return fmt.Errorf("Read data failed: %v", err)
	}
	return nil
}

// Writes to FLASH/EEPROM memory.
type memWriter struct {
	prog      *Programmer
	addr      uint32
	blockSize int
}

func (w *memWriter) Write(p []byte) (n int, err error) {
	// Write memory in small chunks.
	for n < len(p) {
		toWrite := len(p) - n
		if toWrite > w.blockSize {
			toWrite = w.blockSize
		}

		if err = w.prog.cmdWriteMemory(w.addr, p[n:n+toWrite]); err != nil {
			return n, fmt.Errorf("cmdWriteMemory failed: %v", err)
		}

		n += toWrite
		w.addr += uint32(toWrite)
	}
	return n, err
}

func (p *Programmer) NewMemoryWriter(addr uint32) io.Writer {
	return &memWriter{p, addr, 64}
}

// Reads from FLASH/EEPROM memory.
type memReader struct {
	prog      *Programmer
	addr      uint32
	blockSize int
}

func (r *memReader) Read(p []byte) (n int, err error) {
	// Read memory in small chunks.
	for n < len(p) {
		toRead := len(p) - n
		if toRead > r.blockSize {
			toRead = r.blockSize
		}

		if err = r.prog.cmdReadMemory(r.addr, p[n:n+toRead]); err != nil {
			return n, fmt.Errorf("cmdReadMemory failed: %v", err)
		}

		n += toRead
		r.addr += uint32(toRead)
	}
	return n, nil
}

func (p *Programmer) NewMemoryReader(addr uint32) io.Reader {
	return &memReader{p, addr, 64}
}

func (p *Programmer) findChip() (*ChipProperties, error) {
	var err error
	if err = p.initChip(); err != nil {
		p.releaseChip()
		return nil, fmt.Errorf("initChip failed: %v", err)
	}
	if err = p.cmdGetAvailableCommands(); err != nil {
		p.releaseChip()
		return nil, fmt.Errorf("cmdGet failed: %v", err)
	}
	var id []byte
	if id, err = p.cmdGetId(); err != nil {
		p.releaseChip()
		return nil, fmt.Errorf("cmdGetId failed: %v", err)
	}

	for _, chip := range SupportedChips {
		if bytes.Equal(chip.Signature[:], id) {
			return &chip, nil
		}
	}

	p.releaseChip()
	return nil, fmt.Errorf("Unsupported chip. Signature: %v", id)
}

// Takes ownership of dev, adc: programmer closes dev, adc on Close().
func NewProgrammerDeps(dev gocw.UsbDeviceInterface, adc gocw.AdcInterface,
	ser gocw.UsartInterface) (*Programmer, error) {
	var err error
	p := &Programmer{dev, adc, ser, make(map[byte]bool), nil}

	if p.chip, err = p.findChip(); err != nil {
		return nil, fmt.Errorf("findChip failed: %v", err)
	}

	glog.V(1).Infof("Found supported chip %v", p.chip.Name)
	return p, nil
}

func NewProgrammer() (*Programmer, error) {
	var err error
	var dev gocw.UsbDeviceInterface
	if dev, err = gocw.OpenCwLiteUsbDevice(); err != nil {
		return nil, err
	}
	var fpga *gocw.Fpga
	if fpga, err = gocw.NewFpga(dev); err != nil {
		dev.Close()
		return nil, fmt.Errorf("NewFpga failed: %v", err)
	}

	var adc *gocw.Adc
	if adc, err = gocw.NewAdc(fpga); err != nil {
		dev.Close()
		return nil, fmt.Errorf("NewAdc failed: %v", err)
	}

	var ser *gocw.Usart
	if ser, err = gocw.NewUsart(dev,
		&gocw.UsartConfig{
			gocw.BaudRateLow, gocw.StopBitsOne, gocw.ParityEven, gocw.DataBitsOneByte}); err != nil {
		adc.Close()
		dev.Close()
		return nil, fmt.Errorf("NewUsart failed: %v", err)
	}

	return NewProgrammerDeps(dev, adc, ser)
}

func (p *Programmer) Close() error {
	if p.chip != nil {
		p.releaseChip()
	}
	if p.adc != nil {
		p.adc.Close()
	}
	if p.dev != nil {
		p.dev.Close()
	}
	return nil
}

func (p *Programmer) Erase() error {
	return p.cmdEraseMemory()
}
