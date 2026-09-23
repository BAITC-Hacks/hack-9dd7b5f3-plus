// Package elevenlabs is a small, latency-focused client for the ElevenLabs
// speech APIs used by the voice gateway:
//
//   - Scribe v2 Realtime (streaming speech-to-text over WebSocket)
//   - Scribe v2 (batch speech-to-text for whole recordings)
//   - Text-to-speech over HTTP streaming and over WebSockets
//     (stream-input for Flash v2.5, text-to-dialogue for eleven_v3_conversational,
//     the only realtime model that speaks Kazakh)
//
// Every call reuses one tuned HTTP transport (keep-alive, HTTP/2) so that the
// TLS handshake is paid once per process, not once per utterance.
package elevenlabs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// DefaultBaseURL is the global (latency-routed) API endpoint.
const DefaultBaseURL = "https://api.elevenlabs.io"

// Client talks to the ElevenLabs API with one API key.
type Client struct {
	APIKey  string
	BaseURL string // https://api.elevenlabs.io (or a residency endpoint)
	WSURL   string // derived from BaseURL: wss://api.elevenlabs.io
	HTTP    *http.Client
}

// NewClient returns a client with a keep-alive transport tuned for streaming.
func NewClient(apiKey, baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	ws := "wss://" + strings.TrimPrefix(baseURL, "https://")
	if strings.HasPrefix(baseURL, "http://") {
		ws = "ws://" + strings.TrimPrefix(baseURL, "http://")
	}
	return &Client{APIKey: apiKey, BaseURL: baseURL, WSURL: ws, HTTP: &http.Client{Transport: NewTransport()}}
}

// NewTransport is an http.Transport tuned for many short streaming requests.
func NewTransport() *http.Transport {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   16,
		IdleConnTimeout:       120 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
}

// APIError is a non-2xx answer from the API.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("elevenlabs: HTTP %d: %s", e.Status, e.Body)
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("xi-api-key", c.APIKey)
	return req, nil
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		resp.Body.Close()
		return nil, &APIError{Status: resp.StatusCode, Body: strings.TrimSpace(string(b))}
	}
	return resp, nil
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}

// Warm opens a TLS connection to the API and leaves it in the idle pool, so
// the first real request of a call does not pay the handshake (~100-300 ms).
func (c *Client) Warm(ctx context.Context) error {
	var sub json.RawMessage
	return c.getJSON(ctx, "/v1/user/subscription", &sub)
}

// dial opens an authenticated WebSocket. The HTTP handshake response body is
// surfaced as an *APIError when the upgrade is refused (bad key, bad params).
func (c *Client) dial(ctx context.Context, path string, q url.Values) (*websocket.Conn, error) {
	u := c.WSURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	h := http.Header{}
	h.Set("xi-api-key", c.APIKey)
	conn, resp, err := websocket.Dial(ctx, u, &websocket.DialOptions{HTTPHeader: h})
	if err != nil {
		if resp != nil && resp.StatusCode != http.StatusSwitchingProtocols {
			var body string
			if resp.Body != nil {
				b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
				body = strings.TrimSpace(string(b))
			}
			return nil, &APIError{Status: resp.StatusCode, Body: body}
		}
		return nil, err
	}
	conn.SetReadLimit(16 << 20)
	return conn, nil
}
