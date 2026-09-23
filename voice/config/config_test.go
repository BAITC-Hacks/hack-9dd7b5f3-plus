package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// allEnvKeys lists every environment variable name the package reads
// (including every alias). Tests clear all of them first so a developer's
// real shell environment (e.g. a stray OPENROUTER_API_KEY) can never leak
// into a test's expectations.
var allEnvKeys = []string{
	"BACKEND_URL",
	"ELEVENLABS_API_KEY",
	"ELEVENLABS_BASE_URL",
	"ELEVENLABS_STT_KEYTERMS",
	"ELEVENLABS_STT_LANGUAGE",
	"ELEVENLABS_STT_MODEL",
	"ELEVENLABS_STT_SECONDARY_LANGUAGES",
	"ELEVENLABS_TTS_MODEL_KK",
	"ELEVENLABS_TTS_MODEL_RU",
	"ELEVENLABS_TTS_SPEED",
	"ELEVENLABS_VOICE_ID",
	"ELEVENLABS_VOICE_ID_KK",
	"ELEVENLABS",
	"ELEVEN_LABS",
	"ELEVEN_LABS_API_KEY",
	"OPENROUTER",
	"OPENROUTER_API",
	"OPENROUTER_API_KEY",
	"OPENROUTER_BASE_URL",
	"OPENROUTER_FALLBACK_MODELS",
	"OPENROUTER_KEY",
	"OPENROUTER_MODEL",
	"PORT",
	"VOICE_ALLOWED_ORIGINS",
	"VOICE_AUDIOSOCKET_ADDR",
	"VOICE_BARGE_IN",
	"VOICE_BARGE_IN_VAD",
	"VOICE_BRAIN",
	"VOICE_CACHE_DIR",
	"VOICE_DATA_DIR",
	"VOICE_ENV_FILE",
	"VOICE_FILLER_AFTER_MS",
	"VOICE_GREETING",
	"VOICE_HTTP_ADDR",
	"VOICE_LOG_DIR",
	"VOICE_MIN_SILENCE_MS",
	"VOICE_MIN_SPEECH_MS",
	"VOICE_PUBLIC_URL",
	"VOICE_RECORD_AUDIO",
	"VOICE_SPECULATE_AFTER_MS",
	"VOICE_SPECULATIVE",
	"VOICE_SPECULATIVE_TTS",
	"VOICE_VAD_SILENCE_SECS",
	"VOICE_VAD_THRESHOLD",
	"XI_API_KEY",
}

