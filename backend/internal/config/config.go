// Package config loads runtime configuration from environment variables
// (and an optional .env file for local runs). Every value has a keyless
// default so the service starts with no external accounts at all.
package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// Config is the fully resolved runtime configuration.
type Config struct {
	Port        string
	DataDir     string
	VarDir      string
	CORSOrigins []string
	Debug       bool // include prompts / raw LLM output in traces and debug events

	LLM LLMConfig
	STT STTConfig
	TTS TTSConfig

	FastPath string // on | shadow | off
	Policy   PolicyConfig
}

// LLMConfig configures the routing/dialogue model. Any OpenAI-compatible
// chat-completions endpoint works (OpenAI, Groq, OpenRouter, Ollama, ...).
type LLMConfig struct {
	Provider    string // mock | openai
	BaseURL     string
	APIKey      string
	Model       string
	Temperature float64
	MaxTokens   int
	TimeoutMS   int
	JSONMode    bool
}

// STTConfig configures speech-to-text.
type STTConfig struct {
	Provider           string // mock | browser | elevenlabs | elevenlabs_realtime | openai
	BaseURL            string
	APIKey             string
	Model              string
	Language           string // primary language hint ("" = auto)
	SecondaryLanguages string // comma separated, realtime only
	Prompt             string // domain vocabulary hint for OpenAI-style STT
	TimeoutMS          int
}

// TTSConfig configures text-to-speech. Output is always PCM16 mono.
type TTSConfig struct {
	Provider   string // mock | browser | elevenlabs | openai
	BaseURL    string
	APIKey     string
	Model      string
	Voice      string
	ModelKK    string // optional override for Kazakh
	VoiceKK    string
	SampleRate int
	Speed      float64
	TimeoutMS  int
}

// PolicyConfig holds the decision-policy thresholds (all shown in the trace).
type PolicyConfig struct {
	ProceedMin        float64 // confidence >= ProceedMin -> proceed
	ClarifyMin        float64 // ClarifyMin <= confidence < ProceedMin -> clarify
	FastPathMinScore  float64
	FastPathMinMargin float64
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(env(key, "")); err == nil {
		return v
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v, err := strconv.ParseFloat(env(key, ""), 64); err == nil {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	switch strings.ToLower(env(key, "")) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}

// LoadDotEnv reads KEY=VALUE lines from path into the process environment
// without overriding variables that are already set.
func LoadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\'') {
			v = v[1 : len(v)-1]
		}
		if _, exists := os.LookupEnv(k); !exists && v != "" {
			os.Setenv(k, v)
		}
	}
}

