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

// Analog-Digital-Converter (OpenADC). Implements AdcInterface.
package gocw

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/golang/glog"
)

const (
	addrGain       Address = 0
	addrSettings   Address = 1
	addrStatus     Address = 2
	addrAdcData    Address = 3
	addrEcho       Address = 4
	addrFreq       Address = 5
	addrAdvClk     Address = 6
	addrSysFreq    Address = 7
	addrAdcFreq    Address = 8
	addrPhase      Address = 9
	addrVersions   Address = 10
	addrOffset     Address = 26
	addrDecimate   Address = 15
	addrSamples    Address = 16
	addrPresamples Address = 17
	addrBytestorx  Address = 18
	addrTriggerDur Address = 20
	addrMultiEcho  Address = 34
	addrTrigSrc    Address = 39
	addrExtClk     Address = 38
	addrIoRoute    Address = 55
)

const (
	settingsReset    uint8 = 0x01
	settingsGainHigh uint8 = 0x02
	settingsGainLow  uint8 = 0x00
	settingsTrigHigh uint8 = 0x04
	settingsTrigLow  uint8 = 0x00
	settingsArm      uint8 = 0x08
	settingsWaitYes  uint8 = 0x20
	settingsWaitNo   uint8 = 0x00
	settingsTrigNow  uint8 = 0x40
)

const (
	statusArmMask      uint8 = 0x01
	statusFifoMask     uint8 = 0x02
	statusExtMask      uint8 = 0x04
	statusDcmMask      uint8 = 0x08
	statusDdrcalMask   uint8 = 0x10
	statusDdrerrMask   uint8 = 0x20
	statusDdrmodeMask  uint8 = 0x40
	statusOverflowMask uint8 = 0x80
)

const (
	pinRtio1 uint8 = 0x04
	pinRtio2 uint8 = 0x08
	pinRtio3 uint8 = 0x10
	pinRtio4 uint8 = 0x20
	modeOr   uint8 = 0x00
	modeAnd  uint8 = 0x01
	modenand uint8 = 0x02
)

const (
	ioRouteHighZ   uint8 = 0x00
	ioRouteSTX     uint8 = 0x01
	ioRouteSRX     uint8 = 0x02
	ioRouteUSIO    uint8 = 0x04
	ioRouteUSII    uint8 = 0x08
	ioRouteUSInOut uint8 = 0x18
	ioRouteSTxRx   uint8 = 0x22
	ioRouteGpio    uint8 = 0x40
	ioRouteGpioE   uint8 = 0x80
)

// Special IO pins.
const (
	nrstPinNum int = 100
	pdidPinNum int = 101
	pdicPinNum int = 102
)

var unknownHwVersion = HwVersion{0, HwUnknown, 0}

type AdvClkSettings struct {
	SrcAndStatus uint8
	Mul          uint8
	Div          uint8
	ClkGenFlags  uint8
}

var clkReadMask = []byte{0x1f, 0xff, 0xff, 0xfd}

type Adc struct {
	fpga         *Fpga
	err          error
	hwMaxSamples uint32
	extClockFreq uint32
}

func (c *Adc) Close() error {
	return nil
}

func (c *Adc) Error() error {
	return c.err
}

//
// Hardware information.
//
func (c *Adc) Version() HwVersion {
	if c.err != nil {
		return unknownHwVersion
	}
	buf := make([]byte, 6)
	if c.err = c.fpga.Mem.Read(addrVersions, buf); c.err != nil {
		return unknownHwVersion
	}
	ver := HwVersion{}
	ver.RegVersion = buf[0] & 0xff
	ver.HwType = HwType(buf[1] >> 3)
	ver.HwVersion = buf[1] & 0x07
	return ver
}

func (c *Adc) SysFreq() uint32 {
	if c.err != nil {
		return 0
	}
	var freq uint32
	if c.err = c.fpga.Mem.Read(addrSysFreq, &freq); c.err != nil {
		return 0
	}
	return freq
}

func (c *Adc) MaxSamples() uint32 {
	return c.hwMaxSamples
}

//
// Gain settings.
//
func (c *Adc) GainMode() GainMode {
	if (c.settings() & settingsGainHigh) > 0 {
		return GainModeHigh
	} else {
		return GainModeLow
	}
}

func (c *Adc) SetGainMode(mode GainMode) {
	if mode == GainModeHigh {
		c.setSettings(c.settings()|settingsGainHigh, true)
	} else { // mode == GainModeLow
		c.setSettings(c.settings()&(^settingsGainHigh), true)
	}
}