// clearEnv resets every key config.go looks at to "" (which this package
// treats as "unset") for the duration of the test, so the developer
// machine's real environment cannot leak into a test's expectations.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range allEnvKeys {
		t.Setenv(k, "")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func containsSubstring(list []string, substr string) bool {
	for _, s := range list {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// --- ParseDotEnv ------------------------------------------------------

func TestParseDotEnv(t *testing.T) {
	t.Run("hyphenated key normalizes", func(t *testing.T) {
		m := ParseDotEnv(strings.NewReader("eleven-labs=sk_test\n"))
		if got := m["ELEVEN_LABS"]; got != "sk_test" {
			t.Fatalf(`m["ELEVEN_LABS"] = %q, want "sk_test" (m=%#v)`, got, m)
		}
	})

	t.Run("openrouter-api hyphenated alias normalizes", func(t *testing.T) {
		m := ParseDotEnv(strings.NewReader("openrouter-api=sk-or-abc\n"))
		if got := m["OPENROUTER_API"]; got != "sk-or-abc" {
			t.Fatalf(`m["OPENROUTER_API"] = %q, want "sk-or-abc" (m=%#v)`, got, m)
		}
	})

	t.Run("double and single quotes are stripped", func(t *testing.T) {
		m := ParseDotEnv(strings.NewReader("FOO=\"bar baz\"\nBAR='single quoted'\n"))
		if m["FOO"] != "bar baz" {
			t.Errorf("FOO = %q, want %q", m["FOO"], "bar baz")
		}
		if m["BAR"] != "single quoted" {
			t.Errorf("BAR = %q, want %q", m["BAR"], "single quoted")
		}
	})

	t.Run("export prefix is stripped", func(t *testing.T) {
		m := ParseDotEnv(strings.NewReader("export FOO=bar\n"))
		if m["FOO"] != "bar" {
			t.Errorf("FOO = %q, want bar", m["FOO"])
		}
	})

	t.Run("comments and blank lines are ignored", func(t *testing.T) {
		m := ParseDotEnv(strings.NewReader("# a full-line comment\n\nFOO=bar\n   # indented comment\n"))
		if len(m) != 1 || m["FOO"] != "bar" {
			t.Errorf("m = %#v, want only FOO=bar", m)
		}
	})

	t.Run("inline comment on unquoted value is stripped", func(t *testing.T) {
		m := ParseDotEnv(strings.NewReader("FOO=bar #trailing comment\n"))
		if m["FOO"] != "bar" {
			t.Errorf("FOO = %q, want bar", m["FOO"])
		}
	})

	t.Run("quoted value keeps hash and inner spaces verbatim", func(t *testing.T) {
		m := ParseDotEnv(strings.NewReader(`FOO="bar #not a comment"` + "\n"))
		if m["FOO"] != "bar #not a comment" {
			t.Errorf("FOO = %q, want %q", m["FOO"], "bar #not a comment")
		}
	})

	t.Run("CRLF line endings", func(t *testing.T) {
		m := ParseDotEnv(strings.NewReader("FOO=bar\r\nBAZ=qux\r\n"))
		if m["FOO"] != "bar" || m["BAZ"] != "qux" {
			t.Errorf("m = %#v, want FOO=bar BAZ=qux", m)
		}
	})

	t.Run("later value overrides earlier for same key", func(t *testing.T) {
		m := ParseDotEnv(strings.NewReader("FOO=first\nFOO=second\n"))
		if m["FOO"] != "second" {
			t.Errorf("FOO = %q, want second", m["FOO"])
		}
	})

	t.Run("line without equals sign is ignored", func(t *testing.T) {
		m := ParseDotEnv(strings.NewReader("not a valid line\nFOO=bar\n"))
		if len(m) != 1 || m["FOO"] != "bar" {
			t.Errorf("m = %#v, want only FOO=bar", m)
		}
	})

	t.Run("value with equals sign keeps remainder intact", func(t *testing.T) {
		m := ParseDotEnv(strings.NewReader("FOO=a=b=c\n"))
		if m["FOO"] != "a=b=c" {
			t.Errorf("FOO = %q, want a=b=c", m["FOO"])
		}
	})
}

func TestNormalizeKey(t *testing.T) {
	cases := map[string]string{
		"eleven-labs": "ELEVEN_LABS",
		"  spaced  ":  "SPACED",
		"Already_OK":  "ALREADY_OK",
		"a-b-c":       "A_B_C",
	}
	for in, want := range cases {
		if got := normalizeKey(in); got != want {
			t.Errorf("normalizeKey(%q) = %q, want %q", in, got, want)
		}
	}
}

// --- alias resolution / source precedence ------------------------------

func TestLoadFrom_AliasResolutionFromFile(t *testing.T) {
	clearEnv(t)
	envFile := filepath.Join(t.TempDir(), ".env")
	writeFile(t, envFile, "eleven-labs=sk_test_from_file\nOPENROUTER_API=sk-or-from-file\n")

	c := LoadFrom(envFile)

	if c.ElevenLabsKey != "sk_test_from_file" {
		t.Errorf("ElevenLabsKey = %q, want sk_test_from_file", c.ElevenLabsKey)
	}
	if c.OpenRouterKey != "sk-or-from-file" {
		t.Errorf("OpenRouterKey = %q, want sk-or-from-file", c.OpenRouterKey)
	}
}

func TestLoadFrom_EnvOverridesFileForSameKey(t *testing.T) {
	clearEnv(t)
	envFile := filepath.Join(t.TempDir(), ".env")
	writeFile(t, envFile, "ELEVENLABS_API_KEY=from_file\n")
	t.Setenv("ELEVENLABS_API_KEY", "from_env")

	c := LoadFrom(envFile)
	if c.ElevenLabsKey != "from_env" {
		t.Errorf("ElevenLabsKey = %q, want from_env", c.ElevenLabsKey)
	}
}

func TestLoadFrom_AnyEnvAliasBeatsAnyFileAlias(t *testing.T) {
	clearEnv(t)
	envFile := filepath.Join(t.TempDir(), ".env")
	// XI_API_KEY is checked *before* ELEVENLABS_API_KEY within a single
	// source, but the whole process-environment source outranks the whole
	// file source, so ELEVENLABS_API_KEY from the environment must still win.
	writeFile(t, envFile, "XI_API_KEY=file_value\n")
	t.Setenv("ELEVENLABS_API_KEY", "env_value")

	c := LoadFrom(envFile)
	if c.ElevenLabsKey != "env_value" {
		t.Errorf("ElevenLabsKey = %q, want env_value", c.ElevenLabsKey)
	}
}

func TestLoadFrom_EmptyEnvValueFallsThroughToFile(t *testing.T) {
	clearEnv(t)
	envFile := filepath.Join(t.TempDir(), ".env")
	writeFile(t, envFile, "ELEVENLABS_API_KEY=from_file\n")
	t.Setenv("ELEVENLABS_API_KEY", "") // explicitly set but empty: "not set"

	c := LoadFrom(envFile)
	if c.ElevenLabsKey != "from_file" {
		t.Errorf("ElevenLabsKey = %q, want from_file", c.ElevenLabsKey)
	}
}

func TestLoadFrom_MissingFileIsIgnored(t *testing.T) {
	clearEnv(t)
	c := LoadFrom(filepath.Join(t.TempDir(), "does-not-exist.env"))
	if c.HTTPAddr != ":8090" {
		t.Errorf("HTTPAddr = %q, want :8090 (missing file should not error)", c.HTTPAddr)
	}
}

// --- defaults -----------------------------------------------------------

func TestLoadFrom_Defaults(t *testing.T) {
	clearEnv(t)
	c := LoadFrom("")

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"ElevenLabsKey", c.ElevenLabsKey, ""},
		{"ElevenLabsBase", c.ElevenLabsBase, "https://api.elevenlabs.io"},
		{"VoiceID", c.VoiceID, "EXAVITQu4vr4xnSDxMaL"},
		{"VoiceIDKK", c.VoiceIDKK, "EXAVITQu4vr4xnSDxMaL"},
		{"TTSModelRU", c.TTSModelRU, "eleven_flash_v2_5"},
		{"TTSModelKK", c.TTSModelKK, "eleven_v3_conversational"},
		{"TTSSpeed", c.TTSSpeed, 1.0},
		{"STTModel", c.STTModel, "scribe_v2_realtime"},
		{"STTLanguage", c.STTLanguage, "kk"},
		{"VADSilenceSecs", c.VADSilenceSecs, 0.4},
		{"VADThreshold", c.VADThreshold, 0.0},
		{"MinSpeechMS", c.MinSpeechMS, 0},
		{"MinSilenceMS", c.MinSilenceMS, 0},
		{"OpenRouterKey", c.OpenRouterKey, ""},
		{"OpenRouterBase", c.OpenRouterBase, "https://openrouter.ai/api/v1"},
		{"OpenRouterModel", c.OpenRouterModel, "google/gemini-2.5-flash-lite"},
		{"Brain", c.Brain, "auto"},
		{"BackendURL", c.BackendURL, ""},
		{"HTTPAddr", c.HTTPAddr, ":8090"},
		{"AudioSocketAddr", c.AudioSocketAddr, ""},
		{"PublicURL", c.PublicURL, ""},
		{"LogDir", c.LogDir, "logs/voice"},
		{"RecordAudio", c.RecordAudio, false},
		{"CacheDir", c.CacheDir, "cache/voice"},
		{"Speculative", c.Speculative, true},
		{"SpeculativeTTS", c.SpeculativeTTS, false},
		{"SpeculateAfter", c.SpeculateAfter, 250 * time.Millisecond},
		{"FillerAfter", c.FillerAfter, 1500 * time.Millisecond},
		{"BargeIn", c.BargeIn, true},
		{"BargeInVAD", c.BargeInVAD, false},
		{"Greeting", c.Greeting, true},
	}
	for _, tc := range checks {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}

	if c.STTSecondary != nil {
		t.Errorf("STTSecondary = %#v, want nil/empty", c.STTSecondary)
	}
	wantKeyterms := []string{"Saqta", "ОГПО", "КАСКО", "ДМС", "полис", "сақтандыру"}
	if !reflect.DeepEqual(c.STTKeyterms, wantKeyterms) {
		t.Errorf("STTKeyterms = %#v, want %#v", c.STTKeyterms, wantKeyterms)
	}
	wantFallbacks := []string{"openai/gpt-4o-mini", "openai/gpt-4.1-nano"}
	if !reflect.DeepEqual(c.OpenRouterFallbacks, wantFallbacks) {
		t.Errorf("OpenRouterFallbacks = %#v, want %#v", c.OpenRouterFallbacks, wantFallbacks)
	}
	wantOrigins := []string{"*"}
	if !reflect.DeepEqual(c.AllowedOrigins, wantOrigins) {
		t.Errorf("AllowedOrigins = %#v, want %#v", c.AllowedOrigins, wantOrigins)
	}
}

