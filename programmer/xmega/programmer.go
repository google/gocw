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

// Programs an XMEGA device using the NAEUSB firmware.
// Based on chipwhisperer/software/chipwhisperer/hardware/naeusb/programmer_xmega.py.
package xmega

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/google/gocw"

	"github.com/golang/glog"
)

// Implements programmer.ProgrammerInterface
type Programmer struct {
	dev  gocw.UsbDeviceInterface
	chip *ChipProperties
}

const (
	paramTimeout = 8

	// Chip erase types.
	eraseChip = 1
	eraseApp  = 2

	// Write memory flags.
	pageModeErase = 1 << 0
	pageModeWrite = 1 << 1

	signatureAddr = 0x01000090
	signatureSize = 3
)

type MemoryType uint8

const (
	// Memory types.
	MemTypeApp                MemoryType = 1
	MemTypeBoot               MemoryType = 2
	MemTypeEeprom             MemoryType = 3
	MemTypeFuse               MemoryType = 4
	MemTypeLockbits           MemoryType = 5
	MemTypeUsersig            MemoryType = 6
	MemTypeFactoryCalibration MemoryType = 7
)

type MemRegion struct {
	MemType MemoryType
	Offset  uint32
	Size    uint32
}

type ChipProperties struct {
	Name      string
	Signature [3]byte
	Flash     MemRegion
	Eeprom    MemRegion
}

var SupportedChips = map[string]ChipProperties{
	"XMEGA128D4": ChipProperties{
		"XMEGA128D4",              // name
		[3]byte{0x1e, 0x97, 0x47}, // signature
		MemRegion{ // flash
			MemTypeApp,
			0x0800000,
			0x00022000,
		},
		MemRegion{ // eeprom
			MemTypeEeprom,
			0x08c0000,
			0x0800,
		},
	},
}

//go:generate stringer -type Command
type Command uint16

const (
	CmdEnterProgmode Command = 1
	CmdLeaveProgmode Command = 2
	CmdErase         Command = 3
	CmdWriteMem      Command = 4
	CmdReadMem       Command = 5
	CmdCrc           Command = 6
	CmdSetParam      Command = 7
	CmdGetStatus     Command = 0x20
	CmdGetRamBuf     Command = 0x21
	CmdSetRamBuf     Command = 0x22
)

// low-level read command.
func (p *Programmer) doRead(cmd Command, data interface{}) error {
	var err error
	glog.V(1).Infof("[xmega-read]: cmd = %v", cmd)
	if err = p.dev.ControlIn(gocw.ReqXmegaProgram, uint16(cmd), data); err != nil {
		return fmt.Errorf("ReqXmegaProgram: %v", err)
	}
	return nil
}

// low-level write command.
func (p *Programmer) doWrite(cmd Command, data interface{}, checkStatus bool) error {
	var err error
	glog.V(1).Infof("[xmega-write]: cmd = %v", cmd)
	if err = p.dev.ControlOut(gocw.ReqXmegaProgram, uint16(cmd), data); err != nil {
		return fmt.Errorf("ReqXmegaProgram: %v", err)
	}
	if checkStatus {
		if err = p.checkStatusOk(); err != nil {
			return fmt.Errorf("Status failed: %v", err)
		}
	}
	return nil
}

type status struct {
	Command uint8
	Error   uint8
	Timeout uint8
}

func (p *Programmer) readStatus(status *status) error {
	return p.doRead(CmdGetStatus, status)
}

func (p *Programmer) checkStatusOk() error {
	var err error
	status := status{}
	if err = p.readStatus(&status); err != nil {
		return fmt.Errorf("readStatus: %v", err)
	}
	if status.Error != 0 {
		return fmt.Errorf("cmd failed with status %v", status)
	}
	return nil
}

func (p *Programmer) setTimeout(d time.Duration) error {
	type paramBlock struct {
		param uint8
		value uint32
	}
	param := paramBlock{
		paramTimeout,
		uint32(d.Seconds() * 2500), // timeout in ticks.
	}
	if err := p.doWrite(CmdSetParam, &param, true); err != nil {
		return fmt.Errorf("CmdSetParam failed: %v", err)
	}
	return nil
}

// Enter programming mode.
func (p *Programmer) enablePDI() error {
	if err := p.doWrite(CmdEnterProgmode, []byte{}, true); err != nil {
		return fmt.Errorf("CmdEnterProgmode failed: %v", err)
	}
	return nil
}

// Leave programming mode.
func (p *Programmer) disablePDI() error {
	if err := p.doWrite(CmdLeaveProgmode, []byte{}, true); err != nil {
		return fmt.Errorf("CmdLeaveProgmode failed: %v", err)
	}
	return nil
}

// Reads from FLASH/EEPROM memory.
// Implements io.Reader.
type memReader struct {
	prog      *Programmer
	addr      uint32
	chunkSize int
}

