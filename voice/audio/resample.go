package audio

import "math"

// Resample converts s from sample rate "from" Hz to sample rate "to" Hz
// using linear interpolation. When downsampling, a moving-average (box)
// low-pass filter is applied first to reduce aliasing; its window width is
// ceil(from/to).
func Resample(s []int16, from, to int) []int16 {
	if from <= 0 || to <= 0 || len(s) == 0 {
		return nil
	}
	if from == to {
		out := make([]int16, len(s))
		copy(out, s)
		return out
	}

	in := s
	if to < from {
		width := (from + to - 1) / to // ceil(from/to)
		in = boxFilter(s, width)
	}

	outLen := len(s) * to / from
	if outLen <= 0 {
		outLen = 1
	}
	ratio := float64(from) / float64(to)

	out := make([]int16, outLen)
	for j := range out {
		pos := float64(j) * ratio
		i0 := int(pos)
		if i0 >= len(in) {
			i0 = len(in) - 1
		}
		frac := pos - float64(i0)

		s0 := float64(in[i0])
		s1 := s0
		if i0+1 < len(in) {
			s1 = float64(in[i0+1])
		}
		out[j] = int16(math.Round(s0 + frac*(s1-s0)))
	}
	return out
}

// boxFilter applies a centered moving-average filter of the given width,
// shrinking the averaging window at the edges rather than padding with
// zeros.
func boxFilter(s []int16, width int) []int16 {
	if width <= 1 || len(s) == 0 {
		out := make([]int16, len(s))
		copy(out, s)
		return out
	}

	out := make([]int16, len(s))
	before := width / 2
	after := width - before
	for i := range s {
		var sum, count int
		for k := -before; k < after; k++ {
			j := i + k
			if j >= 0 && j < len(s) {
				sum += int(s[j])
				count++
			}
		}
		out[i] = int16(sum / count)
	}
	return out
}
