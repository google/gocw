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

// Programs firmware on target device.
// Supported devices: XMEGA, STM32F. Program identifies the target chip, and calls
// the appropriate flash programmer.
package main

import (
	"flag"
	"path"

	"github.com/google/gocw/util"

	"github.com/golang/glog"
)

var (
	firmwareFile = flag.String("firmware", "", ".hex firmware file name")
)

func init() {
	flag.Parse()
}

func main() {
	var err error
	defer glog.Flush()

	if len(*firmwareFile) == 0 {
		glog.Fatal("Missing --firmware argument")
	}
	if path.Ext(*firmwareFile) != ".hex" {
		glog.Fatal("Expected Intel-Hex firmware file")
	}
	if err = util.ProgramFlashFile(*firmwareFile); err != nil {
		glog.Fatal("Failed programming device: %v", err)
	}

	glog.Info("Successfully programmed device")
}