func (r *memReader) Read(p []byte) (n int, err error) {
	type infoBlock struct {
		typ  uint8
		addr uint32
		dlen uint16
	}

	// Read memory in small chunks.
	for n < len(p) {
		toRead := len(p) - n
		if toRead > r.chunkSize {
			toRead = r.chunkSize
		}

		info := infoBlock{}
		info.typ = 0
		info.addr = r.addr
		info.dlen = uint16(toRead)

		if err = r.prog.doWrite(CmdReadMem, &info, true); err != nil {
			return n, fmt.Errorf("CmdReadMem failed: %v", err)
		}

		if err = r.prog.doRead(CmdGetRamBuf, p[n:n+toRead]); err != nil {
			return n, fmt.Errorf("CmdGetRamBuf failed: %v", err)
		}

		n += toRead
		r.addr += uint32(toRead)
	}

	return n, nil
}

func (p *Programmer) NewMemoryReader(addr uint32) io.Reader {
	if p.chip != nil {
		addr = p.chip.Flash.Offset
	}
	return &memReader{p, addr, 64}
}

// Writes to FLASH/EEPROM memory.
// Implements io.Writer.
type memWriter struct {
	prog      *Programmer
	memType   MemoryType
	addr      uint32
	maxAddr   uint32
	chunkSize int
}

func (w *memWriter) Write(p []byte) (n int, err error) {
	type infoBlock struct {
		typ   uint8
		flags uint8
		addr  uint32
		dlen  uint16
	}

	// Write memory in small chunks.
	for n < len(p) {
		toWrite := len(p) - n
		if toWrite > w.chunkSize {
			toWrite = w.chunkSize
		}

		if w.addr+uint32(toWrite) > w.maxAddr {
			return n, io.ErrShortWrite
		}

		info := infoBlock{}
		info.typ = uint8(w.memType)
		info.flags = pageModeWrite
		info.addr = w.addr
		info.dlen = uint16(toWrite)

		if err = w.prog.doWrite(CmdSetRamBuf, p[n:n+toWrite], false); err != nil {
			return n, fmt.Errorf("CmdSetRamBuf failed: %v", err)
		}

		if err = w.prog.doWrite(CmdWriteMem, &info, true); err != nil {
			return n, fmt.Errorf("CmdWriteMem failed: %v", err)
		}

		n += toWrite
		w.addr += uint32(toWrite)
	}
	return n, nil
}

func (p *Programmer) NewMemoryWriter(addr uint32) io.Writer {
	region := p.chip.Flash
	return &memWriter{p, region.MemType, region.Offset, region.Offset + region.Size, 64}
}

func (p *Programmer) findChip() (*ChipProperties, error) {
	r := p.NewMemoryReader(signatureAddr)
	sig := make([]byte, signatureSize)
	if _, err := r.Read(sig); err != nil {
		return nil, fmt.Errorf("Failed to read signature: %v", err)
	}

	for _, chip := range SupportedChips {
		if bytes.Equal(chip.Signature[:], sig) {
			return &chip, nil
		}
	}

	return nil, fmt.Errorf("Unsupported chip. Signature: %v", sig)
}

// Takes ownership of dev: programmer closes dev on Close().
func NewProgrammerDeps(dev gocw.UsbDeviceInterface) (*Programmer, error) {
	var err error
	p := &Programmer{dev, nil}
	if err = p.setTimeout(400 * time.Millisecond); err != nil {
		return nil, fmt.Errorf("setTimeout failed: %v", err)
	}

	if err = p.enablePDI(); err != nil {
		return nil, fmt.Errorf("enablePDI failed: %v", err)
	}

	if p.chip, err = p.findChip(); err != nil {
		p.Close()
		return nil, fmt.Errorf("Failed to find chip: %v", err)
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
	return NewProgrammerDeps(dev)
}

func (p *Programmer) Close() error {
	var err error
	if p.dev != nil {
		err = p.disablePDI()
		p.dev.Close()
		p.dev = nil
	}
	return err
}

func (p *Programmer) EraseChip() error {
	if err := p.doWrite(CmdErase, []byte{eraseChip, 0, 0, 0, 0}, true); err != nil {
		return fmt.Errorf("EraseChip failed: %v", err)
	}
	return nil
}

func (p *Programmer) EraseApp() error {
	if err := p.doWrite(CmdErase, []byte{eraseApp, 0, 0, 0, 0}, true); err != nil {
		return fmt.Errorf("EraseApp failed: %v", err)
	}
	return nil
}

func (p *Programmer) Erase() error {
	var err error
	glog.Info("Erasing chip")
	if err = p.EraseChip(); err != nil {
		p.disablePDI()
		p.enablePDI()
		glog.Info("Erasing app")
		if err = p.EraseApp(); err != nil {
			return fmt.Errorf("Failed to erase chip before program: %v", err)
		}
	}
	return nil
}
