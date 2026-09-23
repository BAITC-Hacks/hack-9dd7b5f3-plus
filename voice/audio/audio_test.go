package audio

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// sineInt16 generates n samples of a sine wave at freq Hz, sampled at rate
// Hz, with the given peak amplitude.
func sineInt16(freq float64, rate, n int, amp float64) []int16 {
	s := make([]int16, n)
	for i := 0; i < n; i++ {
		v := amp * math.Sin(2*math.Pi*freq*float64(i)/float64(rate))
		s[i] = int16(v)
	}
	return s
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func equalInt16(a, b []int16) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// -- format.go ---------------------------------------------------------

func TestParseFormat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		want    Format
	}{
		{"pcm 8000", "pcm_8000", false, Format{"pcm_8000", "pcm", 8000, 2}},
		{"pcm 16000", "pcm_16000", false, Format{"pcm_16000", "pcm", 16000, 2}},
		{"pcm 22050", "pcm_22050", false, Format{"pcm_22050", "pcm", 22050, 2}},
		{"pcm 24000", "pcm_24000", false, Format{"pcm_24000", "pcm", 24000, 2}},
		{"pcm 44100", "pcm_44100", false, Format{"pcm_44100", "pcm", 44100, 2}},
		{"pcm 48000", "pcm_48000", false, Format{"pcm_48000", "pcm", 48000, 2}},
		{"ulaw 8000", "ulaw_8000", false, Format{"ulaw_8000", "ulaw", 8000, 1}},
		{"mp3", "mp3_44100_128", false, Format{"mp3_44100_128", "mp3", 44100, 0}},
		{"opus", "opus_48000_64", false, Format{"opus_48000_64", "opus", 48000, 0}},
		{"pcm bad rate", "pcm_11025", true, Format{}},
		{"ulaw bad rate", "ulaw_16000", true, Format{}},
		{"unknown codec", "flac_44100", true, Format{}},
		{"empty", "", true, Format{}},
		{"pcm no rate", "pcm", true, Format{}},
		{"mp3 no bitrate", "mp3_44100", true, Format{}},
		{"garbage", "not_a_format_at_all", true, Format{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFormat(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseFormat(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseFormat(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMustFormat(t *testing.T) {
	if f := MustFormat("pcm_16000"); f.Rate != 16000 {
		t.Errorf("MustFormat(pcm_16000).Rate = %d, want 16000", f.Rate)
	}

	defer func() {
		if recover() == nil {
			t.Error("MustFormat did not panic on an invalid format")
		}
	}()
	MustFormat("bogus")
}

func TestFormatDurationBytes(t *testing.T) {
	pcm16k := MustFormat("pcm_16000")
	ulaw8k := MustFormat("ulaw_8000")
	mp3 := MustFormat("mp3_44100_128")

	if got := pcm16k.Duration(32000); got != time.Second {
		t.Errorf("pcm_16000.Duration(32000) = %v, want 1s", got)
	}
	if got := pcm16k.Bytes(time.Second); got != 32000 {
		t.Errorf("pcm_16000.Bytes(1s) = %d, want 32000", got)
	}
	if got := ulaw8k.Duration(8000); got != time.Second {
		t.Errorf("ulaw_8000.Duration(8000) = %v, want 1s", got)
	}
	if got := ulaw8k.Bytes(time.Second); got != 8000 {
		t.Errorf("ulaw_8000.Bytes(1s) = %d, want 8000", got)
	}
	if got := mp3.Duration(1000); got != 0 {
		t.Errorf("mp3.Duration = %v, want 0", got)
	}
	if got := mp3.Bytes(time.Second); got != 0 {
		t.Errorf("mp3.Bytes = %d, want 0", got)
	}

	durations := []time.Duration{
		time.Millisecond, 33 * time.Millisecond, 300 * time.Millisecond, 1234 * time.Millisecond,
	}
	for _, d := range durations {
		if n := pcm16k.Bytes(d); n%pcm16k.BytesPerSample != 0 {
			t.Errorf("pcm_16000.Bytes(%v) = %d, not a multiple of %d", d, n, pcm16k.BytesPerSample)
		}
		if n := ulaw8k.Bytes(d); n%ulaw8k.BytesPerSample != 0 {
			t.Errorf("ulaw_8000.Bytes(%v) = %d, not a multiple of %d", d, n, ulaw8k.BytesPerSample)
		}
	}
}

func TestFormatRawAndSilence(t *testing.T) {
	pcm := MustFormat("pcm_8000")
	ulaw := MustFormat("ulaw_8000")
	mp3 := MustFormat("mp3_44100_128")
	opus := MustFormat("opus_48000_64")

	if !pcm.Raw() || !ulaw.Raw() {
		t.Error("pcm and ulaw should report Raw() == true")
	}
	if mp3.Raw() || opus.Raw() {
		t.Error("mp3 and opus should report Raw() == false")
	}

	s := pcm.Silence(10 * time.Millisecond)
	if want := pcm.Bytes(10 * time.Millisecond); len(s) != want {
		t.Fatalf("pcm silence length = %d, want %d", len(s), want)
	}
	for _, b := range s {
		if b != 0 {
			t.Fatalf("pcm silence byte = %#x, want 0x00", b)
		}
	}

	su := ulaw.Silence(10 * time.Millisecond)
	if want := ulaw.Bytes(10 * time.Millisecond); len(su) != want {
		t.Fatalf("ulaw silence length = %d, want %d", len(su), want)
	}
	for _, b := range su {
		if b != 0xFF {
			t.Fatalf("ulaw silence byte = %#x, want 0xFF", b)
		}
	}

	if s := mp3.Silence(time.Second); s != nil {
		t.Errorf("mp3.Silence = %v, want nil", s)
	}
}

// -- pcm.go --------------------------------------------------------------

func TestBytesSamplesRoundTrip(t *testing.T) {
	orig := []int16{0, 1, -1, 32767, -32768, 12345, -12345}
	b := SamplesToBytes(orig)
	if len(b) != len(orig)*2 {
		t.Fatalf("len(b) = %d, want %d", len(b), len(orig)*2)
	}
	if got := BytesToSamples(b); !equalInt16(got, orig) {
		t.Errorf("BytesToSamples(SamplesToBytes(orig)) = %v, want %v", got, orig)
	}
}

func TestRMS(t *testing.T) {
	if got := RMS(nil); got != 0 {
		t.Errorf("RMS(nil) = %v, want 0", got)
	}
	if got := RMS([]int16{0, 0, 0}); got != 0 {
		t.Errorf("RMS(zeros) = %v, want 0", got)
	}
	if got := RMS([]int16{100, -100, 100, -100}); math.Abs(got-100) > 0.001 {
		t.Errorf("RMS(constant amplitude 100) = %v, want 100", got)
	}
}

func TestToPCM16(t *testing.T) {
	pcmFmt := MustFormat("pcm_16000")
	ulawFmt := MustFormat("ulaw_8000")
	mp3Fmt := MustFormat("mp3_44100_128")

	samples := []int16{100, -100, 5000, -5000}
	if got := ToPCM16(SamplesToBytes(samples), pcmFmt); !equalInt16(got, samples) {
		t.Errorf("ToPCM16(pcm) = %v, want %v", got, samples)
	}

	ulawBytes := SamplesToMulaw(samples)
	want := MulawToSamples(ulawBytes)
	if got := ToPCM16(ulawBytes, ulawFmt); !equalInt16(got, want) {
		t.Errorf("ToPCM16(ulaw) = %v, want %v", got, want)
	}

	if got := ToPCM16([]byte{1, 2, 3}, mp3Fmt); got != nil {
		t.Errorf("ToPCM16(mp3) = %v, want nil", got)
	}
}

func TestConvertSameFormat(t *testing.T) {
	pcmFmt := MustFormat("pcm_16000")
	in := SamplesToBytes([]int16{1, 2, 3, 4})
	got, err := Convert(in, pcmFmt, pcmFmt)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !bytes.Equal(got, in) {
		t.Errorf("Convert(same format) changed bytes: got %v, want %v", got, in)
	}
}

func TestConvertCompressedError(t *testing.T) {
	pcmFmt := MustFormat("pcm_16000")
	mp3Fmt := MustFormat("mp3_44100_128")
	if _, err := Convert([]byte{1, 2, 3}, pcmFmt, mp3Fmt); err == nil {
		t.Error("Convert to a compressed format should error")
	}
	if _, err := Convert([]byte{1, 2, 3}, mp3Fmt, pcmFmt); err == nil {
		t.Error("Convert from a compressed format should error")
	}
}

func TestConvertPCMToUlawRoundTrip(t *testing.T) {
	pcm16k := MustFormat("pcm_16000")
	ulaw8k := MustFormat("ulaw_8000")

	const n = 16000 // 1s @ 16kHz
	in := sineInt16(300, 16000, n, 8000)
	pcmBytes := SamplesToBytes(in)

	mid, err := Convert(pcmBytes, pcm16k, ulaw8k)
	if err != nil {
		t.Fatalf("Convert pcm->ulaw: %v", err)
	}
	if len(mid) != n/2 {
		t.Fatalf("len(ulaw) = %d, want %d", len(mid), n/2)
	}

	back, err := Convert(mid, ulaw8k, pcm16k)
	if err != nil {
		t.Fatalf("Convert ulaw->pcm: %v", err)
	}
	if len(back) != len(pcmBytes) {
		t.Fatalf("round-trip len = %d, want %d", len(back), len(pcmBytes))
	}

	origRMS := RMS(in)
	gotRMS := RMS(BytesToSamples(back))
	if rel := math.Abs(gotRMS-origRMS) / origRMS; rel > 0.25 {
		t.Fatalf("round-trip RMS drifted too much: orig=%.1f got=%.1f rel=%.4f", origRMS, gotRMS, rel)
	}
}

func TestFrames(t *testing.T) {
	f := MustFormat("pcm_16000")
	b := make([]byte, f.Bytes(250*time.Millisecond)) // 250ms -> 100,100,50

	frames := Frames(b, f, 100*time.Millisecond)
	if len(frames) != 3 {
		t.Fatalf("len(frames) = %d, want 3", len(frames))
	}

	want := []int{f.Bytes(100 * time.Millisecond), f.Bytes(100 * time.Millisecond), f.Bytes(50 * time.Millisecond)}
	total := 0
	for i, fr := range frames {
		if len(fr) != want[i] {
			t.Errorf("frame %d len = %d, want %d", i, len(fr), want[i])
		}
		total += len(fr)
	}
	if total != len(b) {
		t.Errorf("total framed bytes = %d, want %d", total, len(b))
	}

	if got := Frames(nil, f, 100*time.Millisecond); got != nil {
		t.Errorf("Frames(nil) = %v, want nil", got)
	}
}

// -- mulaw.go --------------------------------------------------------------

func TestMulawKnownValues(t *testing.T) {
	if got := MulawEncode(0); got != 0xFF {
		t.Errorf("MulawEncode(0) = %#02x, want 0xFF", got)
	}
	if got := MulawDecode(0xFF); got < -2 || got > 2 {
		t.Errorf("MulawDecode(0xFF) = %d, want ~0", got)
	}
}

func TestMulawRoundTripFullRange(t *testing.T) {
	// mu-law's first segment has a fixed quantization step of 8 (absolute
	// error <= 4), so relative error is only a meaningful measure once the
	// magnitude is well clear of that step; below it we just bound the
	// absolute error (and check 0 maps to ~0).
	const nearZeroMag = 128
	const nearZeroAbsErr = 8
	const relErrLimit = 0.10

	for v := -32768; v <= 32767; v++ {
		s := int16(v)
		got := int(MulawDecode(MulawEncode(s)))

		mag := absInt(v)
		if mag < nearZeroMag {
			if absInt(got-v) > nearZeroAbsErr {
				t.Fatalf("near-zero error too large: sample=%d decoded=%d", v, got)
			}
			continue
		}

		if (v > 0) != (got > 0) {
			t.Fatalf("sign not preserved: sample=%d decoded=%d", v, got)
		}
		if rel := math.Abs(float64(got-v)) / float64(mag); rel > relErrLimit {
			t.Fatalf("relative error too large: sample=%d decoded=%d rel=%.4f", v, got, rel)
		}
	}
}

func TestMulawToPCMAndBack(t *testing.T) {
	samples := []int16{0, 100, -100, 1000, -1000, 30000, -30000}
	pcmBytes := SamplesToBytes(samples)

	ulawBytes := PCMToMulaw(pcmBytes)
	if len(ulawBytes) != len(samples) {
		t.Fatalf("len(PCMToMulaw) = %d, want %d", len(ulawBytes), len(samples))
	}

	backPCM := MulawToPCM(ulawBytes)
	if len(backPCM) != len(pcmBytes) {
		t.Fatalf("len(MulawToPCM) = %d, want %d", len(backPCM), len(pcmBytes))
	}
}

// -- resample.go -----------------------------------------------------------

func TestResampleDownsampleHalvesLengthAndKeepsRMS(t *testing.T) {
	const rate = 16000
	const n = rate // 1s
	in := sineInt16(300, rate, n, 8000)

	out := Resample(in, rate, rate/2)
	if len(out) != n/2 {
		t.Fatalf("len(out) = %d, want %d", len(out), n/2)
	}

	wantRMS := RMS(in)
	gotRMS := RMS(out)
	if rel := math.Abs(gotRMS-wantRMS) / wantRMS; rel > 0.15 {
		t.Fatalf("RMS drifted too much: in=%.1f out=%.1f rel=%.4f", wantRMS, gotRMS, rel)
	}
}

func TestResampleUpsampleDoublesLength(t *testing.T) {
	const rate = 8000
	const n = rate / 2 // 0.5s
	in := sineInt16(300, rate, n, 8000)

	out := Resample(in, rate, rate*2)
	if len(out) != n*2 {
		t.Fatalf("len(out) = %d, want %d", len(out), n*2)
	}
}

func TestResampleSameRate(t *testing.T) {
	in := []int16{1, 2, 3, 4, 5}
	out := Resample(in, 16000, 16000)
	if !equalInt16(out, in) {
		t.Errorf("Resample(same rate) = %v, want %v", out, in)
	}
}

func TestResampleEmpty(t *testing.T) {
	if got := Resample(nil, 16000, 8000); got != nil {
		t.Errorf("Resample(nil) = %v, want nil", got)
	}
}

// -- wav.go ------------------------------------------------------------

func TestWAVEncodeDecodeRoundTrip(t *testing.T) {
	const rate = 16000
	samples := sineInt16(300, rate, rate/10, 5000) // 100ms
	pcm := SamplesToBytes(samples)

	wav := EncodeWAV(pcm, rate)
	if len(wav) != 44+len(pcm) {
		t.Fatalf("len(wav) = %d, want %d", len(wav), 44+len(pcm))
	}
	if string(wav[0:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		t.Fatalf("missing RIFF/WAVE header: %v", wav[0:12])
	}

	gotPCM, gotRate, err := DecodeWAV(wav)
	if err != nil {
		t.Fatalf("DecodeWAV: %v", err)
	}
	if gotRate != rate {
		t.Errorf("rate = %d, want %d", gotRate, rate)
	}
	if !bytes.Equal(gotPCM, pcm) {
		t.Errorf("decoded pcm does not match original")
	}
}

func TestDecodeWAVErrors(t *testing.T) {
	if _, _, err := DecodeWAV([]byte("not a wav file")); err == nil {
		t.Error("DecodeWAV(garbage) should error")
	}
	if _, _, err := DecodeWAV(nil); err == nil {
		t.Error("DecodeWAV(nil) should error")
	}
}

// buildWAV hand-builds a RIFF/WAVE file with the given format code, channel
// count, sample rate and bits-per-sample, inserting an odd-sized LIST
// chunk before "data" to exercise chunk skipping and padding.
func buildWAV(t *testing.T, formatCode, channels, rate, bits int, data []byte) []byte {
	t.Helper()

	blockAlign := channels * bits / 8
	byteRate := rate * blockAlign

	var buf bytes.Buffer
	buf.WriteString("RIFF")
	writeU32(&buf, 0) // patched below
	buf.WriteString("WAVE")

	buf.WriteString("fmt ")
	writeU32(&buf, 16)
	writeU16(&buf, uint16(formatCode))
	writeU16(&buf, uint16(channels))
	writeU32(&buf, uint32(rate))
	writeU32(&buf, uint32(byteRate))
	writeU16(&buf, uint16(blockAlign))
	writeU16(&buf, uint16(bits))

	// Odd-sized LIST chunk: exercises both chunk skipping and padding.
	buf.WriteString("LIST")
	writeU32(&buf, 5)
	buf.WriteString("INFOx")
	buf.WriteByte(0)

	buf.WriteString("data")
	writeU32(&buf, uint32(len(data)))
	buf.Write(data)
	if len(data)%2 == 1 {
		buf.WriteByte(0)
	}

	out := buf.Bytes()
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))
	return out
}

func writeU32(buf *bytes.Buffer, v uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	buf.Write(b[:])
}

func writeU16(buf *bytes.Buffer, v uint16) {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], v)
	buf.Write(b[:])
}

func TestDecodeWAVStereoDownmix(t *testing.T) {
	const rate = 8000
	const frames = 100
	left, right := int16(1000), int16(-1000)

	var data []byte
	for i := 0; i < frames; i++ {
		var frame [4]byte
		binary.LittleEndian.PutUint16(frame[0:2], uint16(left))
		binary.LittleEndian.PutUint16(frame[2:4], uint16(right))
		data = append(data, frame[:]...)
	}

	wav := buildWAV(t, 1, 2, rate, 16, data)

	pcm, gotRate, err := DecodeWAV(wav)
	if err != nil {
		t.Fatalf("DecodeWAV: %v", err)
	}
	if gotRate != rate {
		t.Errorf("rate = %d, want %d", gotRate, rate)
	}

	samples := BytesToSamples(pcm)
	if len(samples) != frames {
		t.Fatalf("len(samples) = %d, want %d", len(samples), frames)
	}
	for i, s := range samples {
		if s != 0 { // average of +1000 and -1000
			t.Fatalf("sample %d = %d, want 0", i, s)
		}
	}
}

func TestDecodeWAVMulaw(t *testing.T) {
	const rate = 8000
	raw := []byte{0xFF, 0x80, 0x00, 0xE0, 0x7F}
	wav := buildWAV(t, 7, 1, rate, 8, raw)

	pcm, gotRate, err := DecodeWAV(wav)
	if err != nil {
		t.Fatalf("DecodeWAV: %v", err)
	}
	if gotRate != rate {
		t.Errorf("rate = %d, want %d", gotRate, rate)
	}

	want := SamplesToBytes(MulawToSamples(raw))
	if !bytes.Equal(pcm, want) {
		t.Errorf("decoded mu-law pcm mismatch: got %v, want %v", pcm, want)
	}
}

func TestDecodeWAVExtensiblePCM(t *testing.T) {
	const rate = 16000
	const channels = 1
	const bits = 16

	samples := []int16{1, -1, 100, -100, 0}
	data := SamplesToBytes(samples)

	blockAlign := channels * bits / 8
	byteRate := rate * blockAlign

	var buf bytes.Buffer
	buf.WriteString("RIFF")
	writeU32(&buf, 0)
	buf.WriteString("WAVE")

	buf.WriteString("fmt ")
	writeU32(&buf, 40) // extensible fmt chunk size
	writeU16(&buf, 0xFFFE)
	writeU16(&buf, channels)
	writeU32(&buf, uint32(rate))
	writeU32(&buf, uint32(byteRate))
	writeU16(&buf, uint16(blockAlign))
	writeU16(&buf, bits)
	writeU16(&buf, 22) // cbSize
	writeU16(&buf, bits)
	writeU32(&buf, 0) // channel mask
	// SubFormat GUID: first 2 bytes are the real format code (1 = PCM),
	// followed by the fixed KSDATAFORMAT_SUBTYPE_PCM tail bytes.
	writeU16(&buf, 1)
	buf.Write([]byte{0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x80, 0x00, 0x00, 0xAA, 0x00, 0x38, 0x9B, 0x71})

	buf.WriteString("data")
	writeU32(&buf, uint32(len(data)))
	buf.Write(data)

	out := buf.Bytes()
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))

	pcm, gotRate, err := DecodeWAV(out)
	if err != nil {
		t.Fatalf("DecodeWAV: %v", err)
	}
	if gotRate != rate {
		t.Errorf("rate = %d, want %d", gotRate, rate)
	}
	if !bytes.Equal(pcm, data) {
		t.Errorf("decoded extensible pcm mismatch: got %v, want %v", pcm, data)
	}
}

func TestWAVWriter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "out.wav")

	w, err := CreateWAV(path, 16000)
	if err != nil {
		t.Fatalf("CreateWAV: %v", err)
	}

	samples := sineInt16(300, 16000, 1600, 5000) // 100ms
	pcm := SamplesToBytes(samples)
	half := len(pcm) / 2

	if _, err := w.Write(pcm[:half]); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := w.Write(pcm[half:]); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	gotPCM, gotRate, err := DecodeWAV(raw)
	if err != nil {
		t.Fatalf("DecodeWAV: %v", err)
	}
	if gotRate != 16000 {
		t.Errorf("rate = %d, want 16000", gotRate)
	}
	if !bytes.Equal(gotPCM, pcm) {
		t.Errorf("decoded pcm does not match written pcm")
	}
}

