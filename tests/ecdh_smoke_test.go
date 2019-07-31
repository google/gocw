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
	"crypto/elliptic"
	"flag"
	"fmt"
	"math/big"
	"path"
	"path/filepath"
	"runtime"

	"gocw"
	"gocw/util"

	"testing"
)

const (
	ecdhFirmware = "build/firmware/cryptoc_ecdh.hex"
)

func init() {
	flag.Parse()
}

func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filename))
}

func TestEcdhFirmware(t *testing.T) {
	var err error

	fmt.Printf("Programming device\n")
	if err = util.ProgramFlashFile(path.Join(projectRoot(), ecdhFirmware)); err != nil {
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

	// Verify firmware computes k*G.
	k := big.NewInt(0x12345678)
	fmt.Printf("Writing key\n")
	if err = ser.WriteKey(util.EncodeP256Int(k)); err != nil {
		t.Fatal(err)
	}

	gx := elliptic.P256().Params().Gx
	gy := elliptic.P256().Params().Gy
	fmt.Printf("Writing input point\n")
	if err = ser.WritePlaintext(util.EncodeP256Point(gx, gy)); err != nil {
		t.Fatal(err)
	}

	var output []byte
	fmt.Printf("Reading output point\n")
	if output, err = ser.Response(); err != nil {
		t.Fatal(err)
	}

	x, y := util.DecodeP256Point(output)
	fmt.Printf("Actual: %v, %v\n", x, y)

	wantX, wantY := elliptic.P256().ScalarMult(gx, gy, k.Bytes())
	fmt.Printf("Expected: %v, %v\n", wantX, wantY)

	if x.Cmp(wantX) != 0 || y.Cmp(wantY) != 0 {
		t.Errorf("Output point (%v, %v) did not match expected (%v, %v)", x, y, wantX, wantY)
	}
}
