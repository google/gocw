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

// Analog-Digital-Converter (OpenADC) interface.
package gocw

import (
	"io"
)

//go:generate stringer -type HwType
type HwType int

const (
	HwUnknown               HwType = iota
	HwLx9MicroBoard         HwType = iota
	HwSaseboW               HwType = iota
	HwChipWhispererRev2Lx25 HwType = iota
	HwReserved              HwType = iota
	HwZedBoard              HwType = iota
	HwPapilioPro            HwType = iota
	HwSakuraG               HwType = iota
	HwChipWhispererLite     HwType = iota
	HwChipWhispererCw1200   HwType = iota
)

type HwVersion struct {
	RegVersion uint8
	HwType     HwType
	HwVersion  uint8
}

//go:generate stringer -type GainMode
type GainMode int

const (
	GainModeHigh GainMode = iota
	GainModeLow  GainMode = iota
)

//go:generate stringer -type TriggerMode
type TriggerMode int

const (
	TriggerModeRisingEdge  TriggerMode = iota
	TriggerModeFallingEdge TriggerMode = iota
	TriggerModeLow         TriggerMode = iota
	TriggerModeHigh        TriggerMode = iota
)

//go:generate stringer -type AdcSrc
type AdcSrc int

const (
	AdcSrcDcm    AdcSrc = iota
	AdcSrcExtClk AdcSrc = iota
)

//go:generate stringer -type DcmInput
type DcmInput int

const (
	DcmInputClkGen DcmInput = iota
	DcmInputExtClk DcmInput = iota
)

type AdcSrcTuple struct {
	AdcSrc   AdcSrc
	DcmOut   int
	DcmInput DcmInput
}

// Predefined ADC clock values
var AdcSrcExtClkDirect = AdcSrcTuple{
	AdcSrcExtClk, 4, DcmInputClkGen,
}
var AdcSrcExtClkX4ViaDcm = AdcSrcTuple{
	AdcSrcDcm, 4, DcmInputExtClk,
}
var AdcSrcExtClkX1ViaDcm = AdcSrcTuple{
	AdcSrcDcm, 1, DcmInputExtClk,
}
var AdcSrcClkGenX4ViaDcm = AdcSrcTuple{
	AdcSrcDcm, 4, DcmInputClkGen,
}
var AdcSrcClkGenX1ViaDcm = AdcSrcTuple{
	AdcSrcDcm, 1, DcmInputClkGen,
}

//go:generate stringer -type FreqCounterSrc
type FreqCounterSrc int

const (
	FreqCounterExtClkInput  FreqCounterSrc = iota
	FreqCounterClkGenOutput FreqCounterSrc = iota
)

//go:generate stringer -type ClkGenInputSrc
type ClkGenInputSrc int

const (
	ClkGenInputSystem ClkGenInputSrc = iota
	ClkGenInputExtClk ClkGenInputSrc = iota
)

//go:generate stringer -type TriggerTargetIoPin
type TriggerTargetIoPin int

const (
	TriggerTargetIoPin1 TriggerTargetIoPin = iota
	TriggerTargetIoPin2 TriggerTargetIoPin = iota
	TriggerTargetIoPin3 TriggerTargetIoPin = iota
	TriggerTargetIoPin4 TriggerTargetIoPin = iota
)

//go:generate stringer -type TargetIoMode
type TargetIoMode int

const (
	TargetIoModeSerialRx     TargetIoMode = iota
	TargetIoModeSerialTx     TargetIoMode = iota
	TargetIoModeHighZ        TargetIoMode = iota
	TargetIoModeGpioLow      TargetIoMode = iota
	TargetIoModeGpioHigh     TargetIoMode = iota
	TargetIoModeGpioDisabled TargetIoMode = iota
)

//go:generate stringer -type Hs2Mode
type Hs2Mode int

const (
	Hs2ModeDisabled Hs2Mode = iota
	Hs2ModeClkGen   Hs2Mode = iota
	Hs2ModeGlitch   Hs2Mode = iota
)

//go:generate stringer -type GpioMode
type GpioMode int

const (
	GpioLow      GpioMode = iota
	GpioHigh     GpioMode = iota
	GpioDisabled GpioMode = iota
)

