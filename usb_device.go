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

// Provides low-level interface for ChipWhisperer USB device.
// Based on chipwhisperer/software/chipwhisperer/hardware/naeusb/naeusb.py.
// Implementation is hard-coded for ChipWhisperer-Lite hardware.
package gocw

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/golang/glog"
	"github.com/google/gousb"
)

const (
	cwliteVid   = 0x2b3e
	cwlitePid   = 0xace2
	cwliteInEp  = 1
	cwliteOutEp = 2

	cwliteMjVersion = 0
	cwliteMnVersion = 11
)

//go:generate stringer -type Request
type Request uint8

const (
	ReqMemReadBulk  Request = 0x10
	ReqMemWriteBulk Request = 0x11
	ReqMemReadCtrl  Request = 0x12
	ReqMemWriteCtrl Request = 0x13
	ReqFpgaStatus   Request = 0x15
	ReqFpgaProgram  Request = 0x16
	ReqFwVersion    Request = 0x17
	ReqUsart0Data   Request = 0x1a
	ReqUsart0Config Request = 0x1b
	ReqXmegaProgram Request = 0x20
)

const (
	rTypeControlIn  uint8 = gousb.ControlIn | gousb.ControlVendor | gousb.ControlInterface
	rTypeControlOut uint8 = gousb.ControlOut | gousb.ControlVendor | gousb.ControlInterface
)

//go:generate mockgen -destination=mocks/usb_device.go -package=mocks github.com/google/gocw UsbDeviceInterface
type UsbDeviceInterface interface {
	// Reads/Writes to bulk data endpoint.
	io.Reader
	io.Writer
	io.Closer
	// Sends a request over the control endpoint.
	ControlIn(request Request, val uint16, data interface{}) error
	ControlOut(request Request, val uint16, data interface{}) error
}

// Encapsulates CW USB resources.
type UsbDevice struct {
	ctx *gousb.Context
	// dev also implements the control endpoint.
	dev       *gousb.Device
	intf      *gousb.Interface
	intf_done func()
	// Bulk output/input data endpoints.
	ep_out *gousb.OutEndpoint
	ep_in  *gousb.InEndpoint
}

func OpenCwLiteUsbDevice() (*UsbDevice, error) {
	d := &UsbDevice{}
	d.ctx = gousb.NewContext()

	var err error
	d.dev, err = d.ctx.OpenDeviceWithVIDPID(cwliteVid, cwlitePid)
	if d.dev == nil && err == nil {
		return nil, fmt.Errorf("CWLite device not found")
	}

	if err != nil {
		d.Close()
		return nil, fmt.Errorf("Opening CWLITE device: %v", err)
	}

	// The default interface is always #0 alt #0 in the currently active
	// config.
	d.intf, d.intf_done, err = d.dev.DefaultInterface()
	if err != nil {
		d.Close()
		return nil, fmt.Errorf("Claming default interface: %v", err)
	}

	d.ep_out, err = d.intf.OutEndpoint(cwliteOutEp)
	if err != nil {
		d.Close()
		return nil, fmt.Errorf("Opening output interface: %v", err)
	}

	d.ep_in, err = d.intf.InEndpoint(cwliteInEp)
	if err != nil {
		d.Close()
		return nil, fmt.Errorf("Opening input interface: %v", err)
	}

	ver := FwVersion{}
	if err = d.ReadFwVersion(&ver); err != nil {
		return nil, fmt.Errorf("Failed reading FW version: %v", err)
	}

	if ver.Major != cwliteMjVersion || ver.Minor != cwliteMnVersion {
		return nil, fmt.Errorf("Unexpected FW version: %v", ver)
	}
	return d, err
}

func (d *UsbDevice) Close() error {
	glog.V(1).Infof("Closing USB device")
	if d.intf_done != nil {
		d.intf_done()
		d.intf_done = nil
	}
	if d.intf != nil {
		d.intf.Close()
		d.intf = nil
	}
	if d.dev != nil {
		d.dev.Close()
		d.dev = nil
	}
	if d.ctx != nil {
		d.ctx.Close()
		d.ctx = nil
	}
	return nil
}

func (d *UsbDevice) Read(p []byte) (n int, err error) {
	n, err = d.ep_in.Read(p)
	glog.V(2).Infof("[usb-bulk IN]: read %d bytes. data:[:32]\n%s", n, hex.Dump(p[:32]))
	return n, err
}

func (d *UsbDevice) Write(buf []byte) (n int, err error) {
	n, err = d.ep_out.Write(buf)
	glog.V(2).Infof("[usb-bulk OUT]: wrote %d bytes. data[:32]:\n%s", n, hex.Dump(buf[:32]))
	return n, err
}

func (d *UsbDevice) ControlIn(request Request, val uint16, data interface{}) error {
	if binary.Size(data) == -1 {
		return fmt.Errorf("Failed to get data size")
	}
	// TODO: read directly to data if it's already a slice of bytes.
	buf := make([]byte, binary.Size(data))
	n, err := d.dev.Control(rTypeControlIn, uint8(request), val, 0, buf)
	if err != nil {
		return fmt.Errorf("dev.Control failed %v", err)
	}
	if n != len(buf) {
		return fmt.Errorf("Failed to read entire buffer %v vs %v", n, len(buf))
	}
	r := bytes.NewReader(buf)
	if err := binary.Read(r, binary.LittleEndian, data); err != nil {
		return fmt.Errorf("binary.Read failed: %v", err)
	}
	glog.V(2).Infof("[usb-ctrl IN]: request = %v, val = %x, data =\n%s",
		request, val, hex.Dump(buf))
	return nil
}

func (d *UsbDevice) ControlOut(request Request, val uint16, data interface{}) error {
	var err error
	buf := new(bytes.Buffer)
	if err = binary.Write(buf, binary.LittleEndian, data); err != nil {
		return fmt.Errorf("binary.Write failed: %v", err)
	}
	// TODO: write directly from data if it's already a slice of bytes.
	n, err := d.dev.Control(rTypeControlOut, uint8(request), val, 0, buf.Bytes())
	if err != nil {
		return fmt.Errorf("dev.Control failed %v", err)
	}
	if n != buf.Len() {
		return fmt.Errorf("Failed to write entire buffer %v vs %v", n, buf.Len())
	}
	glog.V(2).Infof("[usb-ctrl OUT]: request = %v, val = %x, data =\n%s",
		request, val, hex.Dump(buf.Bytes()))
	return nil
}

type FwVersion struct {
	Major uint8
	Minor uint8
	Debug uint8
}

// Reads CW capture firmware version.
func (d *UsbDevice) ReadFwVersion(ver *FwVersion) error {
	return d.ControlIn(ReqFwVersion, 0, ver)
}
