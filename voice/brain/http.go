package brain

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// sharedHTTP is the default client: HTTP/2 when offered, keep-alive
// connections pooled for 120 s with ping health checks, TLS session
// resumption, and no response compression (a gzip buffer would hold back
// streamed tokens).
var sharedHTTP = sync.OnceValue(func() *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Client{Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		TLSClientConfig:       &tls.Config{ClientSessionCache: tls.NewLRUClientSessionCache(64)},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   16,
		IdleConnTimeout:       120 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		ExpectContinueTimeout: time.Second,
		DisableCompression:    true,
		HTTP2: &http.HTTP2Config{
			SendPingTimeout: 30 * time.Second,
			PingTimeout:     5 * time.Second,
		},
	}}
})

// eachLine calls fn with every line of r (line ending removed) until fn
// asks to stop, fn fails, or r ends.
func eachLine(r io.Reader, fn func(line []byte) (stop bool, err error)) error {
	br := bufio.NewReaderSize(r, 16<<10)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			stop, ferr := fn(bytes.TrimRight(line, "\r\n"))
			if ferr != nil || stop {
				return ferr
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// payload returns the data of an SSE "data:" line or a raw NDJSON line, and
// nil for blank lines, ": comment" keep-alives and other SSE fields.
func payload(line []byte) []byte {
	line = bytes.TrimSpace(line)
	if len(line) == 0 || line[0] == ':' {
		return nil
	}
	if data, ok := bytes.CutPrefix(line, []byte("data:")); ok {
		return bytes.TrimSpace(data)
	}
	if line[0] == '{' {
		return line
	}
	return nil
}

// snippet reads and closes the body of a failed response and returns at most
// 300 characters of it.
func snippet(resp *http.Response) string {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	resp.Body.Close()
	return truncate(strings.TrimSpace(string(b)), 300)
}

// truncate shortens s to n runes.
func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

// release drains and closes body in the background, so an HTTP/1.1
// connection can be reused without making the caller wait for the stream end.
func release(body io.ReadCloser) {
	go func() {
		t := time.AfterFunc(2*time.Second, func() { body.Close() })
		defer t.Stop()
		io.Copy(io.Discard, io.LimitReader(body, 256<<10))
		body.Close()
	}()
}

// sleepCtx waits for d or until ctx is done.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
