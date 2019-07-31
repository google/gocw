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

package util

import (
	"fmt"
	"os"

	"github.com/marcinbor85/gohex"
)

type Segment struct {
	Address uint32
	Data    []byte
}

func LoadIntelHexFile(filename string) (*Segment, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	mem := gohex.NewMemory()
	if err = mem.ParseIntelHex(file); err != nil {
		return nil, err
	}

	segments := mem.GetDataSegments()
	if len(segments) != 1 {
		return nil, fmt.Errorf("Unexpected number of segments (%v)", len(segments))
	}

	return &Segment{segments[0].Address, segments[0].Data}, nil
}
