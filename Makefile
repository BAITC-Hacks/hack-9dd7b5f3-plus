.PHONY: dev backend frontend build test eval audio-test audio-samples fmt

# Run backend (:8080) and frontend (:3000) together for local development.
dev:
	@trap 'kill 0' EXIT; \
	(cd backend && go run ./cmd/server) & \
	(cd frontend && npm run dev) & \
	wait

backend:
	cd backend && go run ./cmd/server

frontend:
	cd frontend && npm run dev

build:
	cd backend && go build -o bin/server ./cmd/server
	cd frontend && npm run build

test:
	cd backend && go test ./...

# Route the official dev set through the running backend and score it with data/evaluate.py.
eval:
	python3 scripts/eval.py

# Play the test recordings (tests/audio/*.wav) through the running backend.
audio-test:
	python3 scripts/audio_test.py

# Regenerate tests/audio/*.wav (macOS `say` by default; TTS provider when keys are set).
audio-samples:
	python3 scripts/make_audio.py

fmt:
	cd backend && gofmt -w cmd internal
