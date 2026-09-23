// Package events is a tiny pub/sub bus. Every pipeline step publishes an
// Event; the WebSocket handler forwards a session's events to the browser and
// the debug SSE endpoint forwards everything (for the Debug page and curl).
package events

import (
	"sync"
	"time"
)

// Event is the envelope sent to the UI. Data is type-specific.
type Event struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`
	Turn      int    `json:"turn,omitempty"`
	T         int64  `json:"t"` // unix ms
	Data      any    `json:"data,omitempty"`
}

type subscriber struct {
	session string // "" = all
	ch      chan Event
}

// Bus fans events out to subscribers without blocking publishers.
type Bus struct {
	mu   sync.RWMutex
	subs map[*subscriber]struct{}
	ring []Event // recent events for late debug subscribers
}

func NewBus() *Bus { return &Bus{subs: map[*subscriber]struct{}{}} }

// Publish delivers ev to matching subscribers (drops on a full buffer).
func (b *Bus) Publish(ev Event) {
	if ev.T == 0 {
		ev.T = time.Now().UnixMilli()
	}
	b.mu.Lock()
	b.ring = append(b.ring, ev)
	if len(b.ring) > 500 {
		b.ring = b.ring[len(b.ring)-500:]
	}
	b.mu.Unlock()
	b.mu.RLock()
	defer b.mu.RUnlock()
	for s := range b.subs {
		if s.session != "" && s.session != ev.SessionID {
			continue
		}
		select {
		case s.ch <- ev:
		default:
		}
	}
}

// Subscribe returns a channel of events for one session ("" for all) and a
// cancel function.
func (b *Bus) Subscribe(session string, buffer int) (<-chan Event, func()) {
	s := &subscriber{session: session, ch: make(chan Event, buffer)}
	b.mu.Lock()
	b.subs[s] = struct{}{}
	b.mu.Unlock()
	return s.ch, func() {
		b.mu.Lock()
		delete(b.subs, s)
		b.mu.Unlock()
	}
}

// Recent returns the last events (for the debug page on connect).
func (b *Bus) Recent(n int) []Event {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if n > len(b.ring) {
		n = len(b.ring)
	}
	out := make([]Event, n)
	copy(out, b.ring[len(b.ring)-n:])
	return out
}
