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

// Attack sbox lookup of first round of AES-128 using differential power analysis.
// https://www.paulkocher.com/doc/DifferentialPowerAnalysis.pdf

// $ go run power_analysis/attack_sbox_dpa.go -logtostderr -v=1
// [attack_sbox_dpa.go:89] Loaded capture with 500 traces / 5000 samples per trace
// [attack_sbox_dpa.go:95] T is 500 x 2000 matrix
// [attack_sbox_dpa.go:130] Best guess for index 9: <Key:0xf7, Diff:0.003849, Loc: 1277>
// [attack_sbox_dpa.go:130] Best guess for index 14: <Key:0x4f, Diff:0.004740, Loc: 1493>
// [attack_sbox_dpa.go:130] Best guess for index 8: <Key:0xab, Diff:0.006338, Loc: 1101>
// [attack_sbox_dpa.go:130] Best guess for index 10: <Key:0x15, Diff:0.005330, Loc: 1453>
// [attack_sbox_dpa.go:130] Best guess for index 13: <Key:0xcf, Diff:0.005314, Loc: 1317>
// [attack_sbox_dpa.go:130] Best guess for index 11: <Key:0x88, Diff:0.005222, Loc: 1629>
// [attack_sbox_dpa.go:130] Best guess for index 3: <Key:0x16, Diff:0.005074, Loc: 1549>
// [attack_sbox_dpa.go:130] Best guess for index 2: <Key:0x15, Diff:0.004392, Loc: 1373>
// [attack_sbox_dpa.go:130] Best guess for index 0: <Key:0x2b, Diff:0.005253, Loc: 1021>
// [attack_sbox_dpa.go:130] Best guess for index 7: <Key:0xa6, Diff:0.005045, Loc: 1589>
// [attack_sbox_dpa.go:130] Best guess for index 12: <Key:0x09, Diff:0.005581, Loc: 1141>
// [attack_sbox_dpa.go:130] Best guess for index 15: <Key:0x3c, Diff:0.005395, Loc: 1669>
// [attack_sbox_dpa.go:130] Best guess for index 4: <Key:0x28, Diff:0.007074, Loc: 1061>
// [attack_sbox_dpa.go:130] Best guess for index 5: <Key:0xae, Diff:0.004561, Loc: 1237>
// [attack_sbox_dpa.go:130] Best guess for index 1: <Key:0x7e, Diff:0.004438, Loc: 1197>
// [attack_sbox_dpa.go:130] Best guess for index 6: <Key:0xd2, Diff:0.005460, Loc: 1413>
// [attack_sbox_dpa.go:136] Fully recovered key: 2b7e151628aed2a6abf7158809cf4f3c

package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"math"
	"sync"

	"gocw"

	"github.com/golang/glog"
	"gonum.org/v1/gonum/mat"
)

var (
	inputFlag    = flag.String("input", "captures/stm_aes_t500_s5000.json.gz", "Capture input file")
	winStartFlag = flag.Int("t1", 0, "Window start")
	winEndFlag   = flag.Int("t2", 0, "Window end")

	// Copied from third_party/tiny-AES-c/aes.c
	sbox = [256]byte{
		//0     1    2      3     4    5     6     7      8    9     A      B    C     D     E     F
		0x63, 0x7c, 0x77, 0x7b, 0xf2, 0x6b, 0x6f, 0xc5, 0x30, 0x01, 0x67, 0x2b, 0xfe, 0xd7, 0xab, 0x76,
		0xca, 0x82, 0xc9, 0x7d, 0xfa, 0x59, 0x47, 0xf0, 0xad, 0xd4, 0xa2, 0xaf, 0x9c, 0xa4, 0x72, 0xc0,
		0xb7, 0xfd, 0x93, 0x26, 0x36, 0x3f, 0xf7, 0xcc, 0x34, 0xa5, 0xe5, 0xf1, 0x71, 0xd8, 0x31, 0x15,
		0x04, 0xc7, 0x23, 0xc3, 0x18, 0x96, 0x05, 0x9a, 0x07, 0x12, 0x80, 0xe2, 0xeb, 0x27, 0xb2, 0x75,
		0x09, 0x83, 0x2c, 0x1a, 0x1b, 0x6e, 0x5a, 0xa0, 0x52, 0x3b, 0xd6, 0xb3, 0x29, 0xe3, 0x2f, 0x84,
		0x53, 0xd1, 0x00, 0xed, 0x20, 0xfc, 0xb1, 0x5b, 0x6a, 0xcb, 0xbe, 0x39, 0x4a, 0x4c, 0x58, 0xcf,
		0xd0, 0xef, 0xaa, 0xfb, 0x43, 0x4d, 0x33, 0x85, 0x45, 0xf9, 0x02, 0x7f, 0x50, 0x3c, 0x9f, 0xa8,
		0x51, 0xa3, 0x40, 0x8f, 0x92, 0x9d, 0x38, 0xf5, 0xbc, 0xb6, 0xda, 0x21, 0x10, 0xff, 0xf3, 0xd2,
		0xcd, 0x0c, 0x13, 0xec, 0x5f, 0x97, 0x44, 0x17, 0xc4, 0xa7, 0x7e, 0x3d, 0x64, 0x5d, 0x19, 0x73,
		0x60, 0x81, 0x4f, 0xdc, 0x22, 0x2a, 0x90, 0x88, 0x46, 0xee, 0xb8, 0x14, 0xde, 0x5e, 0x0b, 0xdb,
		0xe0, 0x32, 0x3a, 0x0a, 0x49, 0x06, 0x24, 0x5c, 0xc2, 0xd3, 0xac, 0x62, 0x91, 0x95, 0xe4, 0x79,
		0xe7, 0xc8, 0x37, 0x6d, 0x8d, 0xd5, 0x4e, 0xa9, 0x6c, 0x56, 0xf4, 0xea, 0x65, 0x7a, 0xae, 0x08,
		0xba, 0x78, 0x25, 0x2e, 0x1c, 0xa6, 0xb4, 0xc6, 0xe8, 0xdd, 0x74, 0x1f, 0x4b, 0xbd, 0x8b, 0x8a,
		0x70, 0x3e, 0xb5, 0x66, 0x48, 0x03, 0xf6, 0x0e, 0x61, 0x35, 0x57, 0xb9, 0x86, 0xc1, 0x1d, 0x9e,
		0xe1, 0xf8, 0x98, 0x11, 0x69, 0xd9, 0x8e, 0x94, 0x9b, 0x1e, 0x87, 0xe9, 0xce, 0x55, 0x28, 0xdf,
		0x8c, 0xa1, 0x89, 0x0d, 0xbf, 0xe6, 0x42, 0x68, 0x41, 0x99, 0x2d, 0x0f, 0xb0, 0x54, 0xbb, 0x16}
)