func (c *Adc) Gain() uint8 {
	if c.err != nil {
		return 0
	}
	var gain uint8
	if c.err = c.fpga.Mem.Read(addrGain, &gain); c.err != nil {
		return 0
	}
	return gain
}

func (c *Adc) SetGain(gain uint8) {
	if c.err != nil {
		return
	}
	if gain > 78 {
		c.err = fmt.Errorf("Invalid gain (%v), range 0-78 only", gain)
		return
	}
	c.err = c.fpga.Mem.Write(addrGain, &gain, true, nil)
}

//
// Base trigger settings.
//
func (c *Adc) TriggerPinState() bool {
	return (c.status()&statusExtMask > 0)
}

func (c *Adc) TriggerMode() TriggerMode {
	settings := c.settings()
	switch c := settings & (settingsTrigHigh | settingsWaitYes); c {
	case settingsTrigHigh | settingsWaitYes:
		return TriggerModeRisingEdge
	case settingsTrigLow | settingsWaitYes:
		return TriggerModeFallingEdge
	case settingsTrigHigh | settingsWaitNo:
		return TriggerModeHigh
	default:
		return TriggerModeLow
	}
}

func (c *Adc) SetTriggerMode(mode TriggerMode) {
	settings := c.settings()
	settings &= ^(settingsTrigHigh | settingsWaitYes)
	switch mode {
	case TriggerModeRisingEdge:
		settings |= settingsTrigHigh | settingsWaitYes
	case TriggerModeFallingEdge:
		settings |= settingsTrigLow | settingsWaitYes
	case TriggerModeHigh:
		settings |= settingsTrigHigh | settingsWaitNo
	case TriggerModeLow:
		settings |= settingsTrigLow | settingsWaitNo
	}
	c.setSettings(settings, true)
}

func (c *Adc) TriggerOffset() uint32 {
	if c.err != nil {
		return 0
	}
	var offset uint32
	if c.err = c.fpga.Mem.Read(addrOffset, &offset); c.err != nil {
		return 0
	}
	return offset
}

func (c *Adc) SetTriggerOffset(offset uint32) {
	if c.err != nil {
		return
	}
	c.err = c.fpga.Mem.Write(addrOffset, offset, true, nil)
}

func (c *Adc) PreTriggerSamples() uint32 {
	if c.err != nil {
		return 0
	}
	var samples uint32
	if c.err = c.fpga.Mem.Read(addrPresamples, &samples); c.err != nil {
		return 0
	}
	ver := c.Version()

	// CW1200/CW-Lite reports presamples using different method
	if ver.HwType == HwChipWhispererLite || ver.HwType == HwChipWhispererCw1200 {
		samples *= 3
	}
	return samples
}

func (c *Adc) SetPreTriggerSamples(samples uint32) {
	if c.err != nil {
		return
	}
	ver := c.Version()
	if c.err != nil {
		return
	}
	if ver.HwType != HwChipWhispererLite && ver.HwType != HwChipWhispererCw1200 {
		c.err = fmt.Errorf("Not reliable on hardware")
		return
	}
	c.err = c.fpga.Mem.Write(addrPresamples, samples, true, nil)
}

func (c *Adc) TotalSamples() uint32 {
	return c.numSamples()
}

func (c *Adc) SetTotalSamples(samples uint32) {
	if samples > c.hwMaxSamples {
		c.err = fmt.Errorf("samples (%v) outside limit (%v)", samples, c.hwMaxSamples)
		return
	}
	c.setNumSamples(samples)
}

func (c *Adc) DownsampleFactor() uint16 {
	return c.decimate()
}
func (c *Adc) SetDownsampleFactor(factor uint16) {
	c.setDecimate(factor)
}

func (c *Adc) ActiveCount() uint32 {
	if c.err != nil {
		return 0
	}
	var count uint32
	if c.err = c.fpga.Mem.Read(addrTriggerDur, &count); c.err != nil {
		return 0
	}
	return count
}

//
// Clock settings.
//
func (c *Adc) AdcClockSource() AdcSrcTuple {
	var src AdcSrcTuple
	if c.err != nil {
		return src
	}
	settings := c.advClock()
	settings.SrcAndStatus &= 0x07
	if settings.SrcAndStatus&0x04 > 0 {
		src.DcmInput = DcmInputExtClk
	} else {
		src.DcmInput = DcmInputClkGen
	}

	if settings.SrcAndStatus&0x02 > 0 {
		src.DcmOut = 1
	} else {
		src.DcmOut = 4
	}

	if settings.SrcAndStatus&0x01 > 0 {
		src.AdcSrc = AdcSrcExtClk
	} else {
		src.AdcSrc = AdcSrcDcm
	}

	return src
}

