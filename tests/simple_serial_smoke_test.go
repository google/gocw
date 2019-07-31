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

package main

import (
	"bytes"
	"flag"
	"path"
	"path/filepath"
	"runtime"

	"gocw"
	"gocw/util"

	"testing"
)

const (
	incFirmware = "build/firmware/inc_plaintext.hex"
)

func init() {
	flag.Parse()
}

func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filename))
}

func TestSimpleSerial(t *testing.T) {
	var err error
	if err = util.ProgramFlashFile(path.Join(projectRoot(), incFirmware)); err != nil {
		t.Fatal(err)
	}

	var dev gocw.UsbDeviceInterface
	if dev, err = gocw.OpenCwLiteUsbDevice(); err != nil {
		t.Fatal(err)
	}
	defer dev.Close()

	var fpga *gocw.Fpga
	if fpga, err = gocw.NewFpga(dev); err != nil {
		t.Fatal(err)
	}

	var adc *gocw.Adc
	if adc, err = gocw.NewAdc(fpga); err != nil {
		t.Fatal(err)
	}
	defer adc.Close()

	var usart *gocw.Usart
	if usart, err = gocw.NewUsart(dev, nil); err != nil {
		t.Fatal(err)
	}

	var ser *gocw.SimpleSerial
	if ser, err = gocw.NewSimpleSerial(usart); err != nil {
		t.Fatal(err)
	}

	plain := []byte{0x00, 0x11, 0x22, 0x33,
		0x44, 0x55, 0x66, 0x77,
		0x88, 0x99, 0xaa, 0xbb,
		0xcc, 0xdd, 0xee, 0xff}
	if err = ser.WritePlaintext(plain); err != nil {
		t.Fatal(err)
	}

	var actual []byte
	if actual, err = ser.Response(); err != nil {
		t.Fatal(err)
	}

	expected := []byte{0x01, 0x12, 0x23, 0x34,
		0x45, 0x56, 0x67, 0x78,
		0x89, 0x9a, 0xab, 0xbc,
		0xcd, 0xde, 0xef, 0x00}
	if !bytes.Equal(actual, expected) {
		t.Errorf("Response (%v) did not match expected (%v)", actual, expected)
	}
}