//go:generate mockgen -destination=mocks/adc.go -package=mocks github.com/google/gocw AdcInterface
type AdcInterface interface {
	io.Closer
	Error() error
	//
	// Hardware information.
	//
	Version() HwVersion
	SysFreq() uint32
	MaxSamples() uint32
	//
	// Gain settings.
	//
	GainMode() GainMode
	// Sets the AD8331 Low Noise Amplifier into to "High" or \"Low\" gain mode.
	// Low mode ranges from -4.5dB to +43.5dB, and High mode ranges from +7.5dB to +55.5dB.
	// Better performance is found using the "High" gain mode typically.
	SetGainMode(mode GainMode)
	Gain() uint8
	// Sets the AD8331 gain value.
	// This is a unitless number which ranges from 0 (minimum) to 78 (maximum).
	// The resulting gain in dB is given in the "calculated" output.
	SetGain(gain uint8)
	// Gives the status of the digital signal being used as the trigger signal,
	// either high or low.
	TriggerPinState() bool
	// When using a digital system, sets the trigger mode:
	//  =============== ==============================
	//  Mode            Description
	//  =============== ==============================
	//  Rising Edge     Trigger on rising edge only.
	//  Falling Edge    Trigger on falling edge only.
	//  Low             Trigger when line is "low".
	//  High            Trigger when line is "high".
	//  =============== ==============================
	// Note the "Trigger Mode" should be set to "Rising Edge" if using either
	// the "SAD Trigger" or "IO Trigger" modes.
	// If using STREAM mode (CW-Pro only), the trigger should use a rising or
	// falling edge. Using a constant level is possible, but normally requires
	// additional delay added via "offset" (see stream mode help for details).
	TriggerMode() TriggerMode
	SetTriggerMode(mode TriggerMode)
	// Delays this many samples after the trigger event before recording samples.
	// Based on the ADC clock cycles. If using a 4x mode for example, an offset of "1000"
	// would mean we skip 250 cycles of the target device.
	TriggerOffset() uint32
	SetTriggerOffset(offset uint32)
	// Record a certain number of samples before the main samples are captured.
	// If "offset" is set to 0, this means recording samples BEFORE the trigger event.
	PreTriggerSamples() uint32
	SetPreTriggerSamples(samples uint32)
	// Total number of samples to record. Note the capture system has an upper
	// limit. Older FPGA bitstreams had a lower limit of about 256 samples.
	// For the CW-Lite/Pro, the current lower limit is 128 samples due to interactions with
	// the SAD trigger module.
	TotalSamples() uint32
	SetTotalSamples(samples uint32)
	// Downsamples incomming ADC data by throwing away the specified number of
	// samples between captures. Synchronous to the trigger so presample
	// mode is DISABLED when this value is greater than 1.
	DownsampleFactor() uint16
	SetDownsampleFactor(factor uint16)
	// Measures number of ADC clock cycles during which the trigger was active.
	// If trigger toggles more than once this may not be valid.`,
	ActiveCount() uint32
	// The ADC sample clock is generated from this source.
	// Options are either an external input (which input set elsewhere) or an internal
	// clock generator. Details of each option:
	// =================== ====================== =================== ===============
	//  Name                Description            Input Freq Range   Fine Phase Adj.
	// =================== ====================== =================== ===============
	//  EXCLK Direct       Connects sample clock     1-105 MHz            NO
	//                     external pin directly.
	//  EXTCLK xN via DCM  Takes external pin,       5-105 MHz (x1)       YES
	//                     multiplies frequency      5-26.25 MHz (x4)
	//                     xN and feeds to ADC.
	//  CLKGEN xN via DCM  Multiples CLKGEN by       5-105 MHz (x1)       YES
	//                     xN and feeds to ADC.      5-26.25 MHz (x4)
	// =================== ====================== =================== ===============
	AdcClockSource() AdcSrcTuple
	SetAdcClockSource(src AdcSrcTuple)
	AdcFreq() uint32
	// ADC Sample Rate. Takes account of decimation factor (if set).
	AdcSampleRate() uint32
	DcmLocked() bool
	// Freq Counter: Frequency of clock measured on EXTCLOCK pin in Hz.
	FreqCounter() uint32
	FreqCounterSource() FreqCounterSrc
	SetFreqCounterSource(src FreqCounterSrc)
	ClkGenInputSource() ClkGenInputSrc
	SetClkGenInputSource(src ClkGenInputSrc)
	// The input frequency from the EXTCLK source in Hz.
	//
	// This value is used to help calculate the correct CLKGEN settings to
	// obtain a desired output frequency when using EXTCLK as CLKGEN input.
	// It is not a frequency counter - it is only helpful if the EXTCLK
	// frequency is already known.
	//
	// Getter: Return the last set EXTCLK frequency in MHz (int)
	//
	// Setter: Update the EXTCLK frequency
	ExtClockFreq() uint32
	SetExtClockFreq(freq uint32)
	// CLKGEN output frequency in Hz.
	// The CLKGEN module takes the input source and multiplies/divides it
	// to get a faster or slower clock as desired.
	//
	// Getter:
	//   Return the current calculated CLKGEN output frequency in Hz
	//   (float). Note that this is the theoretical frequency - use the
	//   freq counter to determine the actual output.
	//
	// Setter:
	//   Attempt to set a new CLKGEN frequency in Hz. When this value is
	//   set, all possible DCM multiply/divide settings are tested to find
	//   which is closest to the desired output speed. If EXTCLK is the
	//   CLKGEN source, the EXTCLK frequency must be properly set for this
	//   to work. Also, both DCMs are reset.
	ClkGenOutputFreq() uint32
	SetClkGenOutputFreq(freq uint32)
	ClkGenDcmLocked() bool
	// The logical input into the trigger module.
	//
	// The trigger module uses some combination of the scope's I/O pins to
	// produce a single value, which it uses for edge/level detection or UART
	// triggers. This trigger output can combine 5 pins using one of 3
	// different boolean operations.
	//
	TriggerTargetIoPins() []TriggerTargetIoPin
	SetTriggerTargetIoPin(pin TriggerTargetIoPin)
	//
	// GPIO settings.
	//
	// Thr function of the Target IO1 pin.
	TargetIo1() TargetIoMode
	SetTargetIo1(mode TargetIoMode)
	// Thr function of the Target IO2 pin.
	TargetIo2() TargetIoMode
	SetTargetIo2(mode TargetIoMode)
	// Special GPIO: NRST
	NRST() GpioMode
	SetNRST(mode GpioMode)
	// Special GPIO: PDIC
	PDIC() GpioMode
	SetPDIC(mode GpioMode)
	// Special GPIO: PDID
	PDID() GpioMode
	SetPDID(mode GpioMode)
	// The clock signal routed to the HS2 high speed output pin.
	Hs2() Hs2Mode
	SetHs2(mode Hs2Mode)
	//
	// Capture settings.
	//
	SetArmOn()
	SetArmOff()
	WaitForTigger() bool
	TraceData() []float64
}