func (c *Adc) SetAdcClockSource(src AdcSrcTuple) {
	if c.err != nil {
		return
	}
	settings := c.advClock()
	settings.SrcAndStatus &= ^uint8(0x07)

	switch src.DcmInput {
	case DcmInputClkGen:
		break
	case DcmInputExtClk:
		settings.SrcAndStatus |= 0x04
	default:
		c.err = fmt.Errorf("Invalid DcmInput value")
		return
	}

	switch src.DcmOut {
	case 4:
		break
	case 1:
		settings.SrcAndStatus |= 0x02
	default:
		c.err = fmt.Errorf("Invalid DcmOut value")
		return
	}

	switch src.AdcSrc {
	case AdcSrcDcm:
		break
	case AdcSrcExtClk:
		settings.SrcAndStatus |= 0x01
	default:
		c.err = fmt.Errorf("Invalid AdcSrc value")
		return
	}
	c.setAdvClock(settings, true)
	c.resetAdc()
}

func (c *Adc) AdcFreq() uint32 {
	if c.err != nil {
		return 0
	}
	sysFreq := c.SysFreq()
	if c.err != nil {
		return 0
	}

	var adcFreq uint32
	if c.err = c.fpga.Mem.Read(addrAdcFreq, &adcFreq); c.err != nil {
		return 0
	}

	sampleFreq := float64(sysFreq) / float64(1<<23)
	return uint32(float64(adcFreq) * sampleFreq)
}

// ADC Sample Rate. Takes account of decimation factor (if set).
func (c *Adc) AdcSampleRate() uint32 {
	if c.err != nil {
		return 0
	}
	decimation := c.decimate()
	return c.AdcFreq() / uint32(decimation)
}

func (c *Adc) DcmLocked() bool {
	return (c.advClock().SrcAndStatus&0x40 > 0)
}

func (c *Adc) FreqCounter() uint32 {
	if c.err != nil {
		return 0
	}
	sysFreq := c.SysFreq()
	if c.err != nil {
		return 0
	}

	var extFreq uint32
	if c.err = c.fpga.Mem.Read(addrFreq, &extFreq); c.err != nil {
		return 0
	}

	sampleFreq := float64(sysFreq) / float64(1<<23)
	return uint32(float64(extFreq) * sampleFreq)
}

func (c *Adc) FreqCounterSource() FreqCounterSrc {
	if c.advClock().ClkGenFlags&0x08 > 0 {
		return FreqCounterClkGenOutput
	} else {
		return FreqCounterExtClkInput
	}
}

func (c *Adc) SetFreqCounterSource(src FreqCounterSrc) {
	if c.err != nil {
		return
	}
	settings := c.advClock()
	settings.ClkGenFlags &= ^uint8(0x08)
	switch src {
	case FreqCounterExtClkInput:
		break
	case FreqCounterClkGenOutput:
		settings.ClkGenFlags |= 0x08
		break
	}
	c.setAdvClock(settings, true)
	c.resetClkGen()
	c.resetAdc()
}

func (c *Adc) ClkGenInputSource() ClkGenInputSrc {
	if c.advClock().SrcAndStatus&0x08 > 0 {
		return ClkGenInputExtClk
	} else {
		return ClkGenInputSystem
	}
}

func (c *Adc) SetClkGenInputSource(src ClkGenInputSrc) {
	if c.err != nil {
		return
	}
	settings := c.advClock()
	settings.SrcAndStatus &= ^uint8(0x08)
	switch src {
	case ClkGenInputSystem:
		break
	case ClkGenInputExtClk:
		settings.SrcAndStatus |= 0x08
		break
	}
	c.setAdvClock(settings, true)
}

func (c *Adc) ExtClockFreq() uint32 {
	return c.extClockFreq
}

func (c *Adc) SetExtClockFreq(freq uint32) {
	if c.err != nil {
		return
	}
	c.extClockFreq = freq
}

func (c *Adc) ClkGenOutputFreq() uint32 {
	if c.err != nil {
		return 0
	}
	var inpFreq uint32
	switch c.ClkGenInputSource() {
	case ClkGenInputExtClk:
		inpFreq = c.ExtClockFreq()
	case ClkGenInputSystem:
		inpFreq = c.SysFreq()
	}
	mul := c.clkGenMul()
	div := c.clkGenDiv()
	return (inpFreq * mul) / div
}

