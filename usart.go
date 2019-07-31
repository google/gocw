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

// Serial USART interface.
package gocw

import (
	"encoding/hex"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/golang/glog"
)

//go:generate mockgen -destination=mocks/usart.go -package=mocks github.com/google/gocw UsartInterface
type UsartInterface interface {
	io.Reader
	io.Writer
	// Clears any pending data from the read buffer.
	Flush() (err error)
	// Gets/Sets Read timeout.
	Timeout() time.Duration
	SetTimeout(timeout time.Duration)
}

type command uint16

const (
	cmdInit    command = 0x10
	cmdEnable  command = 0x11
	cmdDisable command = 0x12
	cmdNumWait command = 0x14
)

type BaudRate uint32

const (
	BaudRateLow  BaudRate = 38400
	BaudRateHigh BaudRate = 115200
)

type Parity uint8

const (
	ParityNone  Parity = 0
	ParityOdd   Parity = 1
	ParityEven  Parity = 2
	ParityMark  Parity = 3
	ParitySpace Parity = 4
)

type StopBits uint8

const (
	StopBitsOne        StopBits = 0
	StopBitsOneAndHalf StopBits = 1
	StopBitsTwo        StopBits = 2
)

type DataBits uint8

const (
	DataBitsOneByte DataBits = 8
)

// Struct layout matches what cmdInit expects, so don't change this.
type UsartConfig struct {
	BaudRate BaudRate
	StopBits StopBits
	Parity   Parity
	DataBits DataBits
}

var defaultProperties = UsartConfig{
	BaudRateLow,
	StopBitsOne,
	ParityNone,
	DataBitsOneByte,
}

var defaultTimeout = 750 * time.Millisecond

type Usart struct {
	dev     UsbDeviceInterface
	conf    UsartConfig
	timeout time.Duration
}

func (u *Usart) configRead(cmd command, data interface{}) error {
	glog.V(1).Infof("[usart-config-read]: cmd = %v", cmd)
	return u.dev.ControlIn(ReqUsart0Config, uint16(cmd), data)
}

func (u *Usart) configWrite(cmd command, data interface{}) error {
	glog.V(1).Infof("[usart-config-write]: cmd = %v", cmd)
	return u.dev.ControlOut(ReqUsart0Config, uint16(cmd), data)
}

// Returns the number of bytes waiting to be read.
func (u *Usart) inWaiting() (int, error) {
	var err error
	var numBytes uint32
	if err = u.configRead(cmdNumWait, &numBytes); err != nil {
		return 0, fmt.Errorf("cmdNumWait failed: %v", err)
	}
	return int(numBytes), nil
}

func (u *Usart) dataRead(data []byte) error {
	glog.V(1).Infof("[usart-data-read]: len = %v", len(data))
	return u.dev.ControlIn(ReqUsart0Data, 0, data)
}

func (u *Usart) dataWrite(data []byte) error {
	glog.V(1).Infof("[usart-data-write]: data =\n%s", hex.Dump(data))
	return u.dev.ControlOut(ReqUsart0Data, 0, data)
}

func NewUsart(dev UsbDeviceInterface, conf *UsartConfig) (*Usart, error) {
	var err error
	u := &Usart{dev, defaultProperties, defaultTimeout}
	if conf != nil {
		u.conf = *conf
	}
	glog.Infof("USART configution: %v", u.conf)
	if err = u.configWrite(cmdInit, u.conf); err != nil {
		return nil, fmt.Errorf("cmdInit failed: %v", err)
	}
	if err = u.configWrite(cmdEnable, []byte{}); err != nil {
		return nil, fmt.Errorf("cmdEnable failed: %v", err)
	}
	glog.V(1).Infof("USART initialized successfully")
	return u, nil
}

func (u *Usart) Read(p []byte) (n int, err error) {
	var wg sync.WaitGroup
	timedOut := time.NewTimer(u.timeout)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-timedOut.C:
				return
			default:
				var toRead int
				if toRead, err = u.inWaiting(); err != nil {
					err = fmt.Errorf("inWaiting failed: %v", err)
					return
				}

				if n+toRead > len(p) {
					toRead = len(p) - n
				}

				if toRead == 0 {
					time.Sleep(time.Millisecond)
					continue
				}

				if err = u.dataRead(p[n : n+toRead]); err != nil {
					err = fmt.Errorf("dataRead failed: %v", err)
					return
				}

				n += toRead
				if n == len(p) {
					return
				}
			}
		}
	}()
	wg.Wait()
	return n, err
}

func (u *Usart) Write(p []byte) (n int, err error) {
	// Write memory in small chunks.
	for n < len(p) {
		toWrite := len(p) - n
		if toWrite > 58 {
			toWrite = 58
		}
		if err = u.dataWrite(p[n : n+toWrite]); err != nil {
			return n, fmt.Errorf("dataWrite failed: %v", err)
		}
		n += toWrite
	}
	return n, nil
}

func (u *Usart) Flush() (err error) {
	var toRead int
	for true {
		if toRead, err = u.inWaiting(); err != nil {
			return fmt.Errorf("inWaiting failed: %v", err)
		}
		if toRead == 0 {
			break
		}
		buf := make([]byte, toRead)
		if err = u.dataRead(buf); err != nil {
			return fmt.Errorf("dataRead failed: %v", err)
		}
	}
	return nil
}

func (u *Usart) Timeout() time.Duration {
	return u.timeout
}

func (u *Usart) SetTimeout(timeout time.Duration) {
	u.timeout = timeout
}
