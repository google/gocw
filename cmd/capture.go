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

// Captures target power traces to file.
package main

import (
	"encoding/hex"
	"flag"

	"gocw"

	"github.com/golang/glog"
)

var (
	samplesFlag = flag.Int("samples", 1500, "Number of samples per trace")
	tracesFlag  = flag.Int("traces", 50, "Number of traces to capture")
	offsetFlag  = flag.Int("offset", 0, "Offset of capture after trigger")
	outputFlag  = flag.String("output", "", "Capture .json.gz output file")
	keyHexFlag  = flag.String("key", "2b7e151628aed2a6abf7158809cf4f3c",
		"16byte key in hex")
)

func init() {
	flag.Parse()
}

func main() {
	var err error
	defer glog.Flush()

	var key []byte
	if key, err = hex.DecodeString(*keyHexFlag); err != nil {
		glog.Fatal(err)
	}

	var capture gocw.Capture
	if capture, err = gocw.NewCapture(
		key, gocw.RandGen(len(key)), *samplesFlag, *tracesFlag, *offsetFlag); err != nil {
		glog.Fatal(err)
	}

	if len(*outputFlag) > 0 {
		if err = capture.Save(*outputFlag); err != nil {
			glog.Fatal(err)
		}
	} else {
		glog.Infof("Capture: %v", capture)
	}
}