func (c *Adc) SetClkGenOutputFreq(freq uint32) {
	if c.err != nil {
		return
	}
	var inpFreq uint32
	switch c.ClkGenInputSource() {
	case ClkGenInputExtClk:
		inpFreq = c.ExtClockFreq()
	case ClkGenInputSystem:
		inpFreq = c.SysFreq()
	}
	mul, div := calcClkGenMulDiv(int(freq), int(inpFreq))
	c.setClkGenMul(uint32(mul))
	c.setClkGenDiv(uint32(div))
	c.resetClkGen()
	c.resetAdc()
}

func (c *Adc) ClkGenDcmLocked() bool {
	return (c.advClock().SrcAndStatus&0x20 > 0)
}

//
// Trigger settings.
//

// TODO(cfir): add boolean operations support.
func (c *Adc) TriggerTargetIoPins() []TriggerTargetIoPin {
	var res []TriggerTargetIoPin
	if c.err != nil {
		return res
	}
	var pins uint8
	if c.err = c.fpga.Mem.Read(addrTrigSrc, &pins); c.err != nil {
		return res
	}
	if pins&pinRtio1 > 0 {
		res = append(res, TriggerTargetIoPin1)
	}
	if pins&pinRtio2 > 0 {
		res = append(res, TriggerTargetIoPin2)
	}
	if pins&pinRtio3 > 0 {
		res = append(res, TriggerTargetIoPin3)
	}
	if pins&pinRtio4 > 0 {
		res = append(res, TriggerTargetIoPin4)
	}
	return res
}

func (c *Adc) SetTriggerTargetIoPin(pin TriggerTargetIoPin) {
	if c.err != nil {
		return
	}
	var pins uint8
	switch pin {
	case TriggerTargetIoPin1:
		pins |= pinRtio1
	case TriggerTargetIoPin2:
		pins |= pinRtio2
	case TriggerTargetIoPin3:
		pins |= pinRtio3
	case TriggerTargetIoPin4:
		pins |= pinRtio4
	default:
		c.err = fmt.Errorf("Invalid pin %v", pin)
		return
	}
	pins |= (modeOr << 6)
	c.err = c.fpga.Mem.Write(addrTrigSrc, &pins, true, nil)
}

//
// GPIO settings.
//
func (c *Adc) TargetIo1() TargetIoMode {
	return c.targetIo(0)
}
func (c *Adc) SetTargetIo1(mode TargetIoMode) {
	c.setTargetIo(0, mode)
}

func (c *Adc) TargetIo2() TargetIoMode {
	return c.targetIo(1)
}
func (c *Adc) SetTargetIo2(mode TargetIoMode) {
	c.setTargetIo(1, mode)
}

func (c *Adc) NRST() GpioMode {
	return c.specialGpio(nrstPinNum)
}
func (c *Adc) SetNRST(mode GpioMode) {
	c.setSpecialGpio(nrstPinNum, mode)
}

func (c *Adc) PDIC() GpioMode {
	return c.specialGpio(pdicPinNum)
}
func (c *Adc) SetPDIC(mode GpioMode) {
	c.setSpecialGpio(pdicPinNum, mode)
}

func (c *Adc) PDID() GpioMode {
	return c.specialGpio(pdidPinNum)
}
func (c *Adc) SetPDID(mode GpioMode) {
	c.setSpecialGpio(pdidPinNum, mode)
}

func (c *Adc) Hs2() Hs2Mode {
	switch c.targetClkOut() {
	case 0:
		return Hs2ModeDisabled
	case 2:
		return Hs2ModeClkGen
	case 3:
		return Hs2ModeGlitch
	default:
		c.err = fmt.Errorf("Failed to get target clk")
	}
	return 0
}
func (c *Adc) SetHs2(mode Hs2Mode) {
	if c.err != nil {
		return
	}
	switch mode {
	case Hs2ModeDisabled:
		c.setTargetClkOut(0)
	case Hs2ModeClkGen:
		c.setTargetClkOut(2)
	case Hs2ModeGlitch:
		c.setTargetClkOut(3)
	}
}

//
// Capture settings.
//
func (c *Adc) SetArmOn() {
	c.setSettings(c.settings()|settingsArm, true)
}

func (c *Adc) SetArmOff() {
	c.setSettings(c.settings() & ^settingsArm, true)
}

func (c *Adc) WaitForTigger() bool {
	var wg sync.WaitGroup
	timedOut := time.NewTimer(2 * time.Second)
	var ret bool

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-timedOut.C:
				glog.Warning("Timed out waiting for trigger. Forcing trigger")
				c.setTriggerNow()
				ret = true
				return
			default:
				status := c.status()
				if status&statusArmMask != statusArmMask &&
					status&statusFifoMask != 0 {
					glog.V(1).Infof("triggered! (status = %v)", status)
					return
				}
			}
		}
	}()
	wg.Wait()
	c.SetArmOff()
	return ret
}

