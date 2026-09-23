// Command voice runs the real-time voice gateway: browser WebSocket
// (/ws/voice), phone calls (Asterisk AudioSocket for a KZ number, Twilio),
// live admin events and TTS/STT helper endpoints. Configuration: voice/.env.example.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"hackathon/voice/config"
	"hackathon/voice/server"
)

func main() {
	cfg := config.Load()
	for _, w := range cfg.Validate() {
		slog.Warn("config", "warning", w)
	}
	srv, err := server.New(cfg)
	if err != nil {
		slog.Error("cannot start", "err", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := srv.Run(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("voice gateway stopped", "err", err)
		os.Exit(1)
	}
}
