// Package store keeps sessions and turn traces in memory and appends every
// trace to a JSONL file so the supervisor statistics survive restarts. No
// database is required; swapping this for Postgres means implementing the
// same few methods.
package store

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Record is the indexed summary of one turn plus the full trace JSON.
type Record struct {
	SessionID     string          `json:"session_id"`
	Turn          int             `json:"turn"`
	At            time.Time       `json:"at"`
	Transcript    string          `json:"transcript"`
	Language      string          `json:"language"`
	Scenarios     []string        `json:"scenarios"`
	Confidence    float64         `json:"confidence"`
	Path          string          `json:"path"` // llm | fast | mock | continuation
	PolicyAction  string          `json:"policy_action"`
	Handoff       bool            `json:"handoff"`
	FastPathAgree *bool           `json:"fast_path_agree,omitempty"`
	Timings       map[string]int  `json:"timings"`
	Trace         json.RawMessage `json:"trace"`
}

// Session is the persistent view of a conversation.
type Session struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Channel   string    `json:"channel"`
	Turns     int       `json:"turns"`
	LastAt    time.Time `json:"last_at"`
	Title     string    `json:"title"`
	Language  string    `json:"language"`
	ClientID  string    `json:"client_id,omitempty"`
	Handoff   bool      `json:"handoff"`
}

type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	records  []Record
	path     string
	file     *os.File
}

// Open loads the JSONL log from varDir (if any) and opens it for appending.
func Open(varDir string) (*Store, error) {
	if err := os.MkdirAll(varDir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{sessions: map[string]*Session{}, path: filepath.Join(varDir, "traces.jsonl")}
	if f, err := os.Open(s.path); err == nil {
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
		for sc.Scan() {
			var r Record
			if json.Unmarshal(sc.Bytes(), &r) == nil {
				s.records = append(s.records, r)
				s.touch(r)
			}
		}
		f.Close()
	}
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	s.file = f
	return s, nil
}

func (s *Store) touch(r Record) {
	sess, ok := s.sessions[r.SessionID]
	if !ok {
		sess = &Session{ID: r.SessionID, CreatedAt: r.At, Channel: "restored"}
		s.sessions[r.SessionID] = sess
	}
	if r.Turn > sess.Turns {
		sess.Turns = r.Turn
	}
	if r.At.After(sess.LastAt) {
		sess.LastAt = r.At
	}
	if sess.Title == "" && r.Transcript != "" {
		sess.Title = r.Transcript
	}
	if r.Language != "" {
		sess.Language = r.Language
	}
	if r.Handoff {
		sess.Handoff = true
	}
}

// CreateSession registers a new session.
func (s *Store) CreateSession(id, channel string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := &Session{ID: id, CreatedAt: time.Now(), Channel: channel, LastAt: time.Now()}
	s.sessions[id] = sess
	return sess
}

// SetClient records the identified client on a session.
func (s *Store) SetClient(id, clientID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		sess.ClientID = clientID
	}
}

// Append persists a turn record.
func (s *Store) Append(r Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, r)
	s.touch(r)
	if s.file != nil {
		if b, err := json.Marshal(r); err == nil {
			s.file.Write(append(b, '\n'))
		}
	}
}

// Sessions lists sessions, newest first.
func (s *Store) Sessions(limit int) []Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Session, 0, len(s.sessions))
	for _, v := range s.sessions {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastAt.After(out[j].LastAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// Session returns one session and its records.
func (s *Store) Session(id string) (*Session, []Record) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	if !ok {
		return nil, nil
	}
	cp := *sess
	var recs []Record
	for _, r := range s.records {
		if r.SessionID == id {
			recs = append(recs, r)
		}
	}
	return &cp, recs
}

// Records returns the most recent n records (newest last).
func (s *Store) Records(n int) []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if n <= 0 || n > len(s.records) {
		n = len(s.records)
	}
	out := make([]Record, n)
	copy(out, s.records[len(s.records)-n:])
	return out
}

// Stats aggregates the supervisor metrics.
type Stats struct {
	Turns           int                    `json:"turns"`
	Sessions        int                    `json:"sessions"`
	ByScenario      map[string]int         `json:"by_scenario"`
	ByLanguage      map[string]int         `json:"by_language"`
	ByPath          map[string]int         `json:"by_path"`
	ByPolicy        map[string]int         `json:"by_policy"`
	LowConfidence   int                    `json:"low_confidence"`
	Handoffs        int                    `json:"handoffs"`
	FastPathChecked int                    `json:"fast_path_checked"`
	FastPathAgree   int                    `json:"fast_path_agree"`
	Timings         map[string]Percentiles `json:"timings"`
	TimingsByPath   map[string]Percentiles `json:"first_audio_by_path"`
	Uncertain       []Record               `json:"uncertain"`
	Recent          []Record               `json:"recent"`
}

type Percentiles struct {
	N   int `json:"n"`
	P50 int `json:"p50"`
	P90 int `json:"p90"`
	Avg int `json:"avg"`
	Max int `json:"max"`
}

func percentiles(vals []int) Percentiles {
	if len(vals) == 0 {
		return Percentiles{}
	}
	sort.Ints(vals)
	sum := 0
	for _, v := range vals {
		sum += v
	}
	idx := func(p float64) int {
		i := int(math.Ceil(p*float64(len(vals)))) - 1
		if i < 0 {
			i = 0
		}
		return vals[i]
	}
	return Percentiles{N: len(vals), P50: idx(0.5), P90: idx(0.9), Avg: sum / len(vals), Max: vals[len(vals)-1]}
}

// Stats computes aggregates over all records.
func (s *Store) Stats(lowConf float64) Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := Stats{ByScenario: map[string]int{}, ByLanguage: map[string]int{}, ByPath: map[string]int{}, ByPolicy: map[string]int{}, Timings: map[string]Percentiles{}, TimingsByPath: map[string]Percentiles{}}
	st.Sessions = len(s.sessions)
	tim := map[string][]int{}
	byPath := map[string][]int{}
	for _, r := range s.records {
		st.Turns++
		for i, sc := range r.Scenarios {
			if i == 0 {
				st.ByScenario[sc]++
			}
		}
		st.ByLanguage[r.Language]++
		st.ByPath[r.Path]++
		st.ByPolicy[r.PolicyAction]++
		if r.Confidence < lowConf {
			st.LowConfidence++
			if len(st.Uncertain) < 50 {
				st.Uncertain = append(st.Uncertain, stripTrace(r))
			}
		}
		if r.Handoff {
			st.Handoffs++
		}
		if r.FastPathAgree != nil {
			st.FastPathChecked++
			if *r.FastPathAgree {
				st.FastPathAgree++
			}
		}
		for k, v := range r.Timings {
			if v > 0 {
				tim[k] = append(tim[k], v)
			}
		}
		if v := r.Timings["first_audio"]; v > 0 {
			byPath[r.Path] = append(byPath[r.Path], v)
		}
	}
	for k, v := range tim {
		st.Timings[k] = percentiles(v)
	}
	for k, v := range byPath {
		st.TimingsByPath[k] = percentiles(v)
	}
	n := 30
	if n > len(s.records) {
		n = len(s.records)
	}
	for _, r := range s.records[len(s.records)-n:] {
		st.Recent = append(st.Recent, stripTrace(r))
	}
	return st
}

func stripTrace(r Record) Record {
	r.Trace = nil
	return r
}
