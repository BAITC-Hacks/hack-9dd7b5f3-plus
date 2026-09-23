package main

import (
	httpapi "hackathon/backend/internal/http"
	"log/slog"
	"os"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := httpapi.Run(); err != nil {
		slog.Error("server_failed", "error", err.Error())
		os.Exit(1)
	}
}
