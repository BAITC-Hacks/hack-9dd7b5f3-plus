// Package dialog is the per-turn orchestrator: triage → retrieval → prefetch
// facts → fast path or LLM router → policy → executor (actions, confirmation
// gate, topic stack) → sentence-streamed TTS → trace.
package dialog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"hackathon/backend/internal/router"
	"hackathon/backend/internal/stt"
)

// Session is one conversation's live state.
type Session struct {
	ID        string
	CreatedAt time.Time
	Channel   string

	mu           sync.Mutex
	turnMu       sync.Mutex // one turn at a time
	TurnIndex    int
	Turns        []router.TurnView
	Language     string
	Client       map[string]any
	ClientID     string
	Active       string
	Stack        []string
	Slots        map[string]any
	Pending      *router.PendingAction
	ClarifyCount int
	Handoff      *router.Handoff
	Closed       bool

	// audio state for the current utterance
	audioMu     sync.Mutex
	audio       []byte
	rt          stt.Session
	rtErr       error
	speechStart time.Time
	audioSink   func(pcm []byte)
	cancelTurn  context.CancelFunc
}

func newID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// SetAudioSink registers the callback that receives PCM frames for playback.
func (s *Session) SetAudioSink(f func(pcm []byte)) {
	s.audioMu.Lock()
	defer s.audioMu.Unlock()
	s.audioSink = f
}

func (s *Session) sink() func([]byte) {
	s.audioMu.Lock()
	defer s.audioMu.Unlock()
	return s.audioSink
}

// AppendAudio buffers PCM16 16 kHz audio for the current utterance.
func (s *Session) AppendAudio(pcm []byte) {
	s.audioMu.Lock()
	defer s.audioMu.Unlock()
	if s.speechStart.IsZero() {
		s.speechStart = time.Now()
	}
	s.audio = append(s.audio, pcm...)
}

func (s *Session) takeAudio() ([]byte, time.Time) {
	s.audioMu.Lock()
	defer s.audioMu.Unlock()
	a := s.audio
	st := s.speechStart
	s.audio = nil
	s.speechStart = time.Time{}
	return a, st
}

// Cancel aborts the turn in progress (barge-in).
func (s *Session) Cancel() {
	s.mu.Lock()
	c := s.cancelTurn
	s.mu.Unlock()
	if c != nil {
		c()
	}
}

// View is the API representation of the state.
type View struct {
	ID           string                `json:"id"`
	CreatedAt    time.Time             `json:"created_at"`
	Channel      string                `json:"channel"`
	TurnIndex    int                   `json:"turn_index"`
	Language     string                `json:"language"`
	ClientID     string                `json:"client_id,omitempty"`
	ClientName   string                `json:"client_name,omitempty"`
	Active       string                `json:"active_scenario,omitempty"`
	Stack        []string              `json:"stack"`
	Slots        map[string]any        `json:"slots"`
	Pending      *router.PendingAction `json:"pending_confirmation,omitempty"`
	ClarifyCount int                   `json:"clarify_count"`
	Handoff      *router.Handoff       `json:"handoff,omitempty"`
	Closed       bool                  `json:"closed"`
	Turns        []router.TurnView     `json:"turns"`
}

// View snapshots the session.
func (s *Session) View() View {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := View{ID: s.ID, CreatedAt: s.CreatedAt, Channel: s.Channel, TurnIndex: s.TurnIndex, Language: s.Language, ClientID: s.ClientID,
		Active: s.Active, Stack: append([]string{}, s.Stack...), Slots: map[string]any{}, Pending: s.Pending, ClarifyCount: s.ClarifyCount, Handoff: s.Handoff, Closed: s.Closed,
		Turns: append([]router.TurnView{}, s.Turns...)}
	for k, val := range s.Slots {
		v.Slots[k] = val
	}
	if s.Client != nil {
		if n, ok := s.Client["full_name"].(string); ok {
			v.ClientName = n
		}
	}
	if v.Stack == nil {
		v.Stack = []string{}
	}
	return v
}

func (s *Session) stateView(missing func(active string, slots map[string]any) []string) router.StateView {
	s.mu.Lock()
	defer s.mu.Unlock()
	sv := router.StateView{Client: s.Client, ActiveScenario: s.Active, Stack: append([]string{}, s.Stack...), PendingConfirmation: s.Pending,
		Language: s.Language, ClarifyCount: s.ClarifyCount, TurnIndex: s.TurnIndex, ActiveSlots: map[string]any{}}
	for k, v := range s.Slots {
		sv.ActiveSlots[k] = v
	}
	if s.Active != "" {
		sv.MissingSlots = missing(s.Active, s.Slots)
	}
	n := len(s.Turns)
	if n > 8 {
		n = 8
	}
	sv.RecentTurns = append([]router.TurnView{}, s.Turns[len(s.Turns)-n:]...)
	return sv
}