func (c *Adc) TraceData() []float64 {
	var pending uint32
	if c.err = c.fpga.Mem.Read(addrBytestorx, &pending); c.err != nil {
		return nil
	}
	if pending == 0 {
		return nil
	}
	// If pending is huge, we only read what is needed
	// Bytes get packed 3 samples / 4 bytes
	// Add some extra in case needed
	samples := c.numSamples()
	toRead := samples
	if toRead%3 != 0 {
		toRead += 3 - toRead%3
	}
	toRead = (toRead*4)/3 + 256
	if pending < toRead {
		toRead = pending
	}

	glog.V(1).Infof("Reading trace data. samples: %v, toRead: %v", samples, toRead)
	data := make([]byte, toRead)
	if c.err = c.fpga.Mem.Read(addrAdcData, data); c.err != nil {
		c.err = fmt.Errorf("Failed reading trace data: %v", c.err)
		return nil
	}

	measurements := c.ProcessTraceData(data)
	if c.err != nil {
		return nil
	}

	if len(measurements) > int(samples) {
		measurements = measurements[:samples]
	}

	return measurements
}

//
// Support functions.
//
func (c *Adc) status() uint8 {
	if c.err != nil {
		return 0
	}
	var status uint8
	if c.err = c.fpga.Mem.Read(addrStatus, &status); c.err != nil {
		return 0
	}
	return status
}

func (c *Adc) settings() uint8 {
	if c.err != nil {
		return 0
	}
	var settings uint8
	c.err = c.fpga.Mem.Read(addrSettings, &settings)
	return settings
}

func (c *Adc) setSettings(settings uint8, validate bool) {
	if c.err != nil {
		return
	}
	c.err = c.fpga.Mem.Write(addrSettings, &settings, validate, nil)
}

func (c *Adc) numSamples() uint32 {
	if c.err != nil {
		return 0
	}
	var samples uint32
	if c.err = c.fpga.Mem.Read(addrSamples, &samples); c.err != nil {
		return 0
	}
	return samples
}

func (c *Adc) setNumSamples(n uint32) {
	if c.err != nil {
		return
	}
	c.err = c.fpga.Mem.Write(addrSamples, &n, true, nil)
}

func (c *Adc) decimate() uint16 {
	if c.err != nil {
		return 0
	}
	var n uint16
	if c.err = c.fpga.Mem.Read(addrDecimate, &n); c.err != nil {
		return 0
	}
	return n + 1
}

func (c *Adc) setDecimate(n uint16) {
	if c.err != nil {
		return
	}
	n -= 1
	c.err = c.fpga.Mem.Write(addrDecimate, &n, true, nil)
}

func (c *Adc) advClock() AdvClkSettings {
	var settings AdvClkSettings
	if c.err != nil {
		return settings
	}
	if c.err = c.fpga.Mem.Read(addrAdvClk, &settings); c.err != nil {
		return settings
	}
	return settings
}

func (c *Adc) setAdvClock(settings AdvClkSettings, validate bool) {
	if c.err != nil {
		return
	}
	c.err = c.fpga.Mem.Write(addrAdvClk, &settings, validate, clkReadMask)
}

// The multiplier in the CLKGEN DCM.
// This multiplier must be in the range [2, 256].
//
// Getter: Return the current CLKGEN multiplier (integer)
//
// Setter: Set a new CLKGEN multiplier.
func (c *Adc) clkGenMul() uint32 {
	if c.err != nil {
		return 0
	}
	for tries := 0; tries < 2; tries++ {
		settings := c.advClock()
		if settings.Mul == 0 {
			// Fix initialization on FPGA
			c.setClkGenMul(2)
			continue
		}

		if settings.ClkGenFlags&0x02 > 0 {
			return uint32(settings.Mul) + 1
		}

		c.reloadClkGen()
	}
	c.err = fmt.Errorf("Failed to read CLKGEN MUL value")
	return 0
}

func (c *Adc) setClkGenMul(mul uint32) {
	if c.err != nil {
		return
	}
	if mul < 2 {
		c.err = fmt.Errorf("mul %v out of range", mul)
		return
	}
	settings := c.advClock()
	settings.Mul = uint8(mul) - 1
	settings.ClkGenFlags |= 0x01
	c.setAdvClock(settings, true)
	settings.ClkGenFlags &= ^uint8(0x01)
	c.setAdvClock(settings, true)
}

