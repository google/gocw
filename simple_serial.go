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

// simple-serial protocol.
package gocw

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
)

type SimpleSerial struct {
	usart UsartInterface
}

func (s *SimpleSerial) WriteKey(k []byte) error {
	var err error
	cmd := bytes.NewBufferString(fmt.Sprintf("k%s\n", hex.EncodeToString(k)))
	if _, err = s.usart.Write(cmd.Bytes()); err != nil {
		return fmt.Errorf("Failed to write key command: %v", err)
	}
	if err = s.waitForAck(); err != nil {
		return err
	}
	return nil
}

func (s *SimpleSerial) WritePlaintext(p []byte) error {
	var err error
	cmd := bytes.NewBufferString(fmt.Sprintf("p%s\n", hex.EncodeToString(p)))
	if _, err = s.usart.Write(cmd.Bytes()); err != nil {
		return fmt.Errorf("Failed to write p command: %v", err)
	}
	return nil
}

func (s *SimpleSerial) waitForAck() error {
	var err error
	var res string
	if res, err = s.ResponseLine(); err != nil {
		return err
	}
	if res[0] != 'z' {
		return fmt.Errorf("ACK error %v", res)
	}
	return nil
}

// Reads response line.
func (s *SimpleSerial) ResponseLine() (string, error) {
	rd := bufio.NewReader(s.usart)
	return rd.ReadString('\n')
}

// Reads response.
func (s *SimpleSerial) Response() ([]byte, error) {
	var err error
	var res string
	if res, err = s.ResponseLine(); err != nil {
		return nil, err
	}
	if res[0] != 'r' {
		return nil, fmt.Errorf("Res error %v", res)
	}
	res = strings.TrimSuffix(res, "\n")
	return hex.DecodeString(res[1:])
}

func (s *SimpleSerial) checkVersion() error {
	var err error
	if err = s.usart.Flush(); err != nil {
		return fmt.Errorf("Flush failed: %v", err)
	}
	if _, err = s.usart.Write([]byte{'v', '\n'}); err != nil {
		return fmt.Errorf("Failed to write ver command: %v", err)
	}
	res := make([]byte, 4)
	if _, err = s.usart.Read(res); err != nil {
		return fmt.Errorf("Failed to read ver response: %v", err)
	}
	if res[0] != 'z' {
		return fmt.Errorf("Version 1.0 is not supported")
	}
	return nil
}

func (s *SimpleSerial) flush() error {
	var err error
	// 'x' flushes everything & sets system back to idle
	clear := bytes.NewBufferString("xxxxxxxxxxxxxxxxxxx\n")
	if _, err = s.usart.Write(clear.Bytes()); err != nil {
		return fmt.Errorf("Failed to write flush command: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err = s.usart.Flush(); err != nil {
		return fmt.Errorf("Failed to flush read buffer: %v", err)
	}
	return nil
}

func NewSimpleSerial(usart UsartInterface) (*SimpleSerial, error) {
	var err error
	glog.V(1).Infof("Opening SimpleSerial")
	s := &SimpleSerial{usart}
	if err = s.flush(); err != nil {
		return nil, err
	}
	if err = s.checkVersion(); err != nil {
		return nil, err
	}
	return s, nil
}