func TestLoadFrom_DataDirExplicit(t *testing.T) {
	clearEnv(t)
	t.Setenv("VOICE_DATA_DIR", filepath.Join("custom", "data"))
	c := LoadFrom("")
	if c.DataDir != filepath.Join("custom", "data") {
		t.Errorf("DataDir = %q, want custom/data", c.DataDir)
	}
}

// TestLoadFrom_DataDirDefaultsToRepoDataDir exercises the real
// FindDataDir(cwd) default path used by LoadFrom when VOICE_DATA_DIR is
// unset. `go test` runs with the working directory set to this package
// (voice/config), and the monorepo keeps its starter-kit data two levels
// up (voice/config -> voice -> <repo root>/data), so this doubles as an
// integration check of that layout. It reads no file content, only
// os.Stat, so it never touches secret data.
func TestLoadFrom_DataDirDefaultsToRepoDataDir(t *testing.T) {
	clearEnv(t)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	want := filepath.Join(filepath.Dir(filepath.Dir(wd)), "data")
	if _, err := os.Stat(filepath.Join(want, "scenarios.json")); err != nil {
		t.Skipf("repo data dir not found at %q (%v); skipping layout-dependent check", want, err)
	}

	c := LoadFrom("")
	if c.DataDir != want {
		t.Errorf("DataDir = %q, want %q", c.DataDir, want)
	}
}

