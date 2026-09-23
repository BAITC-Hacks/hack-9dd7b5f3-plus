FROM golang:1.26.5-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/backend ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 backend
WORKDIR /app
COPY --from=build /out/backend /app/backend
USER backend
ENV PORT=8080
EXPOSE 8080
CMD ["/app/backend"]
