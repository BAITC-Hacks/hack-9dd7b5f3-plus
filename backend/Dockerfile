FROM golang:1.26.5-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
FROM alpine:3.23
RUN apk add --no-cache ca-certificates && adduser -D app
WORKDIR /app
COPY --from=build /out/api /app/api
COPY data /app/data
USER app
ENV PORT=8080 DATA_DIR=/app/data/official
EXPOSE 8080
CMD ["/app/api"]