// -- vad.go --------------------------------------------------------------

func TestEnergyVADTriggersOnce(t *testing.T) {
	const rate = 16000
	const frame = 320 // 20ms @ 16kHz

	var v EnergyVAD
	triggers := 0

	// 500ms of low-level noise: must never trigger.
	rng := rand.New(rand.NewSource(1))
	noiseFrames := int(500*time.Millisecond) / (frame * int(time.Second) / rate)
	for i := 0; i < noiseFrames; i++ {
		s := make([]int16, frame)
		for j := range s {
			s[j] = int16(rng.Intn(41) - 20) // small noise, amplitude ~20
		}
		if v.Push(s, rate) {
			triggers++
		}
	}
	if v.Active() {
		t.Fatalf("VAD active after noise only")
	}

	// 1s of loud 300Hz sine: must trigger exactly once.
	loud := sineInt16(300, rate, rate, 8000)
	for off := 0; off < len(loud); off += frame {
		end := off + frame
		if end > len(loud) {
			end = len(loud)
		}
		if v.Push(loud[off:end], rate) {
			triggers++
		}
	}

	if triggers != 1 {
		t.Fatalf("triggers = %d, want exactly 1", triggers)
	}
	if !v.Active() {
		t.Fatalf("VAD should be Active() after sustained loud speech")
	}
}

func TestEnergyVADReset(t *testing.T) {
	const rate = 16000
	const frame = 320

	var v EnergyVAD
	loud := sineInt16(300, rate, rate, 8000)
	for off := 0; off < len(loud); off += frame {
		end := off + frame
		if end > len(loud) {
			end = len(loud)
		}
		v.Push(loud[off:end], rate)
	}
	if !v.Active() {
		t.Fatalf("expected VAD to be active before Reset")
	}

	v.Reset()
	if v.Active() {
		t.Errorf("VAD still Active() after Reset")
	}

	retriggered := false
	for off := 0; off < len(loud); off += frame {
		end := off + frame
		if end > len(loud) {
			end = len(loud)
		}
		if v.Push(loud[off:end], rate) {
			retriggered = true
			break
		}
	}
	if !retriggered {
		t.Errorf("VAD did not retrigger after Reset")
	}
}
