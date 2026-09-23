package audio

import "time"

// Defaults used by EnergyVAD when the corresponding field is zero.
const (
	defaultMinSpeech = 200 * time.Millisecond
	defaultHangover  = 300 * time.Millisecond
	defaultRatio     = 3
	defaultMinRMS    = 600
	noiseFloorAlpha  = 0.1
)

// EnergyVAD is a tiny adaptive energy detector used for fast local
// barge-in detection. It tracks a noise floor (an exponential moving
// average updated only on frames it does not consider speech) and flags an
// onset once energy has stayed above the floor for MinSpeech.
type EnergyVAD struct {
	MinSpeech time.Duration // energy must stay above threshold this long to trigger (default 200ms)
	Hangover  time.Duration // silence that resets the detector (default 300ms)
	Ratio     float64       // speech when RMS > noise floor * Ratio (default 3)
	MinRMS    float64       // absolute minimum speech RMS (default 600)

	noiseFloor float64
	speechDur  time.Duration
	silenceDur time.Duration
	active     bool
}

// Push feeds s (nframes samples at the given sample rate) through the
// detector. It returns true exactly once per speech onset: the instant
// accumulated speech energy first reaches MinSpeech. It returns false on
// every other call, including while speech remains Active.
func (v *EnergyVAD) Push(s []int16, rate int) bool {
	if len(s) == 0 || rate <= 0 {
		return false
	}

	minSpeech := v.MinSpeech
	if minSpeech <= 0 {
		minSpeech = defaultMinSpeech
	}
	hangover := v.Hangover
	if hangover <= 0 {
		hangover = defaultHangover
	}
	ratio := v.Ratio
	if ratio <= 0 {
		ratio = defaultRatio
	}
	minRMS := v.MinRMS
	if minRMS <= 0 {
		minRMS = defaultMinRMS
	}

	rms := RMS(s)
	frameDur := time.Duration(len(s)) * time.Second / time.Duration(rate)
	isSpeech := rms > minRMS && rms > v.noiseFloor*ratio

	if isSpeech {
		v.speechDur += frameDur
		v.silenceDur = 0
	} else {
		v.noiseFloor += noiseFloorAlpha * (rms - v.noiseFloor)

		v.silenceDur += frameDur
		if v.silenceDur >= hangover {
			v.speechDur = 0
			v.active = false
		}
	}

	if !v.active && v.speechDur >= minSpeech {
		v.active = true
		return true
	}
	return false
}

// Active reports whether the detector currently considers speech ongoing.
func (v *EnergyVAD) Active() bool {
	return v.active
}

// Reset clears all detector state, including the learned noise floor.
func (v *EnergyVAD) Reset() {
	v.noiseFloor = 0
	v.speechDur = 0
	v.silenceDur = 0
	v.active = false
}
