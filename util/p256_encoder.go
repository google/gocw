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
	"bytes"
	"fmt"
	"math/big"
)

// Returns x as a big-endian 32bytes long byte slice.
func EncodeP256Int(x *big.Int) []byte {
	buf := make([]byte, 32)
	copy(buf[32-len(x.Bytes()):32], x.Bytes())
	return buf
}

// Returns EC point (x, y) as a 64bytes big-endian array
func EncodeP256Point(x, y *big.Int) []byte {
	buf := new(bytes.Buffer)
	buf.Write(EncodeP256Int(x))
	buf.Write(EncodeP256Int(y))
	return buf.Bytes()
}

func DecodeP256Int(buf []byte) *big.Int {
	if len(buf) != 32 {
		panic(fmt.Sprintf("Unexpected buffer length (%v)", len(buf)))
	}
	x := new(big.Int)
	return x.SetBytes(buf)
}

func DecodeP256Point(buf []byte) (*big.Int, *big.Int) {
	if len(buf) != 64 {
		panic(fmt.Sprintf("Unexpected buffer length (%v)", len(buf)))
	}
	return DecodeP256Int(buf[0:32]), DecodeP256Int(buf[32:64])
}
