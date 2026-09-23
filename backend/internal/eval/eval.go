// Package eval runs the labeled dev set (data/dev_utterances.json) through
// the router and computes the same metrics as the kit's evaluate.py, plus
// routing latency percentiles. Results power the Eval page and the README.
package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Utterance is one labeled dev-set item.
type Utterance struct {
	ID       string   `json:"id"`
	Text     string   `json:"text"`
	Lang     string   `json:"lang"`
	Expected []string `json:"expected"`
	Type     string   `json:"type"`
}

// Outcome is what the router returned for one utterance.
type Outcome struct {
	IDs        []string `json:"ids"`
	Confidence float64  `json:"confidence"`
	Path       string   `json:"path"`
	RouteMs    int      `json:"route_ms"`
	Reason     string   `json:"reason,omitempty"`
	Error      string   `json:"error,omitempty"`
}

// RouteFunc routes one text statelessly.
type RouteFunc func(ctx context.Context, text string) Outcome

type Row struct {
	Utterance
	Got        []string `json:"got"`
	Primary    bool     `json:"primary_ok"`
	Full       bool     `json:"full_ok"`
	Confidence float64  `json:"confidence"`
	Path       string   `json:"path"`
	RouteMs    int      `json:"route_ms"`
	Reason     string   `json:"reason,omitempty"`
	Error      string   `json:"error,omitempty"`
}

type Group struct {
	N          int     `json:"n"`
	Primary    int     `json:"primary"`
	Full       int     `json:"full"`
	PrimaryAcc float64 `json:"primary_acc"`
	FullMatch  float64 `json:"full_match"`
}

// Report mirrors evaluate.py's output.
type Report struct {
	RanAt        time.Time           `json:"ran_at"`
	Router       string              `json:"router"`
	N            int                 `json:"n"`
	Groups       map[string]Group    `json:"groups"`
	IntentRecall float64             `json:"intent_recall"`
	LatencyP50   int                 `json:"latency_p50_ms"`
	LatencyP90   int                 `json:"latency_p90_ms"`
	LatencyMax   int                 `json:"latency_max_ms"`
	ByPath       map[string]int      `json:"by_path"`
	Rows         []Row               `json:"rows"`
	Predictions  map[string][]string `json:"predictions"`
	DurationMs   int                 `json:"duration_ms"`
}

// LoadDevSet reads dev_utterances.json.
func LoadDevSet(dataDir string) ([]Utterance, error) {
	b, err := os.ReadFile(filepath.Join(dataDir, "dev_utterances.json"))
	if err != nil {
		return nil, err
	}
	var f struct {
		Utterances []Utterance `json:"utterances"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	return f.Utterances, nil
}

// Run evaluates all utterances with the given concurrency.
func Run(ctx context.Context, utts []Utterance, route RouteFunc, routerName string, concurrency int, progress func(done, total int)) *Report {
	start := time.Now()
	rows := make([]Row, len(utts))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0
	for i, u := range utts {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, u Utterance) {
			defer wg.Done()
			defer func() { <-sem }()
			o := route(ctx, u.Text)
			r := Row{Utterance: u, Got: o.IDs, Confidence: o.Confidence, Path: o.Path, RouteMs: o.RouteMs, Reason: o.Reason, Error: o.Error}
			r.Primary = len(o.IDs) > 0 && len(u.Expected) > 0 && o.IDs[0] == u.Expected[0]
			r.Full = sameSet(o.IDs, u.Expected)
			rows[i] = r
			mu.Lock()
			done++
			if progress != nil {
				progress(done, len(utts))
			}
			mu.Unlock()
		}(i, u)
	}
	wg.Wait()
	rep := &Report{RanAt: time.Now(), Router: routerName, N: len(utts), Groups: map[string]Group{}, ByPath: map[string]int{}, Rows: rows, Predictions: map[string][]string{}}
	hit, total := 0, 0
	var lat []int
	for _, r := range rows {
		rep.Predictions[r.ID] = r.Got
		if r.Got == nil {
			rep.Predictions[r.ID] = []string{}
		}
		for _, key := range []string{"all", "lang=" + r.Lang, "type=" + r.Type} {
			g := rep.Groups[key]
			g.N++
			if r.Primary {
				g.Primary++
			}
			if r.Full {
				g.Full++
			}
			rep.Groups[key] = g
		}
		if r.Type == "multi_intent" {
			total += len(r.Expected)
			hit += intersect(r.Expected, r.Got)
		}
		rep.ByPath[r.Path]++
		if r.RouteMs > 0 {
			lat = append(lat, r.RouteMs)
		}
	}
	for k, g := range rep.Groups {
		if g.N > 0 {
			g.PrimaryAcc = round3(float64(g.Primary) / float64(g.N))
			g.FullMatch = round3(float64(g.Full) / float64(g.N))
		}
		rep.Groups[k] = g
	}
	if total > 0 {
		rep.IntentRecall = round3(float64(hit) / float64(total))
	}
	if len(lat) > 0 {
		sort.Ints(lat)
		rep.LatencyP50 = lat[len(lat)/2]
		rep.LatencyP90 = lat[(len(lat)*9)/10]
		rep.LatencyMax = lat[len(lat)-1]
	}
	rep.DurationMs = int(time.Since(start).Milliseconds())
	return rep
}

// Summary renders the report like evaluate.py.
func (r *Report) Summary() string {
	keys := []string{"all"}
	var langs, types []string
	for k := range r.Groups {
		if len(k) > 5 && k[:5] == "lang=" {
			langs = append(langs, k)
		} else if len(k) > 5 && k[:5] == "type=" {
			types = append(types, k)
		}
	}
	sort.Strings(langs)
	sort.Strings(types)
	keys = append(keys, langs...)
	keys = append(keys, types...)
	out := fmt.Sprintf("%-22s%5s%14s%12s\n", "group", "n", "primary_acc", "full_match")
	for _, k := range keys {
		g := r.Groups[k]
		out += fmt.Sprintf("%-22s%5d%14.3f%12.3f\n", k, g.N, g.PrimaryAcc, g.FullMatch)
	}
	out += fmt.Sprintf("\nintent_recall (multi-intent): %.3f\nroute latency p50/p90/max: %d/%d/%d ms\n", r.IntentRecall, r.LatencyP50, r.LatencyP90, r.LatencyMax)
	return out
}

func sameSet(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	m := map[string]bool{}
	for _, x := range a {
		m[x] = true
	}
	n := map[string]bool{}
	for _, x := range b {
		n[x] = true
	}
	if len(m) != len(n) {
		return false
	}
	for k := range m {
		if !n[k] {
			return false
		}
	}
	return true
}

func intersect(a, b []string) int {
	m := map[string]bool{}
	for _, x := range a {
		m[x] = true
	}
	c := 0
	seen := map[string]bool{}
	for _, x := range b {
		if m[x] && !seen[x] {
			c++
			seen[x] = true
		}
	}
	return c
}

func round3(f float64) float64 { return float64(int(f*1000+0.5)) / 1000 }
