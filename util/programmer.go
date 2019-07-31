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

package util

import (
	"bytes"
	"fmt"

	"gocw/programmer"
	"gocw/programmer/stm32f"
	"gocw/programmer/xmega"

	"github.com/golang/glog"
)

// Writes firmware to flash.
// Erases chip, writes contents to flash, reads and verifies the result.
func ProgramDevice(prog programmer.ProgrammerInterface, firmware *Segment) error {
	var err error
	glog.Info("Erasing chip")
	if err = prog.Erase(); err != nil {
		return fmt.Errorf("Failed to erase chip: %v", err)
	}
	glog.Info("Programming flash")
	w := prog.NewMemoryWriter(firmware.Address)
	if _, err = w.Write(firmware.Data); err != nil {
		return fmt.Errorf("Failed to write to flash: %v", err)
	}
	glog.Info("Verifying contents")
	r := prog.NewMemoryReader(firmware.Address)
	mem := make([]byte, len(firmware.Data))
	if _, err = r.Read(mem); err != nil {
		return fmt.Errorf("Failed to read flash contents: %v", err)
	}
	if !bytes.Equal(firmware.Data, mem) {
		return fmt.Errorf("Data verification failed")
	}
	glog.Info("Device programmed successfully")
	return nil
}

func ProgramFlashFile(filename string) error {
	var err error
	var firmware *Segment
	if firmware, err = LoadIntelHexFile(filename); err != nil {
		glog.Fatalf("Failed loading hex file: %v", err)
	}

	var prog programmer.ProgrammerInterface
	if prog, err = xmega.NewProgrammer(); err != nil {
		glog.Warningf("Failed opening XMEGA device: %v", err)
		if prog, err = stm32f.NewProgrammer(); err != nil {
			glog.Fatalf("Failed opening STM device: %v", err)
		}
	}
	defer prog.Close()

	return ProgramDevice(prog, firmware)
}
