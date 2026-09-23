package voicelog

import (
	"math"
	"sort"
	"sync"
)

// TurnMetrics captures everything worth knowing about a single
// conversational turn: what was said, which components handled it, and how
// long each stage took.
type TurnMetrics struct {
	Turn         int     `json:"turn"`
	Channel      string  `json:"channel"`
	UserText     string  `json:"user_text"`
	ReplyText    string  `json:"reply_text"`
	InLang       string  `json:"in_lang"`
	ReplyLang    string  `json:"reply_lang"`
	LangRule     string  `json:"lang_rule"`
	Scenario     string  `json:"scenario,omitempty"`
	Confidence   float64 `json:"confidence,omitempty"`
	Brain        string  `json:"brain"`
	Model        string  `json:"model,omitempty"`
	TTSModel     string  `json:"tts_model,omitempty"`
	Voice        string  `json:"voice,omitempty"`
	Speculative  bool    `json:"speculative"`
	Interrupted  bool    `json:"interrupted"`
	Filler       bool    `json:"filler"`
	STTms        float64 `json:"stt_ms"`        // speech end -> final transcript
	BrainTTFTms  float64 `json:"brain_ttft_ms"` // brain start -> first reply text
	BrainTotalms float64 `json:"brain_total_ms"`
	TTSTTFBms    float64 `json:"tts_ttfb_ms"`     // first text sent to TTS -> first audio
	EndToAudioms float64 `json:"end_to_audio_ms"` // speech end -> first audio played (filler included)
	EndToReplyms float64 `json:"end_to_reply_ms"` // speech end -> first audio of the actual reply (filler excluded)
	Totalms      float64 `json:"total_ms"`
}

// Quantiles summarizes a set of latency samples using the nearest-rank
// method.
type Quantiles struct {
	Count int     `json:"count"`
	P50   float64 `json:"p50"`
	P90   float64 `json:"p90"`
	P95   float64 `json:"p95"`
	Max   float64 `json:"max"`
}

// Snapshot is a point-in-time view over the turns currently held by Stats.
type Snapshot struct {
	Turns          int            `json:"turns"`
	EndToAudio     Quantiles      `json:"end_to_audio_ms"`
	STT            Quantiles      `json:"stt_ms"`
	BrainTTFT      Quantiles      `json:"brain_ttft_ms"`
	TTSTTFB        Quantiles      `json:"tts_ttfb_ms"`
	ShareUnder1500 float64        `json:"share_under_1500ms"` // share of turns with 0 < EndToAudio <= 1500
	Interrupted    int            `json:"interrupted"`
	Speculative    int            `json:"speculative_hits"`
	ByReplyLang    map[string]int `json:"by_reply_language"`
	ByChannel      map[string]int `json:"by_channel"`
	ByScenario     map[string]int `json:"by_scenario"`
	Recent         []TurnMetrics  `json:"recent"` // last 20, newest first
}

// Stats is a fixed-capacity ring of the most recent TurnMetrics, safe for
// concurrent use.
type Stats struct {
	mu    sync.Mutex
	max   int
	buf   []TurnMetrics
	start int
	count int
}

// NewStats creates a Stats that retains at most max turns (a non-positive
// max defaults to 1000).
func NewStats(max int) *Stats {
	if max <= 0 {
		max = 1000
	}
	return &Stats{max: max, buf: make([]TurnMetrics, max)}
}

// Add records m, evicting the oldest turn once the ring is full. A nil
// *Stats is a no-op.
func (s *Stats) Add(m TurnMetrics) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.count < s.max {
		idx := (s.start + s.count) % s.max
		s.buf[idx] = m
		s.count++
		return
	}
	s.buf[s.start] = m
	s.start = (s.start + 1) % s.max
}

// items returns the buffered turns ordered oldest to newest. Callers must
// hold s.mu.
func (s *Stats) items() []TurnMetrics {
	out := make([]TurnMetrics, s.count)
	for i := 0; i < s.count; i++ {
		out[i] = s.buf[(s.start+i)%s.max]
	}
	return out
}

// Snapshot computes rolling latency quantiles and breakdowns over the
// buffered turns. Quantiles use the nearest-rank method and ignore zero
// values (a zero duration means "not measured", not "instant"). A nil
// *Stats returns an empty (but non-nil-map) Snapshot.
func (s *Stats) Snapshot() Snapshot {
	snap := Snapshot{
		ByReplyLang: map[string]int{},
		ByChannel:   map[string]int{},
		ByScenario:  map[string]int{},
		Recent:      []TurnMetrics{},
	}
	if s == nil {
		return snap
	}

	s.mu.Lock()
	items := s.items()
	s.mu.Unlock()

	snap.Turns = len(items)

	var endToAudio, stt, brainTTFT, ttsTTFB []float64
	var under1500 int

	for _, m := range items {
		if m.EndToAudioms > 0 {
			endToAudio = append(endToAudio, m.EndToAudioms)
			if m.EndToAudioms <= 1500 {
				under1500++
			}
		}
		if m.STTms > 0 {
			stt = append(stt, m.STTms)
		}
		if m.BrainTTFTms > 0 {
			brainTTFT = append(brainTTFT, m.BrainTTFTms)
		}
		if m.TTSTTFBms > 0 {
			ttsTTFB = append(ttsTTFB, m.TTSTTFBms)
		}
		if m.Interrupted {
			snap.Interrupted++
		}
		if m.Speculative {
			snap.Speculative++
		}
		if m.ReplyLang != "" {
			snap.ByReplyLang[m.ReplyLang]++
		}
		if m.Channel != "" {
			snap.ByChannel[m.Channel]++
		}
		if m.Scenario != "" {
			snap.ByScenario[m.Scenario]++
		}
	}

	snap.EndToAudio = quantilesOf(endToAudio)
	snap.STT = quantilesOf(stt)
	snap.BrainTTFT = quantilesOf(brainTTFT)
	snap.TTSTTFB = quantilesOf(ttsTTFB)

	if len(items) > 0 {
		snap.ShareUnder1500 = float64(under1500) / float64(len(items))
	}

	n := len(items)
	recentCount := 20
	if n < recentCount {
		recentCount = n
	}
	snap.Recent = make([]TurnMetrics, recentCount)
	for i := 0; i < recentCount; i++ {
		snap.Recent[i] = items[n-1-i]
	}

	return snap
}

// quantilesOf computes Quantiles over vals using the nearest-rank method.
// vals need not be sorted and is not mutated.
func quantilesOf(vals []float64) Quantiles {
	if len(vals) == 0 {
		return Quantiles{}
	}
	sorted := append([]float64(nil), vals...)
	sort.Float64s(sorted)
	return Quantiles{
		Count: len(sorted),
		P50:   nearestRank(sorted, 50),
		P90:   nearestRank(sorted, 90),
		P95:   nearestRank(sorted, 95),
		Max:   sorted[len(sorted)-1],
	}
}

// nearestRank returns the pct-th percentile of sorted (ascending) using the
// nearest-rank method: rank = ceil(pct/100 * n), clamped to [1, n].
func nearestRank(sorted []float64, pct float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	rank := int(math.Ceil(pct / 100 * float64(n)))
	if rank < 1 {
		rank = 1
	}
	if rank > n {
		rank = n
	}
	return sorted[rank-1]
}
