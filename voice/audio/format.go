// Package audio provides shared PCM audio utilities for the voice pipeline:
// format parsing, a mu-law codec, linear resampling, WAV I/O, and a small
// energy-based voice activity detector.
//
// All PCM in this package is signed 16-bit little-endian mono unless noted
// otherwise. Format names follow the ElevenLabs output/input format strings,
// e.g. "pcm_16000", "ulaw_8000", "mp3_44100_128", "opus_48000_64".
package audio

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Format describes an audio encoding as named by the ElevenLabs STT/TTS
// APIs.
type Format struct {
	Name           string // e.g. "pcm_16000"
	Codec          string // "pcm" | "ulaw" | "mp3" | "opus"
	Rate           int    // sample rate in Hz (for mp3_44100_128 -> 44100)
	BytesPerSample int    // 2 for pcm, 1 for ulaw, 0 for compressed
}

// pcmRates are the sample rates ElevenLabs accepts for raw pcm_<rate>.
var pcmRates = map[int]bool{
	8000:  true,
	16000: true,
	22050: true,
	24000: true,
	44100: true,
	48000: true,
}

// ParseFormat parses an ElevenLabs format name: pcm_8000/16000/22050/24000/
// 44100/48000, ulaw_8000, mp3_<rate>_<kbps>, or opus_<rate>_<kbps>. It
// returns an error for anything else.
func ParseFormat(name string) (Format, error) {
	parts := strings.Split(name, "_")

	switch parts[0] {
	case "pcm":
		if len(parts) != 2 {
			break
		}
		rate, err := strconv.Atoi(parts[1])
		if err != nil || !pcmRates[rate] {
			break
		}
		return Format{Name: name, Codec: "pcm", Rate: rate, BytesPerSample: 2}, nil

	case "ulaw":
		if len(parts) != 2 || parts[1] != "8000" {
			break
		}
		return Format{Name: name, Codec: "ulaw", Rate: 8000, BytesPerSample: 1}, nil

	case "mp3":
		if len(parts) != 3 {
			break
		}
		rate, err := strconv.Atoi(parts[1])
		if err != nil || rate <= 0 {
			break
		}
		if _, err := strconv.Atoi(parts[2]); err != nil {
			break
		}
		return Format{Name: name, Codec: "mp3", Rate: rate, BytesPerSample: 0}, nil

	case "opus":
		if len(parts) != 3 {
			break
		}
		rate, err := strconv.Atoi(parts[1])
		if err != nil || rate <= 0 {
			break
		}
		if _, err := strconv.Atoi(parts[2]); err != nil {
			break
		}
		return Format{Name: name, Codec: "opus", Rate: rate, BytesPerSample: 0}, nil
	}

	return Format{}, fmt.Errorf("audio: invalid format %q", name)
}

// MustFormat is like ParseFormat but panics on error.
func MustFormat(name string) Format {
	f, err := ParseFormat(name)
	if err != nil {
		panic(err)
	}
	return f
}

// Duration returns how long nbytes of this format plays for. It returns 0
// for compressed formats, where byte count does not map to a fixed
// duration.
func (f Format) Duration(nbytes int) time.Duration {
	if !f.Raw() || f.Rate <= 0 || f.BytesPerSample <= 0 {
		return 0
	}
	samples := nbytes / f.BytesPerSample
	return time.Duration(samples) * time.Second / time.Duration(f.Rate)
}

// Bytes returns the number of bytes of this format needed to hold d of
// audio. It returns 0 for compressed formats. The result is always a
// multiple of BytesPerSample.
func (f Format) Bytes(d time.Duration) int {
	if !f.Raw() || f.Rate <= 0 {
		return 0
	}
	samples := int(d * time.Duration(f.Rate) / time.Second)
	return samples * f.BytesPerSample
}

// Raw reports whether f is an uncompressed format (pcm or ulaw).
func (f Format) Raw() bool {
	return f.Codec == "pcm" || f.Codec == "ulaw"
}

// Silence returns d worth of silence in this format: zero bytes for pcm
// (sample 0), or 0xFF bytes for ulaw (which decodes to sample 0). It
// returns nil for compressed formats.
func (f Format) Silence(d time.Duration) []byte {
	n := f.Bytes(d)
	if n == 0 {
		return nil
	}
	buf := make([]byte, n)
	if f.Codec == "ulaw" {
		for i := range buf {
			buf[i] = 0xFF
		}
	}
	return buf
}
