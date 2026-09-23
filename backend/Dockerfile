# Voice Router backend — single static binary + the starter kit data.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/server /app/server
COPY data/ /app/data/
COPY backend/config/ /app/config/
ENV PORT=8080 DATA_DIR=/app/data VAR_DIR=/app/var LEXICON_PATH=/app/config/lexicon.json
EXPOSE 8080
CMD ["/app/server"]
