package audio

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

// BytesToSamples reinterprets b as little-endian int16 PCM samples. A
// trailing odd byte, if any, is ignored.
func BytesToSamples(b []byte) []int16 {
	n := len(b) / 2
	s := make([]int16, n)
	for i := 0; i < n; i++ {
		s[i] = int16(binary.LittleEndian.Uint16(b[2*i:]))
	}
	return s
}

// SamplesToBytes encodes s as little-endian int16 PCM bytes.
func SamplesToBytes(s []int16) []byte {
	b := make([]byte, len(s)*2)
	for i, v := range s {
		binary.LittleEndian.PutUint16(b[2*i:], uint16(v))
	}
	return b
}

// RMS returns the root-mean-square amplitude of s.
func RMS(s []int16) float64 {
	if len(s) == 0 {
		return 0
	}
	var sum float64
	for _, v := range s {
		f := float64(v)
		sum += f * f
	}
	return math.Sqrt(sum / float64(len(s)))
}

// ToPCM16 decodes raw audio b in format f to PCM16 samples. It returns nil
// for compressed formats.
func ToPCM16(b []byte, f Format) []int16 {
	switch f.Codec {
	case "pcm":
		return BytesToSamples(b)
	case "ulaw":
		return MulawToSamples(b)
	default:
		return nil
	}
}

// Convert re-encodes raw audio b from one format to another, resampling
// when the rates differ. from and to must both be raw (pcm or ulaw); use of
// a compressed format returns an error. If from and to are identical, b is
// returned unchanged.
func Convert(b []byte, from, to Format) ([]byte, error) {
	if from == to {
		return b, nil
	}
	if !from.Raw() || !to.Raw() {
		return nil, fmt.Errorf("audio: cannot convert compressed format %s -> %s", from.Name, to.Name)
	}

	samples := ToPCM16(b, from)
	if from.Rate != to.Rate {
		samples = Resample(samples, from.Rate, to.Rate)
	}

	switch to.Codec {
	case "pcm":
		return SamplesToBytes(samples), nil
	case "ulaw":
		return SamplesToMulaw(samples), nil
	default:
		return nil, fmt.Errorf("audio: unsupported target format %s", to.Name)
	}
}

// Frames splits raw audio b (in format f) into consecutive frames of
// duration d. The last frame may be shorter than d. Returned frames are
// sub-slices of b.
func Frames(b []byte, f Format, d time.Duration) [][]byte {
	if len(b) == 0 {
		return nil
	}

	size := f.Bytes(d)
	if size <= 0 {
		return [][]byte{b}
	}

	var frames [][]byte
	for off := 0; off < len(b); off += size {
		end := off + size
		if end > len(b) {
			end = len(b)
		}
		frames = append(frames, b[off:end])
	}
	return frames
}