func (c *Adc) reloadClkGen() {
	if c.err != nil {
		return
	}
	settings := c.advClock()
	settings.ClkGenFlags |= 0x01
	c.setAdvClock(settings, true)
	settings.ClkGenFlags &= ^uint8(0x01)
	c.setAdvClock(settings, true)
}

func (c *Adc) resetClkGen() {
	if c.err != nil {
		return
	}
	settings := c.advClock()
	settings.ClkGenFlags |= 0x04
	c.setAdvClock(settings, false)
	settings.ClkGenFlags &= ^uint8(0x04)
	c.setAdvClock(settings, false)
	c.reloadClkGen()
}

func (c *Adc) resetAdc() {
	if c.err != nil {
		return
	}
	settings := c.advClock()
	settings.SrcAndStatus |= 0x10
	c.setAdvClock(settings, false)
	settings.SrcAndStatus &= ^uint8(0x10)
	c.setAdvClock(settings, false)
}

// The divider in the CLKGEN DCM.
// This divider must be in the range [1, 256].
//
// Getter: Return the current CLKGEN divider (integer)
//
// Setter: Set a new CLKGEN divider.
func (c *Adc) clkGenDiv() uint32 {
	if c.err != nil {
		return 1
	}
	for tries := 0; tries < 2; tries++ {
		settings := c.advClock()
		if settings.ClkGenFlags&0x02 > 0 {
			return uint32(settings.Div) + 1
		}

		c.reloadClkGen()
	}
	c.err = fmt.Errorf("Failed to read CLKGEN DIV value")
	return 1
}

func (c *Adc) setClkGenDiv(div uint32) {
	if c.err != nil {
		return
	}
	if div < 1 {
		c.err = fmt.Errorf("div %v out of range", div)
		return
	}
	settings := c.advClock()
	settings.Div = uint8(div) - 1
	settings.ClkGenFlags |= 0x01
	c.setAdvClock(settings, true)
	settings.ClkGenFlags &= ^uint8(0x01)
	c.setAdvClock(settings, true)
}

// Calculate Multiply & Divide settings based on input frequency.
func calcClkGenMulDiv(freq, inpFreq int) (int, int) {
	var bestMul, bestDiv int

	// Max setting for divide is 60 (see datasheet)
	// Multiply is 2-256
	lowError := 1e99

	var maxDiv int
	// From datasheet, if input freq is < 52MHz limit max divide
	if inpFreq < 52e6 {
		maxDiv = int(inpFreq / 0.5E6)
	} else {
		maxDiv = 256
	}

	for mul := 2; mul < 257; mul++ {
		for div := 1; div < maxDiv; div++ {
			err := math.Abs(float64(freq - ((inpFreq * mul) / div)))
			if err < lowError {
				lowError = err
				bestMul, bestDiv = mul, div
			}
		}
	}

	return bestMul, bestDiv
}

func (c *Adc) tio(pinnum int) uint8 {
	if c.err != nil {
		return 0
	}
	buf := make([]byte, 8)
	if c.err = c.fpga.Mem.Read(addrIoRoute, buf); c.err != nil {
		return 0
	}
	// Don't include GPIO state in mode check
	switch mode := buf[pinnum] & ^ioRouteGpio; mode {
	case ioRouteSTX, ioRouteSRX, ioRouteUSIO, ioRouteUSII, ioRouteGpioE, ioRouteHighZ:
		return mode
	default:
		c.err = fmt.Errorf("Unknown TIO mode %v", mode)
		return 0
	}
}

func (c *Adc) setTio(pinnum int, mode uint8) {
	if c.err != nil {
		return
	}
	buf := make([]byte, 8)
	if c.err = c.fpga.Mem.Read(addrIoRoute, buf); c.err != nil {
		return
	}
	buf[pinnum] = mode
	c.err = c.fpga.Mem.Write(addrIoRoute, buf, true, nil)
}

func (c *Adc) gpio(pinnum int) GpioMode {
	if c.err != nil {
		return 0
	}
	buf := make([]byte, 8)
	if c.err = c.fpga.Mem.Read(addrIoRoute, buf); c.err != nil {
		return 0
	}
	if buf[pinnum]&ioRouteGpioE == 0 {
		return GpioDisabled
	}
	if buf[pinnum]&ioRouteGpio > 0 {
		return GpioHigh
	} else {
		return GpioLow
	}
}

