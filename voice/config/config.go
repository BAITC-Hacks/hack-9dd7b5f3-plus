// Package config provides typed settings for the voice service (ElevenLabs
// STT/TTS, OpenRouter LLM, web/phone transports).
//
// Values come from the process environment first, then from an optional
// .env file (see Load and LoadFrom). Several keys accept aliases and are
// normalized (trimmed, upper-cased, '-' replaced with '_') so that a messy
// hand-written .env still resolves correctly.
package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds every setting the voice service reads from its environment.
type Config struct {
	ElevenLabsKey  string  // ELEVENLABS_API_KEY | ELEVEN_LABS_API_KEY | ELEVEN_LABS | ELEVENLABS | XI_API_KEY
	ElevenLabsBase string  // ELEVENLABS_BASE_URL, default "https://api.elevenlabs.io"
	VoiceID        string  // ELEVENLABS_VOICE_ID, default "EXAVITQu4vr4xnSDxMaL" (premade "Sarah")
	VoiceIDKK      string  // ELEVENLABS_VOICE_ID_KK, default = VoiceID
	TTSModelRU     string  // ELEVENLABS_TTS_MODEL_RU, default "eleven_flash_v2_5"
	TTSModelKK     string  // ELEVENLABS_TTS_MODEL_KK, default "eleven_v3_conversational"
	TTSSpeed       float64 // ELEVENLABS_TTS_SPEED, default 1.0
	STTModel       string  // ELEVENLABS_STT_MODEL, default "scribe_v2_realtime"
	STTLanguage    string  // ELEVENLABS_STT_LANGUAGE, default "kk" (auto-detect mislabels Kazakh as Turkish; "kk" still transcribes Russian and mixed speech)

	STTSecondary []string // ELEVENLABS_STT_SECONDARY_LANGUAGES, comma list, default empty
	STTKeyterms  []string // ELEVENLABS_STT_KEYTERMS, comma list, default "Saqta,ОГПО,КАСКО,ДМС,полис,сақтандыру"

	VADSilenceSecs float64 // VOICE_VAD_SILENCE_SECS, default 0.4 (split utterances are merged by the engine)
	VADThreshold   float64 // VOICE_VAD_THRESHOLD, default 0 (server default)
	MinSpeechMS    int     // VOICE_MIN_SPEECH_MS, default 0
	MinSilenceMS   int     // VOICE_MIN_SILENCE_MS, default 0

	OpenRouterKey       string   // OPENROUTER_API_KEY | OPENROUTER_API | OPENROUTER | OPENROUTER_KEY
	OpenRouterBase      string   // OPENROUTER_BASE_URL, default "https://openrouter.ai/api/v1"
	OpenRouterModel     string   // OPENROUTER_MODEL, default "google/gemini-2.5-flash-lite"
	OpenRouterFallbacks []string // OPENROUTER_FALLBACK_MODELS, default "openai/gpt-4o-mini,openai/gpt-4.1-nano" (no "thinking" models: they delay the first word)

	Brain      string // VOICE_BRAIN: auto | backend | openrouter | echo, default "auto"
	BackendURL string // BACKEND_URL, default "" (team backend, e.g. http://backend:8080)
	DataDir    string // VOICE_DATA_DIR, default: FindDataDir() from the working directory, may be ""

	HTTPAddr        string   // VOICE_HTTP_ADDR, else ":"+PORT if PORT set, else ":8090"
	AudioSocketAddr string   // VOICE_AUDIOSOCKET_ADDR, default "" (disabled), e.g. ":9092"
	PublicURL       string   // VOICE_PUBLIC_URL, default "" (https base URL used in Twilio TwiML)
	AllowedOrigins  []string // VOICE_ALLOWED_ORIGINS, comma list, default ["*"]

	LogDir         string        // VOICE_LOG_DIR, default "logs/voice"
	RecordAudio    bool          // VOICE_RECORD_AUDIO, default false
	CacheDir       string        // VOICE_CACHE_DIR, default "cache/voice"
	Speculative    bool          // VOICE_SPECULATIVE, default true
	SpeculativeTTS bool          // VOICE_SPECULATIVE_TTS, default false
	SpeculateAfter time.Duration // VOICE_SPECULATE_AFTER_MS, default 250ms
	FillerAfter    time.Duration // VOICE_FILLER_AFTER_MS, default 1500ms (0 disables)
	BargeIn        bool          // VOICE_BARGE_IN, default true
	BargeInVAD     bool          // VOICE_BARGE_IN_VAD, default false
	Greeting       bool          // VOICE_GREETING, default true
}

// defaultSTTKeyterms are insurance-domain terms worth biasing the STT model
// towards; they show up constantly in the scenario catalog.
var defaultSTTKeyterms = []string{"Saqta", "ОГПО", "КАСКО", "ДМС", "полис", "сақтандыру"}