// --- PORT -> HTTPAddr -----------------------------------------------------

func TestLoadFrom_HTTPAddr(t *testing.T) {
	t.Run("default when nothing set", func(t *testing.T) {
		clearEnv(t)
		c := LoadFrom("")
		if c.HTTPAddr != ":8090" {
			t.Errorf("HTTPAddr = %q, want :8090", c.HTTPAddr)
		}
	})

	t.Run("PORT sets HTTPAddr", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("PORT", "3000")
		c := LoadFrom("")
		if c.HTTPAddr != ":3000" {
			t.Errorf("HTTPAddr = %q, want :3000", c.HTTPAddr)
		}
	})

	t.Run("VOICE_HTTP_ADDR beats PORT", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("PORT", "3000")
		t.Setenv("VOICE_HTTP_ADDR", ":9999")
		c := LoadFrom("")
		if c.HTTPAddr != ":9999" {
			t.Errorf("HTTPAddr = %q, want :9999", c.HTTPAddr)
		}
	})
}

// --- list parsing ---------------------------------------------------------

func TestLoadFrom_ListParsing(t *testing.T) {
	clearEnv(t)
	t.Setenv("ELEVENLABS_STT_SECONDARY_LANGUAGES", " ru , kk ,, en ")
	t.Setenv("VOICE_ALLOWED_ORIGINS", "http://localhost:3000,https://example.com")

	c := LoadFrom("")

	wantSecondary := []string{"ru", "kk", "en"}
	if !reflect.DeepEqual(c.STTSecondary, wantSecondary) {
		t.Errorf("STTSecondary = %#v, want %#v", c.STTSecondary, wantSecondary)
	}
	wantOrigins := []string{"http://localhost:3000", "https://example.com"}
	if !reflect.DeepEqual(c.AllowedOrigins, wantOrigins) {
		t.Errorf("AllowedOrigins = %#v, want %#v", c.AllowedOrigins, wantOrigins)
	}
}

func TestLoadFrom_ListParsing_BlankEntriesFallBackToDefault(t *testing.T) {
	clearEnv(t)
	t.Setenv("VOICE_ALLOWED_ORIGINS", "  ,  , ")
	c := LoadFrom("")
	if !reflect.DeepEqual(c.AllowedOrigins, []string{"*"}) {
		t.Errorf("AllowedOrigins = %#v, want [*]", c.AllowedOrigins)
	}
}

// --- duration parsing -------------------------------------------------

func TestLoadFrom_DurationParsing(t *testing.T) {
	clearEnv(t)
	t.Setenv("VOICE_SPECULATE_AFTER_MS", "400")
	t.Setenv("VOICE_FILLER_AFTER_MS", "0")

	c := LoadFrom("")
	if c.SpeculateAfter != 400*time.Millisecond {
		t.Errorf("SpeculateAfter = %v, want 400ms", c.SpeculateAfter)
	}
	if c.FillerAfter != 0 {
		t.Errorf("FillerAfter = %v, want 0 (disabled)", c.FillerAfter)
	}
}

