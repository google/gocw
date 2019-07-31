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
	"encoding/hex"
	"fmt"
	"gocw"
	"reflect"

	"testing"
)

func TestSaveLoad(t *testing.T) {
	var err error
	var c1, c2 gocw.Capture
	c1 = gocw.Capture{gocw.Trace{Key: []byte{1},
		Pt:                []byte{2},
		Ct:                []byte{3},
		PowerMeasurements: []float64{4.5, 6.7}}}

	buf := bytes.Buffer{}
	if err := c1.SaveIo(&buf); err != nil {
		t.Errorf("Save failed: %v", err)
	}
	fmt.Printf("Capture file: %v\n", hex.EncodeToString(buf.Bytes()))

	if c2, err = gocw.LoadCaptureIo(bytes.NewReader(buf.Bytes())); err != nil {
		t.Errorf("Load failed: %v", err)
	}
	if !reflect.DeepEqual(c1, c2) {
		t.Errorf("Loaded capture (%v) did not match original (%v)", c2, c1)
	}
}
