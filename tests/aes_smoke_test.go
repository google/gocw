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
	"crypto/aes"
	"crypto/cipher"
	"flag"
	"path"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/gocw"
	"github.com/google/gocw/util"
)

const (
	aesFirmware = "build/firmware/tiny_aes.hex"
)

var (
	key = []byte{0x2b, 0x7e, 0x15, 0x16,
		0x28, 0xae, 0xd2, 0xa6,
		0xab, 0xf7, 0x15, 0x88,
		0x09, 0xcf, 0x4f, 0x3c}
	plain = []byte{0x6b, 0xc1, 0xbe, 0xe2,
		0x2e, 0x40, 0x9f, 0x96,
		0xe9, 0x3d, 0x7e, 0x11,
		0x73, 0x93, 0x17, 0x2a}
)

func init() {
	flag.Parse()
}

func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filename))
}

func TestAesFirmware(t *testing.T) {
	var err error
	if err = util.ProgramFlashFile(path.Join(projectRoot(), aesFirmware)); err != nil {
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

	if err = ser.WriteKey(key); err != nil {
		t.Fatal(err)
	}

	var bc cipher.Block
	if bc, err = aes.NewCipher(key); err != nil {
		t.Fatal(err)
	}
	expected := make([]byte, 16)
	bc.Encrypt(expected, plain)

	for i := 0; i < 2; i++ {
		if err = ser.WritePlaintext(plain); err != nil {
			t.Fatal(err)
		}
		var actual []byte
		if actual, err = ser.Response(); err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(actual, expected) {
			t.Errorf("Response (%v) did not match expected (%v)", actual, expected)
		}
	}
}
