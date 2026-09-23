# Voice gateway image. Build context = repository root (the image bundles data/
# for the built-in Saqta brain):
#   docker build -f voice/Dockerfile -t saqta-voice .
FROM golang:1.26-alpine AS build
WORKDIR /src/voice
COPY voice/go.mod voice/go.sum ./
RUN go mod download
COPY voice/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/voice ./cmd/voice

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 voice
WORKDIR /app
COPY --from=build /out/voice /app/voice
COPY data /app/data
RUN mkdir -p /app/logs /app/cache && chown -R voice /app
USER voice
ENV VOICE_DATA_DIR=/app/data \
    VOICE_HTTP_ADDR=:8090 \
    VOICE_LOG_DIR=/app/logs \
    VOICE_CACHE_DIR=/app/cache
EXPOSE 8090 9092
CMD ["/app/voice"]
