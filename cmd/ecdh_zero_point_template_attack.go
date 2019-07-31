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

// Mounts template-attack on ECDH power traces, and proves zero-coordinate
// point operations are discernible from power traces.
// This can be used to mount Goubin's "Refined Power-Analysis Attack".
// https://wiki.newae.com/Template_Attacks

// $ go run cmd/ecdh_zero_point_template_attack.go -logtostderr \
//      -zero_capture captures/stm_ecdh_zero_t60_s5000.json.gz \
//      -rand_capture captures/stm_ecdh_rand_t120_s5000.json.gz
// [ecdh_zero_point_template_attack.go:176] Loading zero-point capture
// [ecdh_zero_point_template_attack.go:180] Loading rand-point capture
// [ecdh_zero_point_template_attack.go:187] Finding points of interest
// [ecdh_zero_point_template_attack.go:189] Selected POI: [4549 3745 4593 4753 2421]
// [ecdh_zero_point_template_attack.go:191] Building zero-point template
// [ecdh_zero_point_template_attack.go:194] Building rand-point template
// [ecdh_zero_point_template_attack.go:197] Testing zero-point validation set
// [ecdh_zero_point_template_attack.go:159] Classified x=[...] as a zero point trace
// [ecdh_zero_point_template_attack.go:159] Classified x=[...] as a zero point trace
// [ecdh_zero_point_template_attack.go:159] Classified x=[...] as a zero point trace
// [ecdh_zero_point_template_attack.go:159] ...
// [ecdh_zero_point_template_attack.go:200] Testing rand-point validation set
// [ecdh_zero_point_template_attack.go:161] Classified x=[...] as a rand point trace
// [ecdh_zero_point_template_attack.go:161] Classified x=[...] as a rand point trace
// [ecdh_zero_point_template_attack.go:161] Classified x=[...] as a rand point trace
// [ecdh_zero_point_template_attack.go:161] ...

package main

import (
	"flag"
	"math"
	"sort"

	"github.com/google/gocw"

	"github.com/golang/glog"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
	"gonum.org/v1/gonum/stat/distmv"
)

var (
	zeroCaptureFlag = flag.String("zero_capture", "captures/stm_ecdh_zero_t60_s5000.json.gz",
		"Capture with ECDH operations that resulted with a zero x-coordinate point")
	randCaptureFlag = flag.String("rand_capture", "captures/stm_ecdh_rand_t120_s5000.json.gz",
		"Capture with ECDH operations with random EC point")
)

const (
	numPoi = 5
)

func init() {
	flag.Parse()
}

// Points-Of-Interest are the points in time where the avg trace for the zero-point
// computation is different from the avg trace for the random point computation.
// These will be used in our template based classifier.
type pointOfInterest struct {
	diff     float64
	location int
}

type pointsOfInterest []pointOfInterest

func (p pointsOfInterest) Len() int      { return len(p) }
func (p pointsOfInterest) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

type ByDiff struct{ pointsOfInterest }

func (p ByDiff) Less(i, j int) bool {
	return p.pointsOfInterest[i].diff < p.pointsOfInterest[j].diff
}

func findPointsOfInterest(zeroAvg, randAvg mat.Vector) []int {
	poi := make([]pointOfInterest, zeroAvg.Len())
	for i := 0; i < zeroAvg.Len(); i++ {
		poi[i] = pointOfInterest{math.Abs(zeroAvg.AtVec(i) - randAvg.AtVec(i)), i}
	}
	sort.Sort(sort.Reverse(ByDiff{poi}))
	glog.V(1).Infof("Top POI: %v", poi[:10])

	// Pick peaks that aren't too close.
	var res []int
	for _, p := range poi {
		if len(res) == numPoi {
			return res
		}
		var skip bool
		for _, l := range res {
			if l-10 <= p.location && p.location <= l+10 {
				skip = true
			}
		}
		if !skip {
			res = append(res, p.location)
		}
	}
	glog.Fatal("Did not find enough points-of-interest")
	return nil
}