// Load resolves the configuration from the environment.
func Load() Config {
	openaiKey := env("OPENAI_API_KEY", "")
	elevenKey := env("ELEVENLABS_API_KEY", "")

	llmProvider := strings.ToLower(env("LLM_PROVIDER", "mock"))
	llmKey := env("LLM_API_KEY", openaiKey)
	if llmProvider == "openai" && llmKey == "" {
		// Without a key the OpenAI-compatible client cannot work; fall back to mock so
		// the app still starts, and say so loudly in the config endpoint.
		llmProvider = "mock"
	}

	sttProvider := strings.ToLower(env("STT_PROVIDER", ""))
	if sttProvider == "" {
		switch {
		case elevenKey != "":
			sttProvider = "elevenlabs_realtime"
		case openaiKey != "":
			sttProvider = "openai"
		default:
			sttProvider = "browser"
		}
	}
	sttKey := env("STT_API_KEY", "")
	if sttKey == "" {
		if strings.HasPrefix(sttProvider, "elevenlabs") {
			sttKey = elevenKey
		} else {
			sttKey = openaiKey
		}
	}

	ttsProvider := strings.ToLower(env("TTS_PROVIDER", ""))
	if ttsProvider == "" {
		switch {
		case elevenKey != "":
			ttsProvider = "elevenlabs"
		case openaiKey != "":
			ttsProvider = "openai"
		default:
			ttsProvider = "browser"
		}
	}
	ttsKey := env("TTS_API_KEY", "")
	if ttsKey == "" {
		if ttsProvider == "elevenlabs" {
			ttsKey = elevenKey
		} else {
			ttsKey = openaiKey
		}
	}
	ttsModelDefault := "gpt-4o-mini-tts"
	ttsVoiceDefault := "coral"
	if ttsProvider == "elevenlabs" {
		ttsModelDefault = "eleven_flash_v2_5"
		ttsVoiceDefault = "21m00Tcm4TlvDq8ikWAM" // "Rachel", a stock multilingual voice
	}
	sttModelDefault := "gpt-4o-mini-transcribe"
	if sttProvider == "elevenlabs" {
		sttModelDefault = "scribe_v2"
	} else if sttProvider == "elevenlabs_realtime" {
		sttModelDefault = "scribe_v2_realtime"
	}

	origins := strings.Split(env("CORS_ORIGINS", "*"), ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}

	return Config{
		Port:        env("PORT", "8080"),
		DataDir:     env("DATA_DIR", "../data"),
		VarDir:      env("VAR_DIR", "./var"),
		CORSOrigins: origins,
		Debug:       envBool("DEBUG", true),
		LLM: LLMConfig{
			Provider:    llmProvider,
			BaseURL:     strings.TrimRight(env("LLM_BASE_URL", "https://api.openai.com/v1"), "/"),
			APIKey:      llmKey,
			Model:       env("LLM_MODEL", "gpt-4.1-mini"),
			Temperature: envFloat("LLM_TEMPERATURE", 0.1),
			MaxTokens:   envInt("LLM_MAX_TOKENS", 600),
			TimeoutMS:   envInt("LLM_TIMEOUT_MS", 20000),
			JSONMode:    envBool("LLM_JSON_MODE", true),
		},
		STT: STTConfig{
			Provider:           sttProvider,
			BaseURL:            strings.TrimRight(env("STT_BASE_URL", defaultSTTBase(sttProvider)), "/"),
			APIKey:             sttKey,
			Model:              env("STT_MODEL", sttModelDefault),
			Language:           env("STT_LANGUAGE", ""),
			SecondaryLanguages: env("STT_SECONDARY_LANGUAGES", ""),
			Prompt:             env("STT_PROMPT", "Saqta Insurance, ОГПО, КАСКО, ДМС, полис, страховка, сақтандыру, полисім, ЖСН, ИИН"),
			TimeoutMS:          envInt("STT_TIMEOUT_MS", 15000),
		},
		TTS: TTSConfig{
			Provider:   ttsProvider,
			BaseURL:    strings.TrimRight(env("TTS_BASE_URL", defaultTTSBase(ttsProvider)), "/"),
			APIKey:     ttsKey,
			Model:      env("TTS_MODEL", ttsModelDefault),
			Voice:      env("TTS_VOICE", ttsVoiceDefault),
			ModelKK:    env("TTS_MODEL_KK", ""),
			VoiceKK:    env("TTS_VOICE_KK", ""),
			SampleRate: envInt("TTS_SAMPLE_RATE", 24000),
			Speed:      envFloat("TTS_SPEED", 1.0),
			TimeoutMS:  envInt("TTS_TIMEOUT_MS", 15000),
		},
		FastPath: strings.ToLower(env("FAST_PATH", "on")),
		Policy: PolicyConfig{
			ProceedMin:        envFloat("POLICY_PROCEED_MIN", 0.55),
			ClarifyMin:        envFloat("POLICY_CLARIFY_MIN", 0.30),
			FastPathMinScore:  envFloat("FAST_PATH_MIN_SCORE", 0.75),
			FastPathMinMargin: envFloat("FAST_PATH_MIN_MARGIN", 0.35),
		},
	}
}

func defaultSTTBase(p string) string {
	switch p {
	case "elevenlabs", "elevenlabs_realtime":
		return "https://api.elevenlabs.io"
	default:
		return "https://api.openai.com/v1"
	}
}

func defaultTTSBase(p string) string {
	switch p {
	case "elevenlabs":
		return "https://api.elevenlabs.io"
	default:
		return "https://api.openai.com/v1"
	}
}

// Public returns a redacted view safe to expose through /api/config.
func (c Config) Public() map[string]any {
	return map[string]any{
		"llm": map[string]any{
			"provider": c.LLM.Provider, "model": c.LLM.Model, "base_url": c.LLM.BaseURL,
			"has_key": c.LLM.APIKey != "", "json_mode": c.LLM.JSONMode,
		},
		"stt": map[string]any{
			"provider": c.STT.Provider, "model": c.STT.Model, "base_url": c.STT.BaseURL,
			"has_key": c.STT.APIKey != "", "language": c.STT.Language,
		},
		"tts": map[string]any{
			"provider": c.TTS.Provider, "model": c.TTS.Model, "voice": c.TTS.Voice,
			"model_kk": c.TTS.ModelKK, "voice_kk": c.TTS.VoiceKK,
			"sample_rate": c.TTS.SampleRate, "has_key": c.TTS.APIKey != "",
		},
		"fast_path": c.FastPath,
		"policy": map[string]any{
			"proceed_min": c.Policy.ProceedMin, "clarify_min": c.Policy.ClarifyMin,
			"fast_path_min_score": c.Policy.FastPathMinScore, "fast_path_min_margin": c.Policy.FastPathMinMargin,
		},
		"debug": c.Debug,
	}
}
