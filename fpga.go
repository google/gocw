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

// FPGA interface.
package gocw

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/gocw/hardware"

	"github.com/golang/glog"
)

type Fpga struct {
	dev UsbDeviceInterface
	Mem *Memory
}

func (f *Fpga) IsProgrammed() (bool, error) {
	glog.V(2).Info("FPGA is programmed")
	var err error
	var status uint32
	if err = f.dev.ControlIn(ReqFpgaStatus, 0, &status); err != nil {
		return false, fmt.Errorf("ReqFpgaStatus: %v", err)
	}
	return bool(status&1 == 1), nil
}

func (f *Fpga) ctrlProgram(val uint16) error {
	return f.dev.ControlOut(ReqFpgaProgram, val, []byte{})
}

func (f *Fpga) Program(bitstream io.Reader) error {
	var err error
	glog.V(1).Info("Programming FPGA")
	// Erase the FPGA by toggling PROGRAM pin, setup
	// NAEUSB chip for FPGA programming
	if err = f.ctrlProgram(0xA0); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)

	if err = f.ctrlProgram(0xA1); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)

	// Download bitstream to device
	if _, err = io.Copy(f.dev, bitstream); err != nil {
		return fmt.Errorf("Failed to download bitstream %v", err)
	}

	var ready bool
	for attempts := 0; attempts < 5; attempts-- {
		ready, err = f.IsProgrammed()
		if err != nil {
			return err
		}
		if ready {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Exit FPGA programming mode
	if err = f.ctrlProgram(0xA2); err != nil {
		return err
	}

	if !ready {
		return fmt.Errorf("FPGA done pin failed to go high, bad bitstream?")
	}
	return nil
}

func (f *Fpga) ProgramCwlite() error {
	var err error
	var bs http.File
	if bs, err = hardware.FS.Open("/cwlite_interface.bit"); err != nil {
		return fmt.Errorf("Failed opening bitstream file %v", err)
	}
	defer bs.Close()
	return f.Program(bs)
}

func NewFpga(dev UsbDeviceInterface) (*Fpga, error) {
	var err error
	var programmed bool
	f := &Fpga{dev, NewMemory(dev)}

	if programmed, err = f.IsProgrammed(); err != nil {
		return nil, fmt.Errorf("IsProgrammed failed %v", err)
	}

	if !programmed {
		if err = f.ProgramCwlite(); err != nil {
			return nil, fmt.Errorf("ProgramCwlite failed %v", err)
		}
	}

	return f, nil
}
