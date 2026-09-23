// Package speech is the seam between the conversation engine and speech
// providers: small Recognizer/Synthesizer interfaces plus their ElevenLabs
// implementations tuned for the RU/KZ contact-center case.
//
// Model routing (defaults, see config):
//
//	reply in Russian -> eleven_flash_v2_5         (~75 ms model latency)
//	reply in Kazakh  -> eleven_v3_conversational  (only realtime model with Kazakh)
//	speech-to-text   -> scribe_v2_realtime        (one session per call, auto language)
package speech

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hackathon/voice/config"
	"hackathon/voice/elevenlabs"
	"hackathon/voice/lang"
)

// RecognizerOptions configures one streaming recognition session.
type RecognizerOptions struct {
	Format       string // input audio: pcm_16000 (web) | pcm_8000 (Asterisk) | ulaw_8000 (Twilio)
	Manual       bool   // push-to-talk: the client sends commits instead of server VAD
	PreviousText string
}

// RecognizerSession is one open speech-to-text stream (usually a whole call).
type RecognizerSession interface {
	Send(audio []byte) error
	Commit() error
	Events() <-chan elevenlabs.STTEvent
	SpeechEnd(words []elevenlabs.Word) (time.Time, bool)
	Close() error
}

// Recognizer opens recognition sessions.
type Recognizer interface {
	Open(ctx context.Context, o RecognizerOptions) (RecognizerSession, error)
}

// Stream speaks one reply: text goes in as it is generated, audio comes out.
type Stream interface {
	Send(text string, flush bool) error
	Finish() error
	Audio() <-chan []byte
	Err() error
	Close() error
	Info() elevenlabs.SocketInfo
}

// Synthesizer opens streaming TTS and synthesizes short fixed phrases.
type Synthesizer interface {
	Open(ctx context.Context, l lang.Lang, format string) (Stream, error)
	// Say returns the audio for a short fixed phrase (greeting, filler),
	// cached on disk so repeated phrases cost no credits and no latency.
	Say(ctx context.Context, text string, l lang.Lang, format string) ([]byte, error)
}

// ElevenSTT implements Recognizer with Scribe v2 Realtime.
type ElevenSTT struct {
	Client *elevenlabs.Client
	Cfg    config.Config
}

// Options builds the STT options used for every call.
func (e *ElevenSTT) Options(o RecognizerOptions) elevenlabs.STTOptions {
	strategy := "vad"
	if o.Manual {
		strategy = "manual"
	}
	return elevenlabs.STTOptions{
		Model:                    e.Cfg.STTModel,
		AudioFormat:              o.Format,
		Language:                 e.Cfg.STTLanguage,
		SecondaryLanguages:       e.Cfg.STTSecondary,
		CommitStrategy:           strategy,
		VADSilenceSecs:           e.Cfg.VADSilenceSecs,
		VADThreshold:             e.Cfg.VADThreshold,
		MinSpeechMS:              e.Cfg.MinSpeechMS,
		MinSilenceMS:             e.Cfg.MinSilenceMS,
		IncludeTimestamps:        true,
		IncludeLanguageDetection: true,
		Keyterms:                 e.Cfg.STTKeyterms,
		PreviousText:             o.PreviousText,
	}
}

// Open starts a realtime session.
func (e *ElevenSTT) Open(ctx context.Context, o RecognizerOptions) (RecognizerSession, error) {
	s, err := e.Client.OpenSTT(ctx, e.Options(o))
	if err != nil {
		return nil, err
	}
	return sttSession{s}, nil
}

type sttSession struct{ *elevenlabs.STTSession }

func (s sttSession) Send(b []byte) error { return s.STTSession.Send(b, false) }

// ElevenTTS implements Synthesizer with ElevenLabs WebSockets (live replies)
// and HTTP streaming (cached fixed phrases).
type ElevenTTS struct {
	Client *elevenlabs.Client
	Cfg    config.Config
}

// Model returns the TTS model used for a reply language.
func (t *ElevenTTS) Model(l lang.Lang) string {
	if l == lang.KK || l == lang.Mixed {
		return t.Cfg.TTSModelKK
	}
	return t.Cfg.TTSModelRU
}

// Voice returns the voice used for a reply language.
func (t *ElevenTTS) Voice(l lang.Lang) string {
	if (l == lang.KK || l == lang.Mixed) && t.Cfg.VoiceIDKK != "" {
		return t.Cfg.VoiceIDKK
	}
	return t.Cfg.VoiceID
}

func isV3(model string) bool { return strings.HasPrefix(model, "eleven_v3") }

// settings for non-v3 models; v3 accepts only discrete stability values, so
// it keeps the voice defaults.
func (t *ElevenTTS) settings(model string) *elevenlabs.VoiceSettings {
	if isV3(model) {
		return nil
	}
	speed := t.Cfg.TTSSpeed
	if speed <= 0 {
		speed = 1
	}
	return &elevenlabs.VoiceSettings{
		Stability:       elevenlabs.Float(0.45),
		SimilarityBoost: elevenlabs.Float(0.8),
		Style:           elevenlabs.Float(0),
		UseSpeakerBoost: elevenlabs.Bool(false), // speaker boost adds latency
		Speed:           elevenlabs.Float(speed),
	}
}

// languageCode enforces Russian on Flash; Flash has no Kazakh and v3 detects
// the language from the text.
func languageCode(model string, l lang.Lang) string {
	if !isV3(model) && l == lang.RU {
		return "ru"
	}
	return ""
}

// Open connects a streaming TTS socket for one reply.
func (t *ElevenTTS) Open(ctx context.Context, l lang.Lang, format string) (Stream, error) {
	model := t.Model(l)
	return t.Client.OpenSocket(ctx, elevenlabs.SocketOptions{
		VoiceID:      t.Voice(l),
		Model:        model,
		OutputFormat: format,
		Language:     languageCode(model, l),
		AutoMode:     true,
		Settings:     t.settings(model),
	})
}

// Say synthesizes a fixed phrase through HTTP and caches the audio on disk.
func (t *ElevenTTS) Say(ctx context.Context, text string, l lang.Lang, format string) ([]byte, error) {
	model, voice := t.Model(l), t.Voice(l)
	path := t.cachePath(model, voice, format, text)
	if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
		return b, nil
	}
	b, _, err := t.Client.TTS(ctx, elevenlabs.TTSRequest{
		VoiceID:      voice,
		Model:        model,
		Text:         text,
		Language:     languageCode(model, l),
		OutputFormat: format,
		Settings:     t.settings(model),
	})
	if err != nil {
		return nil, err
	}
	if t.Cfg.CacheDir != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
			_ = os.WriteFile(path, b, 0o644)
		}
	}
	return b, nil
}

func (t *ElevenTTS) cachePath(model, voice, format, text string) string {
	h := sha1.Sum([]byte(model + "|" + voice + "|" + format + "|" + text))
	return filepath.Join(t.Cfg.CacheDir, "tts", format, hex.EncodeToString(h[:10])+".raw")
}
