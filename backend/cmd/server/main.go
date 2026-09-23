// Command server runs the Voice Router backend: REST + WebSocket API over
// the dialogue engine. Configuration comes from the environment (.env is
// loaded if present); with no API keys it runs fully in keyless mock mode.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"hackathon/backend/internal/catalog"
	"hackathon/backend/internal/config"
	"hackathon/backend/internal/dialog"
	"hackathon/backend/internal/events"
	"hackathon/backend/internal/httpapi"
	"hackathon/backend/internal/llm"
	"hackathon/backend/internal/mockbackend"
	"hackathon/backend/internal/retrieval"
	"hackathon/backend/internal/router"
	"hackathon/backend/internal/store"
	"hackathon/backend/internal/stt"
	"hackathon/backend/internal/tts"
)

func main() {
	config.LoadDotEnv(".env")
	config.LoadDotEnv("../.env")
	cfg := config.Load()

	cat, err := catalog.Load(cfg.DataDir)
	if err != nil {
		log.Fatalf("catalog: %v (set DATA_DIR to the starter kit folder)", err)
	}
	lexPath := os.Getenv("LEXICON_PATH")
	if lexPath == "" {
		lexPath = "config/lexicon.json"
		if _, err := os.Stat(lexPath); err != nil {
			lexPath = filepath.Join("backend", "config", "lexicon.json")
		}
	}
	lex, err := retrieval.LoadLexicon(lexPath)
	if err != nil {
		log.Printf("lexicon: %v (retrieval runs on scenario examples only)", err)
		lex = &retrieval.Lexicon{}
	}
	ix := retrieval.New(cat, lex)
	be, err := mockbackend.New(cat)
	if err != nil {
		log.Fatalf("mock backend: %v", err)
	}
	mock := router.NewMockRouter(cat, ix)
	var primary router.Router = mock
	if cfg.LLM.Provider == "openai" {
		p := llm.NewOpenAI(cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model, time.Duration(cfg.LLM.TimeoutMS)*time.Millisecond)
		primary = router.NewLLMRouter(cat, p, cfg.LLM.Temperature, cfg.LLM.MaxTokens, cfg.LLM.JSONMode)
	}

	var sttP stt.Provider
	switch cfg.STT.Provider {
	case "mock":
		sttP = stt.Mock{}
	case "elevenlabs":
		sttP = stt.NewElevenLabs(cfg.STT.BaseURL, cfg.STT.APIKey, cfg.STT.Model, cfg.STT.Language, time.Duration(cfg.STT.TimeoutMS)*time.Millisecond)
	case "elevenlabs_realtime":
		sttP = stt.NewElevenLabsRealtime(cfg.STT.BaseURL, cfg.STT.APIKey, cfg.STT.Language, cfg.STT.SecondaryLanguages, time.Duration(cfg.STT.TimeoutMS)*time.Millisecond)
	case "openai":
		sttP = stt.NewOpenAI(cfg.STT.BaseURL, cfg.STT.APIKey, cfg.STT.Model, cfg.STT.Language, cfg.STT.Prompt, time.Duration(cfg.STT.TimeoutMS)*time.Millisecond)
	default: // browser
		sttP = stt.Mock{}
	}
	var ttsP tts.Provider
	switch cfg.TTS.Provider {
	case "elevenlabs":
		ttsP = tts.NewElevenLabs(cfg.TTS.BaseURL, cfg.TTS.APIKey, cfg.TTS.Model, cfg.TTS.Voice, cfg.TTS.ModelKK, cfg.TTS.VoiceKK, cfg.TTS.SampleRate, cfg.TTS.Speed, time.Duration(cfg.TTS.TimeoutMS)*time.Millisecond)
	case "openai":
		ttsP = tts.NewOpenAI(cfg.TTS.BaseURL, cfg.TTS.APIKey, cfg.TTS.Model, cfg.TTS.Voice, cfg.TTS.ModelKK, cfg.TTS.VoiceKK, cfg.TTS.SampleRate, cfg.TTS.Speed, time.Duration(cfg.TTS.TimeoutMS)*time.Millisecond)
	default:
		ttsP = tts.Browser{}
	}

	st, err := store.Open(cfg.VarDir)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	bus := events.NewBus()
	eng := dialog.New(cfg, cat, ix, primary, mock, be, sttP, ttsP, st, bus)
	srv := httpapi.New(cfg, eng, lex, ttsP)

	httpSrv := &http.Server{Addr: ":" + cfg.Port, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Printf("voice router listening on :%s | llm=%s (%s) stt=%s tts=%s fast_path=%s data=%s", cfg.Port, cfg.LLM.Provider, cfg.LLM.Model, cfg.STT.Provider, cfg.TTS.Provider, cfg.FastPath, cfg.DataDir)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpSrv.Shutdown(ctx)
}
