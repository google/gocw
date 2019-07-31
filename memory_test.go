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

package gocw_test

import (
	"bytes"
	"testing"

	"github.com/google/gocw"
	"github.com/google/gocw/mocks"

	"github.com/golang/mock/gomock"
)

func TestMemoryRead(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	data := []byte{0xaa, 0xbb, 0xcc}
	const addr = 0x11223344
	dev := mocks.NewMockUsbDeviceInterface(mockCtrl)
	gomock.InOrder(
		// Address block
		dev.EXPECT().ControlOut(
			gocw.ReqMemReadCtrl, uint16(0), &gocw.AddressBlock{uint32(len(data)), addr}).
			Return(nil),
		// Read data
		dev.EXPECT().ControlIn(
			gocw.ReqMemReadCtrl, uint16(0), gomock.Any()).
			SetArg(2, data).
			Return(nil),
	)
	m := gocw.NewMemory(dev)
	out := make([]byte, 3)
	err := m.Read(addr, out)
	if err != nil {
		t.Errorf("Memory Read failed: %v", err)
	}
	if !bytes.Equal(out, data) {
		t.Errorf("Unexpected data returned (%v)", out)
	}
}

func TestMemoryWrite(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	data := []byte{0xaa, 0xbb, 0xcc}
	const addr = 0x11223344
	dev := mocks.NewMockUsbDeviceInterface(mockCtrl)
	gomock.InOrder(
		// Address block + data
		dev.EXPECT().ControlOut(
			gocw.ReqMemWriteCtrl, uint16(0),
			[]byte{3, 0, 0, 0, // dlen
				0x44, 0x33, 0x22, 0x11, // addr
				0xaa, 0xbb, 0xcc, // data
			}).
			Return(nil),
	)
	m := gocw.NewMemory(dev)
	err := m.Write(addr, data, false, nil)
	if err != nil {
		t.Errorf("Memory Write failed: %v", err)
	}
}

func TestMemoryWriteDataVerificationPasses(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	data := []byte{0xaa, 0xbb, 0xcc}
	dataRead := []byte{0xaa, 0xbb, 0xcc} // dataRead equals dataWrite.
	const addr = 0x11223344
	dev := mocks.NewMockUsbDeviceInterface(mockCtrl)
	gomock.InOrder(
		// Address block + data
		dev.EXPECT().ControlOut(
			gocw.ReqMemWriteCtrl, uint16(0),
			[]byte{3, 0, 0, 0, // dlen
				0x44, 0x33, 0x22, 0x11, // addr
				0xaa, 0xbb, 0xcc, // data
			}).
			Return(nil),
		// Address block
		dev.EXPECT().ControlOut(
			gocw.ReqMemReadCtrl, uint16(0), &gocw.AddressBlock{uint32(len(dataRead)), addr}).
			Return(nil),
		// Read data
		dev.EXPECT().ControlIn(
			gocw.ReqMemReadCtrl, uint16(0), gomock.Any()).
			SetArg(2, dataRead).
			Return(nil),
	)
	m := gocw.NewMemory(dev)
	err := m.Write(addr, data, true, nil)
	if err != nil {
		t.Errorf("Memory Write failed: %v", err)
	}
}

func TestMemoryWriteDataVerificationFails(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	data := []byte{0xaa, 0xbb, 0xcc}
	dataRead := []byte{0xaa, 0xdd, 0xcc} // 2nd byte is different.
	const addr = 0x11223344
	dev := mocks.NewMockUsbDeviceInterface(mockCtrl)
	gomock.InOrder(
		// Address block + data
		dev.EXPECT().ControlOut(
			gocw.ReqMemWriteCtrl, uint16(0),
			[]byte{3, 0, 0, 0, // dlen
				0x44, 0x33, 0x22, 0x11, // addr
				0xaa, 0xbb, 0xcc, // data
			}).
			Return(nil),
		// Address block
		dev.EXPECT().ControlOut(
			gocw.ReqMemReadCtrl, uint16(0), &gocw.AddressBlock{uint32(len(dataRead)), addr}).
			Return(nil),
		// Read data
		dev.EXPECT().ControlIn(
			gocw.ReqMemReadCtrl, uint16(0), gomock.Any()).
			SetArg(2, dataRead).
			Return(nil),
	)
	m := gocw.NewMemory(dev)
	err := m.Write(addr, data, true, nil)
	if err == nil {
		t.Errorf("Memory Write expected to fail")
	}
}

func TestMemoryWriteReadMaskPasses(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	data := []byte{0xaa, 0xbb, 0xcc}
	dataRead := []byte{0xaa, 0xdd, 0xcc} // 2nd byte is different.
	mask := []byte{0xff, 0x00, 0xff}     // mask out 2nd byte.
	const addr = 0x11223344
	dev := mocks.NewMockUsbDeviceInterface(mockCtrl)
	gomock.InOrder(
		// Address block + data
		dev.EXPECT().ControlOut(
			gocw.ReqMemWriteCtrl, uint16(0),
			[]byte{3, 0, 0, 0, // dlen
				0x44, 0x33, 0x22, 0x11, // addr
				0xaa, 0xbb, 0xcc, // data
			}).
			Return(nil),
		// Address block
		dev.EXPECT().ControlOut(
			gocw.ReqMemReadCtrl, uint16(0), &gocw.AddressBlock{uint32(len(dataRead)), addr}).
			Return(nil),
		// Read data
		dev.EXPECT().ControlIn(
			gocw.ReqMemReadCtrl, uint16(0), gomock.Any()).
			SetArg(2, dataRead).
			Return(nil),
	)
	m := gocw.NewMemory(dev)
	err := m.Write(addr, data, true, mask)
	if err != nil {
		t.Errorf("Memory Write failed: %v", err)
	}
}