func TestLoadFrom_DurationParsing_InvalidFallsBackToDefault(t *testing.T) {
	clearEnv(t)
	t.Setenv("VOICE_SPECULATE_AFTER_MS", "not-a-number")
	c := LoadFrom("")
	if c.SpeculateAfter != 250*time.Millisecond {
		t.Errorf("SpeculateAfter = %v, want default 250ms", c.SpeculateAfter)
	}
}

// --- bool parsing -------------------------------------------------------

func TestLoadFrom_BoolParsing(t *testing.T) {
	truthy := []string{"1", "true", "TRUE", "True", "yes", "YES", "on", "On"}
	falsy := []string{"0", "false", "FALSE", "no", "NO", "off", "OFF"}

	for _, v := range truthy {
		clearEnv(t)
		t.Setenv("VOICE_GREETING", v)
		if c := LoadFrom(""); !c.Greeting {
			t.Errorf("VOICE_GREETING=%q -> Greeting = false, want true", v)
		}
	}
	for _, v := range falsy {
		clearEnv(t)
		t.Setenv("VOICE_GREETING", v)
		if c := LoadFrom(""); c.Greeting {
			t.Errorf("VOICE_GREETING=%q -> Greeting = true, want false", v)
		}
	}

	t.Run("invalid value falls back to default", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("VOICE_GREETING", "maybe")
		if c := LoadFrom(""); !c.Greeting {
			t.Errorf("Greeting = false, want default true for invalid value")
		}
	})
}

// --- FindDotEnv / FindDataDir --------------------------------------------

func TestFindDotEnv_WalksUpFromNestedDir(t *testing.T) {
	root := t.TempDir()
	envPath := filepath.Join(root, ".env")
	writeFile(t, envPath, "FOO=bar\n")

	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	got := FindDotEnv(nested)
	want, err := filepath.Abs(envPath)
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	if got != want {
		t.Errorf("FindDotEnv(%q) = %q, want %q", nested, got, want)
	}
}

func TestFindDotEnv_NotFound(t *testing.T) {
	dir := t.TempDir()
	if got := FindDotEnv(dir); got != "" {
		t.Errorf("FindDotEnv(%q) = %q, want empty", dir, got)
	}
}

func TestFindDotEnv_EmptyStart(t *testing.T) {
	if got := FindDotEnv(""); got != "" {
		t.Errorf(`FindDotEnv("") = %q, want empty`, got)
	}
}

func TestFindDataDir_WalksUp(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	writeFile(t, filepath.Join(dataDir, "scenarios.json"), "[]")

	nested := filepath.Join(root, "x", "y")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	got := FindDataDir(nested)
	want, err := filepath.Abs(dataDir)
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	if got != want {
		t.Errorf("FindDataDir(%q) = %q, want %q", nested, got, want)
	}
}

func TestFindDataDir_RequiresScenariosFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// "data" exists but has no scenarios.json, so it must not count.
	if got := FindDataDir(root); got != "" {
		t.Errorf("FindDataDir(%q) = %q, want empty", root, got)
	}
}

func TestFindDataDir_NotFound(t *testing.T) {
	dir := t.TempDir()
	if got := FindDataDir(dir); got != "" {
		t.Errorf("FindDataDir(%q) = %q, want empty", dir, got)
	}
}

// --- Load() itself, via VOICE_ENV_FILE only (never the real .env) --------

func TestLoad_UsesVoiceEnvFile(t *testing.T) {
	clearEnv(t)
	envFile := filepath.Join(t.TempDir(), "custom.env")
	writeFile(t, envFile, "ELEVENLABS_API_KEY=from_voice_env_file\n")
	t.Setenv("VOICE_ENV_FILE", envFile)

	c := Load()
	if c.ElevenLabsKey != "from_voice_env_file" {
		t.Errorf("ElevenLabsKey = %q, want from_voice_env_file", c.ElevenLabsKey)
	}
}

func TestLoad_MissingVoiceEnvFileIsIgnored(t *testing.T) {
	clearEnv(t)
	t.Setenv("VOICE_ENV_FILE", filepath.Join(t.TempDir(), "does-not-exist.env"))

	c := Load()
	if c.HTTPAddr != ":8090" {
		t.Errorf("HTTPAddr = %q, want :8090", c.HTTPAddr)
	}
}

// --- Redacted -------------------------------------------------------------

