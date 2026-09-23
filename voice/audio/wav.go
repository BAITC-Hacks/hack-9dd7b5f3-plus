package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const wavHeaderSize = 44

// EncodeWAV wraps pcm (signed 16-bit little-endian mono samples) in a
// minimal 44-byte canonical WAV header.
func EncodeWAV(pcm []byte, rate int) []byte {
	const (
		numChannels   = 1
		bitsPerSample = 16
	)
	blockAlign := numChannels * bitsPerSample / 8
	byteRate := rate * blockAlign

	buf := make([]byte, wavHeaderSize+len(pcm))
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(36+len(pcm)))
	copy(buf[8:12], "WAVE")

	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(buf[22:24], numChannels)
	binary.LittleEndian.PutUint32(buf[24:28], uint32(rate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(buf[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(buf[34:36], bitsPerSample)

	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(len(pcm)))
	copy(buf[44:], pcm)
	return buf
}

// DecodeWAV parses a RIFF/WAVE file and returns its audio as PCM16 mono
// bytes plus its sample rate. It supports 16-bit PCM (mono or stereo,
// stereo downmixed to mono), WAVE_FORMAT_EXTENSIBLE with a PCM subformat,
// and 8-bit G.711 mu-law (decoded to PCM16). Chunks other than "fmt " and
// "data" (e.g. "LIST") are skipped, honouring RIFF's odd-size padding rule.
func DecodeWAV(b []byte) (pcm []byte, rate int, err error) {
	if len(b) < 12 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return nil, 0, fmt.Errorf("audio: not a RIFF/WAVE file")
	}

	var (
		haveFmt     bool
		haveData    bool
		audioFormat uint16
		numChannels uint16
		sampleRate  uint32
		bits        uint16
		data        []byte
	)

	off := 12
	for off+8 <= len(b) {
		id := string(b[off : off+4])
		size := int(binary.LittleEndian.Uint32(b[off+4 : off+8]))
		body := off + 8
		if size < 0 || body+size > len(b) {
			size = len(b) - body // tolerate a truncated trailing chunk
		}
		if size < 0 {
			break
		}

		switch id {
		case "fmt ":
			if size < 16 {
				return nil, 0, fmt.Errorf("audio: fmt chunk too small")
			}
			chunk := b[body : body+size]
			audioFormat = binary.LittleEndian.Uint16(chunk[0:2])
			numChannels = binary.LittleEndian.Uint16(chunk[2:4])
			sampleRate = binary.LittleEndian.Uint32(chunk[4:8])
			bits = binary.LittleEndian.Uint16(chunk[14:16])
			if audioFormat == 0xFFFE { // WAVE_FORMAT_EXTENSIBLE
				if size < 40 {
					return nil, 0, fmt.Errorf("audio: extensible fmt chunk too small")
				}
				// The real format code is the first two bytes of the
				// SubFormat GUID, which starts at offset 24.
				audioFormat = binary.LittleEndian.Uint16(chunk[24:26])
			}
			haveFmt = true

		case "data":
			data = b[body : body+size]
			haveData = true
		}

		adv := size
		if adv%2 == 1 {
			adv++ // chunks are padded to an even number of bytes
		}
		off = body + adv
	}

	if !haveFmt || !haveData {
		return nil, 0, fmt.Errorf("audio: missing fmt or data chunk")
	}
	if numChannels == 0 {
		return nil, 0, fmt.Errorf("audio: invalid channel count")
	}

	switch audioFormat {
	case 1: // PCM
		if bits != 16 {
			return nil, 0, fmt.Errorf("audio: unsupported PCM bit depth %d", bits)
		}
		samples := downmixSamples(BytesToSamples(data), int(numChannels))
		pcm = SamplesToBytes(samples)

	case 7: // G.711 mu-law
		if bits != 8 {
			return nil, 0, fmt.Errorf("audio: unsupported mu-law bit depth %d", bits)
		}
		samples := downmixSamples(MulawToSamples(data), int(numChannels))
		pcm = SamplesToBytes(samples)

	default:
		return nil, 0, fmt.Errorf("audio: unsupported WAV format code %d", audioFormat)
	}

	return pcm, int(sampleRate), nil
}

// downmixSamples averages interleaved multi-channel samples down to mono.
// Mono input is returned unchanged.
func downmixSamples(samples []int16, channels int) []int16 {
	if channels <= 1 {
		return samples
	}
	frames := len(samples) / channels
	out := make([]int16, frames)
	for i := 0; i < frames; i++ {
		var sum int
		for c := 0; c < channels; c++ {
			sum += int(samples[i*channels+c])
		}
		out[i] = int16(sum / channels)
	}
	return out
}

// WAVWriter streams PCM16 mono samples to a WAV file, patching the RIFF and
// data chunk sizes when Close is called.
type WAVWriter struct {
	f         *os.File
	dataBytes int
}

// CreateWAV creates path (and any missing parent directories) and writes a
// placeholder 44-byte WAV header for PCM16 mono audio at the given sample
// rate. Call Write to stream samples and Close to finalize the header.
func CreateWAV(path string, rate int) (*WAVWriter, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	if _, err := f.Write(EncodeWAV(nil, rate)); err != nil {
		f.Close()
		return nil, err
	}
	return &WAVWriter{f: f}, nil
}

// Write appends pcm (signed 16-bit little-endian mono samples) to the file.
func (w *WAVWriter) Write(pcm []byte) (int, error) {
	n, err := w.f.Write(pcm)
	w.dataBytes += n
	return n, err
}

// Close patches the RIFF and data chunk sizes to reflect the bytes written,
// then closes the file.
func (w *WAVWriter) Close() error {
	defer w.f.Close()

	var sizeBuf [4]byte

	if _, err := w.f.Seek(4, io.SeekStart); err != nil {
		return err
	}
	binary.LittleEndian.PutUint32(sizeBuf[:], uint32(36+w.dataBytes))
	if _, err := w.f.Write(sizeBuf[:]); err != nil {
		return err
	}

	if _, err := w.f.Seek(40, io.SeekStart); err != nil {
		return err
	}
	binary.LittleEndian.PutUint32(sizeBuf[:], uint32(w.dataBytes))
	if _, err := w.f.Write(sizeBuf[:]); err != nil {
		return err
	}

	return nil
}
