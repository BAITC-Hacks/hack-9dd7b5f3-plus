package audio

// Standard G.711 mu-law codec constants (ITU-T reference implementation).
const (
	mulawBias = 0x84
	mulawClip = 32635
)

// mulawSegEnd holds the upper bound of each of the 8 mu-law segments.
var mulawSegEnd = [8]int{0xFF, 0x1FF, 0x3FF, 0x7FF, 0xFFF, 0x1FFF, 0x3FFF, 0x7FFF}

// mulawDecodeTable maps every possible mu-law byte to its linear PCM16
// value, computed once at init time.
var mulawDecodeTable = func() [256]int16 {
	var t [256]int16
	for i := range t {
		t[i] = mulawDecodeBits(byte(i))
	}
	return t
}()

// mulawDecodeBits computes the linear PCM16 value for a mu-law byte
// directly from the G.711 formula. mulawDecodeTable precomputes this for
// all 256 byte values so MulawDecode is a single table lookup.
func mulawDecodeBits(u byte) int16 {
	comp := ^u
	mantissa := int(comp & 0x0F)
	exponent := int((comp & 0x70) >> 4)
	sign := comp & 0x80

	t := (mantissa << 3) + mulawBias
	t <<= exponent

	if sign != 0 {
		return int16(mulawBias - t)
	}
	return int16(t - mulawBias)
}

// MulawEncode encodes a linear PCM16 sample to a single G.711 mu-law byte.
func MulawEncode(s int16) byte {
	sample := int(s)

	mask := byte(0xFF)
	if sample < 0 {
		sample = -sample
		mask = 0x7F
	}
	if sample > mulawClip {
		sample = mulawClip
	}
	sample += mulawBias

	seg := len(mulawSegEnd)
	for i, end := range mulawSegEnd {
		if sample <= end {
			seg = i
			break
		}
	}
	if seg >= len(mulawSegEnd) {
		return 0x7F ^ mask
	}

	uval := byte(seg<<4) | byte((sample>>(seg+3))&0x0F)
	return uval ^ mask
}

// MulawDecode decodes a single G.711 mu-law byte to a linear PCM16 sample.
func MulawDecode(u byte) int16 {
	return mulawDecodeTable[u]
}

// MulawToSamples decodes mu-law bytes to PCM16 samples.
func MulawToSamples(b []byte) []int16 {
	s := make([]int16, len(b))
	for i, u := range b {
		s[i] = MulawDecode(u)
	}
	return s
}

// SamplesToMulaw encodes PCM16 samples to mu-law bytes.
func SamplesToMulaw(s []int16) []byte {
	b := make([]byte, len(s))
	for i, v := range s {
		b[i] = MulawEncode(v)
	}
	return b
}

// MulawToPCM decodes raw mu-law bytes to raw little-endian PCM16 bytes.
func MulawToPCM(b []byte) []byte {
	return SamplesToBytes(MulawToSamples(b))
}

// PCMToMulaw encodes raw little-endian PCM16 bytes to raw mu-law bytes.
func PCMToMulaw(b []byte) []byte {
	return SamplesToMulaw(BytesToSamples(b))
}