func averageTraces(M mat.Matrix) mat.Vector {
	numTraces, _ := M.Dims()
	data := make([]float64, numTraces)
	for i := range data {
		data[i] = 1.0 / float64(numTraces)
	}
	S := mat.NewVecDense(numTraces, data)

	var avg mat.Dense
	avg.Product(mat.TransposeVec{S}, M)
	return avg.RowView(0)
}

func loadCapture(filename string) mat.Matrix {
	capture, err := gocw.LoadCapture(filename)
	if err != nil {
		glog.Fatalf("Failed to load capture: %v", err)
		return nil
	}

	return capture.SamplesMatrix()
}

// Builds template based classifier.
// The power profile at the points-of-interest is modeled as a multivariate normal
// distribution. Its mean and covariance matrix are computed from the training set.
func buildTemplate(M mat.Matrix, poi []int) *distmv.Normal {
	T := mat.DenseCopyOf(M.T())
	n := len(poi)
	mu := make([]float64, n)
	sigma := mat.NewSymDense(n, nil)
	for i := 0; i < n; i++ {
		X := T.RawRowView(poi[i])
		mu[i] = stat.Mean(X, nil)
		for j := 0; j < n; j++ {
			Y := T.RawRowView(poi[j])
			sigma.SetSym(i, j, stat.Covariance(X, Y, nil))
		}
	}
	glog.V(1).Infof("mu: %v", mu)
	glog.V(1).Infof("sigma: %v", sigma)

	ndist, pos := distmv.NewNormal(mu, sigma, nil)
	if !pos {
		glog.Fatal("Covariance matrix is not positive definite => no PDF")
	}
	return ndist
}

// Tests the classifier on the validation set.
// Computes the log probability of the observed traces at the points-of-interest.
// The model with the highest probability is selected.
func testValidationSet(validation mat.Matrix, poi []int,
	zeroDist *distmv.Normal, randDist *distmv.Normal) {
	numTraces, _ := validation.Dims()
	for i := 0; i < numTraces; i++ {
		var x []float64
		for _, p := range poi {
			x = append(x, validation.At(i, p))
		}
		glog.V(1).Infof("x: %v, zero PDF: %f, rand PDF: %f", x, zeroDist.LogProb(x), randDist.LogProb(x))
		if zeroDist.LogProb(x) > randDist.LogProb(x) {
			glog.Infof("Classified x=%v as a zero point trace", x)
		} else {
			glog.Infof("Classified x=%v as a rand point trace", x)
		}
	}
}

// Split traces: 80% for training, 20% for validation.
func splitTraces(M mat.Matrix) (mat.Matrix, mat.Matrix) {
	r, c := M.Dims()
	return M.(*mat.Dense).Slice(0, (r*80)/100, 0, c),
		M.(*mat.Dense).Slice((r*80)/100, r, 0, c)
}

func main() {
	defer glog.Flush()

	glog.Info("Loading zero-point capture")
	zeroTraces := loadCapture(*zeroCaptureFlag)
	zeroTraining, zeroValidation := splitTraces(zeroTraces)

	glog.Info("Loading rand-point capture")
	randTraces := loadCapture(*randCaptureFlag)
	randTraining, randValidation := splitTraces(randTraces)

	zeroAvg := averageTraces(zeroTraining)
	randAvg := averageTraces(randTraining)

	glog.Info("Finding points of interest")
	poi := findPointsOfInterest(zeroAvg, randAvg)
	glog.Infof("Selected POI: %v", poi)

	glog.Info("Building zero-point template")
	zeroDist := buildTemplate(zeroTraining, poi)

	glog.Info("Building rand-point template")
	randDist := buildTemplate(randTraining, poi)

	glog.Info("Testing zero-point validation set")
	testValidationSet(zeroValidation, poi, zeroDist, randDist)

	glog.Info("Testing rand-point validation set")
	testValidationSet(randValidation, poi, zeroDist, randDist)
}