var defaultOpenRouterFallbacks = []string{"openai/gpt-4o-mini", "openai/gpt-4.1-nano"}

var defaultAllowedOrigins = []string{"*"}

// Load builds a Config from the process environment and an optional .env
// file. The file is chosen by VOICE_ENV_FILE if set, otherwise by walking up
// from the current working directory with FindDotEnv. Process environment
// variables always take precedence over the file.
func Load() Config {
	envFile := os.Getenv("VOICE_ENV_FILE")
	if envFile == "" {
		if wd, err := os.Getwd(); err == nil {
			envFile = FindDotEnv(wd)
		}
	}
	return LoadFrom(envFile)
}

// LoadFrom builds a Config the same way Load does, but reads the given .env
// path instead of discovering one. An empty path (or one that can't be
// opened) simply means no file values are available; process environment
// variables are still applied and defaults still fill the rest.
func LoadFrom(envFile string) Config {
	fileVals := map[string]string{}
	if envFile != "" {
		if f, err := os.Open(envFile); err == nil {
			fileVals = ParseDotEnv(f)
			f.Close()
		}
	}
	r := resolver{file: fileVals, env: environ()}

	var c Config

	c.ElevenLabsKey = r.get("ELEVENLABS_API_KEY", "ELEVEN_LABS_API_KEY", "ELEVEN_LABS", "ELEVENLABS", "XI_API_KEY")
	c.ElevenLabsBase = orDefault(r.get("ELEVENLABS_BASE_URL"), "https://api.elevenlabs.io")
	c.VoiceID = orDefault(r.get("ELEVENLABS_VOICE_ID"), "EXAVITQu4vr4xnSDxMaL")
	c.VoiceIDKK = orDefault(r.get("ELEVENLABS_VOICE_ID_KK"), c.VoiceID)
	c.TTSModelRU = orDefault(r.get("ELEVENLABS_TTS_MODEL_RU"), "eleven_flash_v2_5")
	c.TTSModelKK = orDefault(r.get("ELEVENLABS_TTS_MODEL_KK"), "eleven_v3_conversational")
	c.TTSSpeed = r.getFloat(1.0, "ELEVENLABS_TTS_SPEED")
	c.STTModel = orDefault(r.get("ELEVENLABS_STT_MODEL"), "scribe_v2_realtime")
	c.STTLanguage = orDefault(r.get("ELEVENLABS_STT_LANGUAGE"), "kk")
	c.STTSecondary = r.getList(nil, "ELEVENLABS_STT_SECONDARY_LANGUAGES")
	c.STTKeyterms = r.getList(defaultSTTKeyterms, "ELEVENLABS_STT_KEYTERMS")

	c.VADSilenceSecs = r.getFloat(0.4, "VOICE_VAD_SILENCE_SECS")
	c.VADThreshold = r.getFloat(0, "VOICE_VAD_THRESHOLD")
	c.MinSpeechMS = r.getInt(0, "VOICE_MIN_SPEECH_MS")
	c.MinSilenceMS = r.getInt(0, "VOICE_MIN_SILENCE_MS")

	c.OpenRouterKey = r.get("OPENROUTER_API_KEY", "OPENROUTER_API", "OPENROUTER", "OPENROUTER_KEY")
	c.OpenRouterBase = orDefault(r.get("OPENROUTER_BASE_URL"), "https://openrouter.ai/api/v1")
	c.OpenRouterModel = orDefault(r.get("OPENROUTER_MODEL"), "google/gemini-2.5-flash-lite")
	c.OpenRouterFallbacks = r.getList(defaultOpenRouterFallbacks, "OPENROUTER_FALLBACK_MODELS")

	c.Brain = orDefault(r.get("VOICE_BRAIN"), "auto")
	c.BackendURL = r.get("BACKEND_URL")
	if dd := r.get("VOICE_DATA_DIR"); dd != "" {
		c.DataDir = dd
	} else if wd, err := os.Getwd(); err == nil {
		c.DataDir = FindDataDir(wd)
	}

	if addr := r.get("VOICE_HTTP_ADDR"); addr != "" {
		c.HTTPAddr = addr
	} else if port := r.get("PORT"); port != "" {
		c.HTTPAddr = ":" + port
	} else {
		c.HTTPAddr = ":8090"
	}
	c.AudioSocketAddr = r.get("VOICE_AUDIOSOCKET_ADDR")
	c.PublicURL = r.get("VOICE_PUBLIC_URL")
	c.AllowedOrigins = r.getList(defaultAllowedOrigins, "VOICE_ALLOWED_ORIGINS")

	c.LogDir = orDefault(r.get("VOICE_LOG_DIR"), "logs/voice")
	c.RecordAudio = r.getBool(false, "VOICE_RECORD_AUDIO")
	c.CacheDir = orDefault(r.get("VOICE_CACHE_DIR"), "cache/voice")
	c.Speculative = r.getBool(true, "VOICE_SPECULATIVE")
	c.SpeculativeTTS = r.getBool(false, "VOICE_SPECULATIVE_TTS")
	c.SpeculateAfter = r.getDurationMS(250*time.Millisecond, "VOICE_SPECULATE_AFTER_MS")
	c.FillerAfter = r.getDurationMS(1500*time.Millisecond, "VOICE_FILLER_AFTER_MS")
	c.BargeIn = r.getBool(true, "VOICE_BARGE_IN")
	c.BargeInVAD = r.getBool(false, "VOICE_BARGE_IN_VAD")
	c.Greeting = r.getBool(true, "VOICE_GREETING")

	return c
}