func TestConfig_Redacted_HidesSecrets(t *testing.T) {
	c := Config{
		ElevenLabsKey:  "sk_super_secret",
		OpenRouterKey:  "sk-or-super-secret",
		ElevenLabsBase: "https://api.elevenlabs.io",
	}
	red := c.Redacted()

	if red["ElevenLabsKey"] != "set" {
		t.Errorf("ElevenLabsKey redacted = %v, want set", red["ElevenLabsKey"])
	}
	if red["OpenRouterKey"] != "set" {
		t.Errorf("OpenRouterKey redacted = %v, want set", red["OpenRouterKey"])
	}
	if red["ElevenLabsBase"] != "https://api.elevenlabs.io" {
		t.Errorf("ElevenLabsBase = %v, want passthrough value", red["ElevenLabsBase"])
	}

	for k, v := range red {
		if s, ok := v.(string); ok && (s == "sk_super_secret" || s == "sk-or-super-secret") {
			t.Errorf("field %s leaks a raw secret value: %v", k, v)
		}
	}
}

func TestConfig_Redacted_MissingSecrets(t *testing.T) {
	red := Config{}.Redacted()
	if red["ElevenLabsKey"] != "missing" {
		t.Errorf("ElevenLabsKey redacted = %v, want missing", red["ElevenLabsKey"])
	}
	if red["OpenRouterKey"] != "missing" {
		t.Errorf("OpenRouterKey redacted = %v, want missing", red["OpenRouterKey"])
	}
}

// --- Validate ---------------------------------------------------------

func TestConfig_Validate(t *testing.T) {
	base := Config{
		ElevenLabsKey:  "x",
		OpenRouterKey:  "x",
		Brain:          "auto",
		DataDir:        filepath.Join("tmp", "data"),
		TTSSpeed:       1.0,
		VADSilenceSecs: 0.5,
	}

	t.Run("clean config has no warnings", func(t *testing.T) {
		if warnings := base.Validate(); len(warnings) != 0 {
			t.Errorf("warnings = %v, want none", warnings)
		}
	})

	t.Run("missing keys warn", func(t *testing.T) {
		c := base
		c.ElevenLabsKey = ""
		c.OpenRouterKey = ""
		warnings := c.Validate()
		if !containsSubstring(warnings, "ElevenLabs API key is missing") {
			t.Errorf("warnings = %v, want ElevenLabs warning", warnings)
		}
		if !containsSubstring(warnings, "OpenRouter key missing") {
			t.Errorf("warnings = %v, want OpenRouter warning", warnings)
		}
	})

	t.Run("backend brain without backend url warns", func(t *testing.T) {
		c := base
		c.Brain = "backend"
		c.BackendURL = ""
		warnings := c.Validate()
		if !containsSubstring(warnings, "BACKEND_URL") {
			t.Errorf("warnings = %v, want BACKEND_URL warning", warnings)
		}
	})

	t.Run("unknown brain warns", func(t *testing.T) {
		c := base
		c.Brain = "bogus"
		warnings := c.Validate()
		if !containsSubstring(warnings, "VOICE_BRAIN") {
			t.Errorf("warnings = %v, want VOICE_BRAIN warning", warnings)
		}
	})

	t.Run("audiosocket without public url warns", func(t *testing.T) {
		c := base
		c.AudioSocketAddr = ":9092"
		c.PublicURL = ""
		warnings := c.Validate()
		if !containsSubstring(warnings, "VOICE_PUBLIC_URL") {
			t.Errorf("warnings = %v, want VOICE_PUBLIC_URL warning", warnings)
		}
	})

	t.Run("missing data dir warns", func(t *testing.T) {
		c := base
		c.DataDir = ""
		warnings := c.Validate()
		if !containsSubstring(warnings, "scenario data directory") {
			t.Errorf("warnings = %v, want data dir warning", warnings)
		}
	})

	t.Run("non-positive tts speed warns", func(t *testing.T) {
		c := base
		c.TTSSpeed = 0
		warnings := c.Validate()
		if !containsSubstring(warnings, "ELEVENLABS_TTS_SPEED") {
			t.Errorf("warnings = %v, want TTS speed warning", warnings)
		}
	})

	t.Run("negative vad silence warns", func(t *testing.T) {
		c := base
		c.VADSilenceSecs = -1
		warnings := c.Validate()
		if !containsSubstring(warnings, "VOICE_VAD_SILENCE_SECS") {
			t.Errorf("warnings = %v, want VAD silence warning", warnings)
		}
	})
}
