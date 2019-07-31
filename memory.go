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

// External memory interface.
package gocw

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/golang/glog"
)

type Address uint32

type Memory struct {
	dev UsbDeviceInterface
}

type AddressBlock struct {
	Dlen uint32
	Addr uint32
}

// Reads len(data) bytes from memory address addr.
// Automatically decides to use control-transfer or build-endpoint transfer
// based on data length.
func (m *Memory) doRead(addr Address, data []byte) error {
	var err error
	glog.V(1).Infof("[ext-mem-read]: addr = %v, dlen = %v", addr, len(data))

	cmd := ReqMemReadBulk
	if len(data) < 48 {
		cmd = ReqMemReadCtrl
	}

	info := AddressBlock{}
	info.Dlen = uint32(len(data))
	info.Addr = uint32(addr)

	if err = m.dev.ControlOut(cmd, 0, &info); err != nil {
		return fmt.Errorf("ControlOut AddressBlock failed: %v", err)
	}

	switch cmd {
	case ReqMemReadBulk:
		var n int
		if n, err = m.dev.Read(data); err != nil {
			return fmt.Errorf("ReqMemReadBulk data failed: %v", err)
		}
		if n != len(data) {
			return fmt.Errorf("Failed to read entire buffer over bulk interface")
		}
	case ReqMemReadCtrl:
		if err = m.dev.ControlIn(ReqMemReadCtrl, 0, data); err != nil {
			return fmt.Errorf("ReqMemReadCtrl data failed: %v", err)
		}
	}

	return nil
}

func (m *Memory) Read(addr Address, data interface{}) error {
	var err error
	if binary.Size(data) == -1 {
		return fmt.Errorf("Failed to get data size")
	}
	// TODO: read directly to data if it's already a slice of bytes.
	buf := make([]byte, binary.Size(data))
	if err = m.doRead(addr, buf); err != nil {
		return fmt.Errorf("m.doRead failed %v", err)
	}
	r := bytes.NewReader(buf)
	if err := binary.Read(r, binary.LittleEndian, data); err != nil {
		return fmt.Errorf("binary.Read failed: %v", err)
	}
	return nil
}

// Writes data to memory address addr.
// Automatically decides to use control-transfer of build-endpoint trasfer
// based on data length.
func (m *Memory) doWrite(addr Address, data []byte, validate bool, mask []byte) error {
	var err error
	var written int
	glog.V(1).Infof("[ext-mem-write]: addr = %v, dlen = %v", addr, len(data))

	cmd := ReqMemWriteBulk
	if len(data) < 48 {
		cmd = ReqMemWriteCtrl
	}

	info := AddressBlock{}
	info.Dlen = uint32(len(data))
	info.Addr = uint32(addr)

	infoBuf := new(bytes.Buffer)
	if err = binary.Write(infoBuf, binary.LittleEndian, info); err != nil {
		return fmt.Errorf("binary.Write failed: %v", err)
	}

	if cmd == ReqMemWriteCtrl {
		written, err = infoBuf.Write(data)
		if err != nil {
			return fmt.Errorf("Failed to append data: %v", err)
		}
		if written != len(data) {
			return fmt.Errorf("Failed to append data bytes")
		}
	}

	if err = m.dev.ControlOut(cmd, 0, infoBuf.Bytes()); err != nil {
		return fmt.Errorf("ControlOut AddressBlock failed: %v", err)
	}

	if cmd == ReqMemWriteBulk {
		if written, err = m.dev.Write(data); err != nil {
			return fmt.Errorf("ReqMemWriteBulk data failed: %v", err)
		}
		if written != len(data) {
			return fmt.Errorf("Failed to write entire buffer over bulk interface")
		}
	}

	if validate {
		actual := make([]byte, len(data))
		if err = m.Read(addr, actual); err != nil {
			return fmt.Errorf("Read for verify failed %v", err)
		}
		expected := make([]byte, len(data))
		copy(expected, data)
		if mask != nil {
			if len(mask) != len(data) {
				return fmt.Errorf("Mask length (%v) doesn't match data length (%v)", len(mask), len(data))
			}
			for i, m := range mask {
				actual[i] &= m
				expected[i] &= m
			}
		}
		if !bytes.Equal(expected, actual) {
			return fmt.Errorf("Write verification failed")
		}
	}
	return nil
}

func (m *Memory) Write(addr Address, data interface{}, validate bool, mask interface{}) error {
	var err error
	buf := new(bytes.Buffer)
	if err = binary.Write(buf, binary.LittleEndian, data); err != nil {
		return fmt.Errorf("binary.Write failed: %v", err)
	}
	// TODO: write directly from data if it's already a slice of bytes.
	var maskBytes []byte
	if mask != nil {
		var ok bool
		maskBytes, ok = mask.([]byte)
		if !ok {
			return fmt.Errorf("Invalid readMask type")
		}
	}
	if err = m.doWrite(addr, buf.Bytes(), validate, maskBytes); err != nil {
		return fmt.Errorf("m.doWrite failed %v", err)
	}
	return nil
}

func NewMemory(dev UsbDeviceInterface) *Memory {
	return &Memory{dev}
}