// Splits the capture into two sets: the ones where the expected sbox bit is one,
// and the ones where the expected sbox bit is zero.
// Returns boolean vectors.
func leakModel(key byte, keyIdx int, capture gocw.Capture) (mat.Vector, mat.Vector) {
	split0 := mat.NewVecDense(len(capture), nil)
	split1 := mat.NewVecDense(len(capture), nil)
	// Any bit can be used as a predicate for the split.
	indicatorBit := byte(2)
	for i := 0; i < len(capture); i++ {
		pt := capture[i].Pt[keyIdx]
		ct := sbox[pt^key]
		if ct&(1<<indicatorBit) > 0 {
			split0.SetVec(i, 0.0)
			split1.SetVec(i, 1.0)
		} else {
			split0.SetVec(i, 1.0)
			split1.SetVec(i, 0.0)
		}
	}
	return split0, split1
}

type keyGuess struct {
	key         byte
	maxDiff     float64
	maxLocation int
}

func (g keyGuess) String() string {
	return fmt.Sprintf("<Key:0x%02x, Diff:%f, Loc: %d>", g.key, g.maxDiff, g.maxLocation)
}

func init() {
	flag.Parse()
}

func main() {
	defer glog.Flush()

	capture, err := gocw.LoadCapture(*inputFlag)
	if err != nil {
		glog.Fatal(err)
	}

	glog.Infof("Loaded capture with %d traces / %d samples per trace",
		len(capture), len(capture[0].PowerMeasurements))

	M := capture.SamplesMatrix()
	if *winEndFlag == 0 {
		*winEndFlag = len(capture[0].PowerMeasurements)
	}
	T := M.(*mat.Dense).Slice(0, len(capture), *winStartFlag, *winEndFlag)
	r, c := T.Dims()
	glog.Infof("T is %d x %d matrix", r, c)

	fullKey := make([]byte, 16)
	var wg sync.WaitGroup
	wg.Add(16)
	for k := 0; k < 16; k++ {
		go func(keyIdx int) {
			defer wg.Done()
			bestGuess := keyGuess{byte(0), 0, 0}
			for key := 0; key < 256; key++ {
				S0, S1 := leakModel(byte(key), keyIdx, capture)

				// Compute difference of means.
				var avg0 mat.Dense
				S0.(*mat.VecDense).ScaleVec(1.0/mat.Sum(S0), S0)
				avg0.Product(mat.TransposeVec{S0}, T)

				var avg1 mat.Dense
				S1.(*mat.VecDense).ScaleVec(1.0/mat.Sum(S1), S1)
				avg1.Product(mat.TransposeVec{S1}, T)

				var diff mat.Dense
				diff.Sub(&avg0, &avg1)

				// Best guess is the key with the highest difference-of-means between all possible keys,
				// across all possible time-slices.
				for i, v := range diff.RawRowView(0) {
					v = math.Abs(v)
					if v > bestGuess.maxDiff {
						bestGuess = keyGuess{byte(key), v, *winStartFlag + i}
					}
				}
			}
			glog.V(1).Infof("Best guess for index %d: %v", keyIdx, bestGuess)
			fullKey[keyIdx] = bestGuess.key
		}(k)
	}

	wg.Wait()
	glog.Infof("Fully recovered key: %v", hex.EncodeToString(fullKey))
}
