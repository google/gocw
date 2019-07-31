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

// Collects power trace measurements.
package gocw

import (
	"compress/gzip"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/golang/glog"
	"gonum.org/v1/gonum/mat"
)

type Trace struct {
	Key               []byte    `json:"k"`
	Pt                []byte    `json:"pt"`
	Ct                []byte    `json:"ct"`
	PowerMeasurements []float64 `json:"pm"`
}

type Capture []Trace

type PtGen func() ([]byte, error)

// Generates random plaintext for each trace.
func RandGen(numBytes int) PtGen {
	return func() ([]byte, error) {
		buf := make([]byte, numBytes)
		if _, err := rand.Read(buf); err != nil {
			return nil, err
		}
		return buf, nil
	}
}

// Captures a set traces.
// Retries on transient errors.
func NewCapture(key []byte, ptGen PtGen, numSamples, numTraces, offset int) (Capture, error) {
	var err error

	var dev UsbDeviceInterface
	if dev, err = OpenCwLiteUsbDevice(); err != nil {
		return nil, err
	}
	defer dev.Close()

	var fpga *Fpga
	if fpga, err = NewFpga(dev); err != nil {
		return nil, err
	}

	var adc *Adc
	if adc, err = NewAdc(fpga); err != nil {
		return nil, err
	}
	defer adc.Close()

	adc.SetTotalSamples(uint32(numSamples))
	adc.SetTriggerOffset(uint32(offset))

	var usart *Usart
	if usart, err = NewUsart(dev, nil); err != nil {
		return nil, err
	}

	var ser *SimpleSerial
	if ser, err = NewSimpleSerial(usart); err != nil {
		return nil, err
	}

	if err = ser.WriteKey(key); err != nil {
		return nil, err
	}

	var capture Capture
	for len(capture) < numTraces {
		if err = adc.Error(); err != nil {
			return nil, err
		}

		glog.Infof("Starting trace [%d/%d]\n", len(capture)+1, numTraces)
		trace := Trace{}
		trace.Key = key

		// Generate plaintext for this trace.
		if trace.Pt, err = ptGen(); err != nil {
			return nil, err
		}

		adc.SetArmOn()

		if err = ser.WritePlaintext(trace.Pt); err != nil {
			return nil, err
		}

		timedOut := adc.WaitForTigger()
		if timedOut {
			glog.Warning("Timed out during capture. Re-trying")
			continue
		}

		if trace.Ct, err = ser.Response(); err != nil {
			return nil, err
		}

		trace.PowerMeasurements = adc.TraceData()
		if len(trace.PowerMeasurements) == 0 {
			glog.Warning("TraceData did not return measurements. Re-trying")
			continue
		}

		capture = append(capture, trace)
	}

	return capture, nil
}

// Exported for testing.
func LoadCaptureIo(src io.Reader) (Capture, error) {
	var capture Capture
	zipper, err := gzip.NewReader(src)
	if err != nil {
		return nil, fmt.Errorf("gzip NewReader failed %v", err)
	}
	decoder := json.NewDecoder(zipper)
	if err = decoder.Decode(&capture); err != nil {
		return nil, fmt.Errorf("JSON decoder failed %v", err)
	}
	return capture, nil
}

// Loads capture from file.
func LoadCapture(filename string) (Capture, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("Error opening capture file: %v", err)
	}
	defer f.Close()
	return LoadCaptureIo(f)
}

// Exported for testing.
func (c Capture) SaveIo(dst io.Writer) error {
	var err error
	zipper := gzip.NewWriter(dst)
	encoder := json.NewEncoder(zipper)
	if err = encoder.Encode(c); err != nil {
		return fmt.Errorf("JSON encoder failed %v", err)
	}
	if err = zipper.Close(); err != nil {
		return fmt.Errorf("gzip close failed %v", err)
	}
	return nil
}

func (c Capture) Save(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("Error creating capture file: %v", err)
	}
	defer f.Close()
	return c.SaveIo(f)
}

// Collects all samples in a single m (#traces) by n (#samples) matrix.
//  _         _
// | -- T1  -- |
// | -- T2  -- |
// | -- ..  -- |
// | -- TM  -- |
// |_         _|
//
func (c Capture) SamplesMatrix() mat.Matrix {
	rows := len(c)
	cols := len(c[0].PowerMeasurements)
	data := make([]float64, rows*cols)
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			data[i*cols+j] = c[i].PowerMeasurements[j]
		}
	}
	return mat.NewDense(rows, cols, data)
}
