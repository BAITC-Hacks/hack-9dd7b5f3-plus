package voicelog

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// subscriberBuffer is the per-subscriber channel capacity.
const subscriberBuffer = 256

// pingInterval is how often ServeSSE sends a comment ping to keep idle
// connections alive through proxies.
const pingInterval = 15 * time.Second

// Hub fans out published log lines to any number of subscribers (e.g. SSE
// connections for an admin console), plus a bounded backlog so a new
// subscriber can catch up on recent history. It is safe for concurrent
// use.
type Hub struct {
	mu      sync.Mutex
	subs    map[chan []byte]struct{}
	backlog [][]byte
	maxBack int
}

// NewHub creates a Hub that retains up to backlog recent lines for new
// subscribers to replay.
func NewHub(backlog int) *Hub {
	if backlog < 0 {
		backlog = 0
	}
	return &Hub{
		subs:    make(map[chan []byte]struct{}),
		maxBack: backlog,
	}
}

// Publish appends line to the backlog and sends a copy to every current
// subscriber. Publish never blocks: a subscriber whose buffer is full
// simply misses the message.
func (h *Hub) Publish(line []byte) {
	if h == nil {
		return
	}
	cp := make([]byte, len(line))
	copy(cp, line)

	h.mu.Lock()
	if h.maxBack > 0 {
		h.backlog = append(h.backlog, cp)
		if len(h.backlog) > h.maxBack {
			h.backlog = append([][]byte(nil), h.backlog[len(h.backlog)-h.maxBack:]...)
		}
	}
	subs := make([]chan []byte, 0, len(h.subs))
	for ch := range h.subs {
		subs = append(subs, ch)
	}
	h.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- cp:
		default:
			// Slow subscriber: drop rather than block Publish.
		}
	}
}

// Subscribe registers a new subscriber and returns a channel of future
// published lines, a snapshot of the current backlog, and a cancel func
// that unregisters the subscriber. cancel is idempotent and safe to call
// more than once or from multiple goroutines.
func (h *Hub) Subscribe() (ch <-chan []byte, backlog [][]byte, cancel func()) {
	c := make(chan []byte, subscriberBuffer)

	h.mu.Lock()
	h.subs[c] = struct{}{}
	backlogCopy := make([][]byte, len(h.backlog))
	copy(backlogCopy, h.backlog)
	h.mu.Unlock()

	var once sync.Once
	cancelFn := func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.subs, c)
			h.mu.Unlock()
		})
	}

	return c, backlogCopy, cancelFn
}

// ServeSSE streams the current backlog and then any newly published lines
// to w as Server-Sent Events ("data: <json>\n\n"), sending a ": ping\n\n"
// comment every 15 seconds to keep the connection alive through proxies.
// It sets the standard SSE headers, flushes after every event, and returns
// once the request context is done.
func (h *Hub) ServeSSE(w http.ResponseWriter, r *http.Request) {
	header := w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-store")
	header.Set("X-Accel-Buffering", "no")
	header.Set("Access-Control-Allow-Origin", "*")

	flusher, canFlush := w.(http.Flusher)
	w.WriteHeader(http.StatusOK)
	if canFlush {
		flusher.Flush()
	}

	ch, backlog, cancel := h.Subscribe()
	defer cancel()

	write := func(data []byte) bool {
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return false
		}
		if canFlush {
			flusher.Flush()
		}
		return true
	}

	for _, line := range backlog {
		if !write(line) {
			return
		}
	}

	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-ch:
			if !ok {
				return
			}
			if !write(line) {
				return
			}
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			if canFlush {
				flusher.Flush()
			}
		}
	}
}
