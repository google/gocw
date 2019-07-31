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

// Captures ECDH power traces to file.
package main

import (
	"crypto/elliptic"
	"crypto/rand"
	"flag"
	"fmt"
	"math/big"
	"path"
	"path/filepath"
	"runtime"

	"gocw"
	"gocw/util"

	"github.com/golang/glog"
)

var (
	programFlag = flag.Bool("program", true, "Program device at startup")
	samplesFlag = flag.Int("samples", 5000, "Number of samples per trace")
	tracesFlag  = flag.Int("traces", 50, "Number of traces to capture")
	offsetFlag  = flag.Int("offset", 0, "Offset of capture after trigger")
	pointFlag   = flag.String("point", "rand", "Input point type ['rand', 'zero']")
	outputFlag  = flag.String("output", "", "Capture .json.gz output file")
)

// Pre-computed points with sage:
// sage: params = {
//     "p": 0xFFFFFFFF00000001000000000000000000000000FFFFFFFFFFFFFFFFFFFFFFFF,
//     "a": 0xFFFFFFFF00000001000000000000000000000000FFFFFFFFFFFFFFFFFFFFFFFC,
//     "b": 0x5AC635D8AA3A93E7B3EBBD55769886BC651D06B0CC53B0F63BCE3C3E27D2604B,
//     "Gx": 0x6B17D1F2E12C4247F8BCE6E563A440F277037D812DEB33A0F4A13945D898C296,
//     "Gy": 0x4FE342E2FE1A7F9B8EE7EB4A7C0F9E162BCE33576B315ECECBB6406837BF51F5,
//     "n": 0xFFFFFFFF00000000FFFFFFFFFFFFFFFFBCE6FAADA7179E84F3B9CAC2FC632551,
// }
// sage: FF = FiniteField(params["p"])
// sage: a = FF(params["a"])
// sage: b = FF(params["b"])
// sage: Gx = FF(params["Gx"])
// sage: Gy = FF(params["Gy"])
// sage: assert (Gy**2 == Gx**3 + a * Gx + b)
// sage: EC = EllipticCurve(FF, [a, b])
// sage: G = EC.point([Gx, Gy])
// sage: assert (params["n"] == G.order())
//
// R is a P256 point with zero x-coordinate:
// sage: R = EC.point([0, b.sqrt()])
// sage: R
// (0 : 46263761741508638697010950048709651021688891777877937875096931459006746039284 : 1)
// sage: assert (EC.is_on_curve(*R.xy()))
//
// T is a P256 point such that 2*T has zero x-coordinate:
// sage: T = R*(R.order()//2)
// sage: assert (EC.is_on_curve(*T.xy()))
// sage: T
// (58687076926167833526398910613448791887093835024037337763248351435517941536121 : 52095585056448092084327535138728052592797106431412055157299799799802024215207 : 1)
// sage: sage: T*2
// (0 : 69528327468847610065686496900697922508397251637412376320436699849860351814667 : 1)
// sage: -R
// (0 : 69528327468847610065686496900697922508397251637412376320436699849860351814667 : 1)
//
var (
	Rx    = big.NewInt(0)
	Ry, _ = new(big.Int).SetString(
		"69528327468847610065686496900697922508397251637412376320436699849860351814667", 10)

	Tx, _ = new(big.Int).SetString(
		"58687076926167833526398910613448791887093835024037337763248351435517941536121", 10)
	Ty, _ = new(big.Int).SetString(
		"52095585056448092084327535138728052592797106431412055157299799799802024215207", 10)

	K = big.NewInt(2)
)

const (
	ecdhFirmware = "build/firmware/cryptoc_ecdh.hex"
)

func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filename))
}

func init() {
	flag.Parse()
}

func main() {
	var err error
	defer glog.Flush()

	// Sanity check points.
	x, y := elliptic.P256().ScalarMult(Tx, Ty, K.Bytes())
	if x.Cmp(Rx) != 0 || y.Cmp(Ry) != 0 {
		glog.Fatal("Bad T variable")
	}

	if *programFlag {
		glog.Info("Programming device")
		if err = util.ProgramFlashFile(path.Join(projectRoot(), ecdhFirmware)); err != nil {
			glog.Fatal(err)
		}
	}

	var pointGen gocw.PtGen
	switch *pointFlag {
	case "rand":
		// Capture 2*Q EC point multiplication operations.
		// The multiplication results in a random point on the curve.
		pointGen = func() ([]byte, error) {
			_, qx, qy, e := elliptic.GenerateKey(elliptic.P256(), rand.Reader)
			if e != nil {
				return nil, fmt.Errorf("GenerateKey failed: %v", e)
			}
			return util.EncodeP256Point(qx, qy), nil
		}
	case "zero":
		// Capture 2*T EC point multiplication operations.
		// The multiplication results in R, a point with zero x-coordinate.
		pointGen = func() ([]byte, error) {
			return util.EncodeP256Point(Tx, Ty), nil
		}
	default:
		glog.Fatal("Unknown --point flag. Valid values ['rand', 'zero']")
	}

	var capture gocw.Capture
	if capture, err = gocw.NewCapture(
		util.EncodeP256Int(K), pointGen, *samplesFlag, *tracesFlag, *offsetFlag); err != nil {
		glog.Fatal(err)
	}

	if len(*outputFlag) > 0 {
		capture.Save(*outputFlag)
	}
}