func (c *Adc) setGpio(pinnum int, mode GpioMode) {
	if c.err != nil {
		return
	}
	buf := make([]byte, 8)
	if c.err = c.fpga.Mem.Read(addrIoRoute, buf); c.err != nil {
		return
	}
	if buf[pinnum]&ioRouteGpioE == 0 {
		c.err = fmt.Errorf("TargetIO %v is not in GPIO mode", pinnum)
		return
	}
	switch mode {
	case GpioHigh:
		buf[pinnum] |= ioRouteGpio
	case GpioLow:
		buf[pinnum] &= ^ioRouteGpio
	}
	c.err = c.fpga.Mem.Write(addrIoRoute, buf, true, nil)
}

// Special GPIO nRST, PDID, PDIC.
func (c *Adc) specialGpio(pinnum int) GpioMode {
	if c.err != nil {
		return 0
	}
	buf := make([]byte, 8)
	if c.err = c.fpga.Mem.Read(addrIoRoute, buf); c.err != nil {
		return 0
	}
	var bitnum uint
	switch pinnum {
	case nrstPinNum:
		bitnum = 0
	case pdidPinNum:
		bitnum = 2
	case pdicPinNum:
		bitnum = 4
	}
	if buf[6]&(1<<bitnum) == 0 {
		return GpioDisabled
	}
	if buf[6]&(1<<(bitnum+1)) > 0 {
		return GpioHigh
	} else {
		return GpioLow
	}
}

func (c *Adc) setSpecialGpio(pinnum int, mode GpioMode) {
	if c.err != nil {
		return
	}
	buf := make([]byte, 8)
	if c.err = c.fpga.Mem.Read(addrIoRoute, buf); c.err != nil {
		return
	}

	var bitnum uint
	switch pinnum {
	case nrstPinNum:
		bitnum = 0
	case pdidPinNum:
		bitnum = 2
	case pdicPinNum:
		bitnum = 4
	}

	switch mode {
	case GpioDisabled:
		buf[6] &= ^uint8(1 << bitnum)
	case GpioHigh:
		buf[6] |= (1 << bitnum)
		buf[6] |= (1 << (bitnum + 1))
	case GpioLow:
		buf[6] |= (1 << bitnum)
		buf[6] &= ^uint8(1 << (bitnum + 1))
	}
	c.err = c.fpga.Mem.Write(addrIoRoute, buf, true, nil)
}

func (c *Adc) targetIo(pinnum int) TargetIoMode {
	var mode TargetIoMode
	if c.err != nil {
		return mode
	}
	tioMode := c.tio(pinnum)
	if tioMode == ioRouteGpioE {
		switch c.gpio(pinnum) {
		case GpioLow:
			return TargetIoModeGpioLow
		case GpioHigh:
			return TargetIoModeGpioHigh
		case GpioDisabled:
			return TargetIoModeGpioDisabled
		}
	} else {
		switch tioMode {
		case ioRouteSTX:
			return TargetIoModeSerialTx
		case ioRouteSRX:
			return TargetIoModeSerialRx
		default:
			c.err = fmt.Errorf("Unsupported tio mode %v", tioMode)
		}
	}
	return 0
}

func (c *Adc) setTargetIo(pinnum int, mode TargetIoMode) {
	if c.err != nil {
		return
	}
	switch mode {
	case TargetIoModeSerialRx:
		c.setTio(pinnum, ioRouteSRX)
	case TargetIoModeSerialTx:
		c.setTio(pinnum, ioRouteSTX)
	case TargetIoModeGpioLow:
		c.setTio(pinnum, ioRouteGpioE)
		c.setGpio(pinnum, GpioLow)
	case TargetIoModeGpioHigh:
		c.setTio(pinnum, ioRouteGpioE)
		c.setGpio(pinnum, GpioHigh)
	default:
		c.err = fmt.Errorf("Unsupported TIO mode %v", mode)
	}
}

func (c *Adc) targetClkOut() uint8 {
	if c.err != nil {
		return 0
	}
	var data uint8
	if c.err = c.fpga.Mem.Read(addrExtClk, &data); c.err != nil {
		return 0
	}

	return (data & (3 << 5)) >> 5
}

func (c *Adc) setTargetClkOut(clkout uint8) {
	if c.err != nil {
		return
	}
	var data uint8
	if c.err = c.fpga.Mem.Read(addrExtClk, &data); c.err != nil {
		return
	}
	data &= ^uint8(3 << 5)
	data |= clkout << 5
	c.err = c.fpga.Mem.Write(addrExtClk, &data, true, nil)
}

func (c *Adc) setTriggerNow() {
	if c.err != nil {
		return
	}
	initial := c.settings()
	c.setSettings(initial|settingsTrigNow, true)
	c.setSettings(initial & ^settingsTrigNow, true)
}

