package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"hackathon/voice/config"
)

func TestPagesServed(t *testing.T) {
	cfg := config.Config{ElevenLabsKey: "test", Brain: "echo", LogDir: t.TempDir(), AllowedOrigins: []string{"*"}}
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	h := srv.Handler()
	for path, want := range map[string]string{"/": "/ws/voice", "/call": "Saqta Insurance", "/phone": "down16k"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("%s: code %d, missing %q", path, rec.Code, want)
		}
	}
}