// orDefault returns v, or def when v is empty.
func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// secretState reports "set" or "missing" for a secret value without ever
// revealing it.
func secretState(v string) string {
	if v == "" {
		return "missing"
	}
	return "set"
}

// Redacted returns every field as a map suitable for logging: secrets
// (API keys) are replaced with "set" or "missing" so they never reach logs.
func (c Config) Redacted() map[string]any {
	return map[string]any{
		"ElevenLabsKey":  secretState(c.ElevenLabsKey),
		"ElevenLabsBase": c.ElevenLabsBase,
		"VoiceID":        c.VoiceID,
		"VoiceIDKK":      c.VoiceIDKK,
		"TTSModelRU":     c.TTSModelRU,
		"TTSModelKK":     c.TTSModelKK,
		"TTSSpeed":       c.TTSSpeed,
		"STTModel":       c.STTModel,
		"STTLanguage":    c.STTLanguage,
		"STTSecondary":   c.STTSecondary,
		"STTKeyterms":    c.STTKeyterms,

		"VADSilenceSecs": c.VADSilenceSecs,
		"VADThreshold":   c.VADThreshold,
		"MinSpeechMS":    c.MinSpeechMS,
		"MinSilenceMS":   c.MinSilenceMS,

		"OpenRouterKey":       secretState(c.OpenRouterKey),
		"OpenRouterBase":      c.OpenRouterBase,
		"OpenRouterModel":     c.OpenRouterModel,
		"OpenRouterFallbacks": c.OpenRouterFallbacks,

		"Brain":      c.Brain,
		"BackendURL": c.BackendURL,
		"DataDir":    c.DataDir,

		"HTTPAddr":        c.HTTPAddr,
		"AudioSocketAddr": c.AudioSocketAddr,
		"PublicURL":       c.PublicURL,
		"AllowedOrigins":  c.AllowedOrigins,

		"LogDir":         c.LogDir,
		"RecordAudio":    c.RecordAudio,
		"CacheDir":       c.CacheDir,
		"Speculative":    c.Speculative,
		"SpeculativeTTS": c.SpeculativeTTS,
		"SpeculateAfter": c.SpeculateAfter,
		"FillerAfter":    c.FillerAfter,
		"BargeIn":        c.BargeIn,
		"BargeInVAD":     c.BargeInVAD,
		"Greeting":       c.Greeting,
	}
}

// Validate returns human-readable warnings about the current configuration.
// An empty result does not guarantee the service will work end to end (keys
// may still be invalid) — it only flags configuration that is obviously
// incomplete or inconsistent.
func (c Config) Validate() []string {
	var warnings []string

	if c.ElevenLabsKey == "" {
		warnings = append(warnings, "ElevenLabs API key is missing")
	}
	if c.OpenRouterKey == "" {
		warnings = append(warnings, "OpenRouter key missing (brain falls back to echo)")
	}

	switch c.Brain {
	case "auto", "backend", "openrouter", "echo":
	default:
		warnings = append(warnings, fmt.Sprintf("VOICE_BRAIN=%q is not one of auto|backend|openrouter|echo", c.Brain))
	}
	if c.Brain == "backend" && c.BackendURL == "" {
		warnings = append(warnings, "VOICE_BRAIN=backend but BACKEND_URL is not set")
	}

	if c.DataDir == "" {
		warnings = append(warnings, "scenario data directory not found (set VOICE_DATA_DIR or run near a data/scenarios.json)")
	}

	if c.AudioSocketAddr != "" && c.PublicURL == "" {
		warnings = append(warnings, "VOICE_AUDIOSOCKET_ADDR is set but VOICE_PUBLIC_URL is empty (Twilio TwiML needs a public https URL)")
	}

	if c.TTSSpeed <= 0 {
		warnings = append(warnings, "ELEVENLABS_TTS_SPEED should be greater than 0")
	}
	if c.VADSilenceSecs < 0 {
		warnings = append(warnings, "VOICE_VAD_SILENCE_SECS should not be negative")
	}

	return warnings
}