// Converts encoded data samples to float measurements.
// Exported for testing.
func (c *Adc) ProcessTraceData(data []byte) []float64 {
	glog.V(1).Infof("Processing %d trace data samples", len(data))

	offset := float64(0.5)
	glog.V(1).Infof("Trigger offset (hardcoded): %v", offset)

	if len(data) < 4 || len(data)%4 != 0 {
		c.err = fmt.Errorf("Unexpected data length (%v)", len(data))
		return nil
	}

	if data[0] != 0xac {
		c.err = fmt.Errorf("Unexpected sync byte %x", data[0])
		return nil
	}

	var measurements []float64
	triggerFound := false
	for i := 1; i < len(data)-3; i += 4 {
		// Read off 4 bytes
		var word uint32
		r := bytes.NewReader(data[i : i+4])
		if c.err = binary.Read(r, binary.BigEndian, &word); c.err != nil {
			return nil
		}

		// Convert to float samples.
		w1 := word & 0x3ff
		w2 := (word >> 10) & 0x3ff
		w3 := (word >> 20) & 0x3ff

		m1 := float64(w1)/1024.0 - offset
		m2 := float64(w2)/1024.0 - offset
		m3 := float64(w3)/1024.0 - offset

		// Skip samples before the trigger.
		trigger := word >> 30
		if !triggerFound {
			// trigger = 3 -> []
			// trigger = 2 -> [m3]
			// trigger = 1 -> [m2, m3]
			// trigger = 0 -> [m1, m2, m3]
			if trigger == 3 {
				glog.V(2).Infof("Skipping sample %d (%x) before trigger", i, word)
				continue
			}
			if trigger < 3 {
				measurements = append(measurements, m3)
			}
			if trigger < 2 {
				measurements = append(measurements, m2)
			}
			if trigger < 1 {
				measurements = append(measurements, m1)
			}
			triggerFound = true
			continue
		}
		measurements = append(measurements, m1)
		measurements = append(measurements, m2)
		measurements = append(measurements, m3)
	}
	// TODO: handle PreTriggerSamples.
	return measurements
}

func (c *Adc) setResetOn() {
	glog.V(1).Infof("[adc] setting reset on")
	c.setSettings(c.settings()|settingsReset, false)

	// HACK: adjust max samples, since the number should be smaller than what
	// is being returned.
	c.hwMaxSamples = c.numSamples() - 45
	c.setNumSamples(c.hwMaxSamples)
}

func (c *Adc) setResetOff() {
	glog.V(1).Infof("[adc] setting reset off")
	c.setSettings(c.settings()&(^settingsReset), true)
}

func (c *Adc) refreshParams() {
	glog.V(1).Infof("[adc] refreshing parameters")
	c.SetGainMode(c.GainMode())
	c.SetGain(c.Gain())
	c.SetTriggerMode(c.TriggerMode())
	c.SetTriggerOffset(c.TriggerOffset())
	c.SetPreTriggerSamples(c.PreTriggerSamples())
	c.SetTotalSamples(c.TotalSamples())
	c.SetDownsampleFactor(c.DownsampleFactor())
	c.SetAdcClockSource(c.AdcClockSource())
	c.SetFreqCounterSource(c.FreqCounterSource())
	c.SetClkGenInputSource(c.ClkGenInputSource())
	c.SetExtClockFreq(c.ExtClockFreq())
	c.SetClkGenOutputFreq(c.ClkGenOutputFreq())
}

func (c *Adc) defaultSetup() {
	if c.Version().HwType == HwChipWhispererLite {
		glog.V(1).Infof("[adc] default setup for CWLite")
		c.SetGain(45)
		c.SetTotalSamples(3000)
		c.SetTriggerOffset(0)
		c.SetTriggerMode(TriggerModeRisingEdge)
		c.SetClkGenOutputFreq(7370000)
		c.SetAdcClockSource(AdcSrcClkGenX4ViaDcm)
		c.SetTriggerTargetIoPin(TriggerTargetIoPin4)
		c.SetTargetIo1(TargetIoModeSerialRx)
		c.SetTargetIo2(TargetIoModeSerialTx)
		c.SetHs2(Hs2ModeClkGen)
	}
}

func NewAdc(fpga *Fpga) (*Adc, error) {
	c := &Adc{fpga, nil, 0, 10e6}

	c.setResetOn()
	c.setResetOff()
	c.refreshParams()
	c.defaultSetup()

	if c.err != nil {
		return nil, c.err
	}
	return c, nil
}
