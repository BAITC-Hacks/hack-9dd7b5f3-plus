package dialog

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"
	"unicode"

	"hackathon/backend/internal/events"
	"hackathon/backend/internal/tts"
)

// speaker turns a streamed reply into sentence-sized TTS requests and pushes
// the PCM frames to the session's audio sink as they arrive.
type speaker struct {
	e          *Engine
	s          *Session
	ctx        context.Context
	lang       string
	turn       int
	t0         time.Time
	buf        strings.Builder
	queue      chan string
	wg         sync.WaitGroup
	mu         sync.Mutex
	first      time.Time // first audio byte sent
	ttsFirst   int       // ms from first synth request to first byte
	sentences  int
	collect    []byte
	collecting bool
	errs       []string
	text       strings.Builder
}

func (e *Engine) newSpeaker(ctx context.Context, s *Session, lang string, turn int, t0 time.Time, collect bool) *speaker {
	sp := &speaker{e: e, s: s, ctx: ctx, lang: lang, turn: turn, t0: t0, queue: make(chan string, 16), collecting: collect}
	sp.wg.Add(1)
	go sp.run()
	return sp
}

// Feed adds reply text; complete sentences are synthesized immediately.
func (sp *speaker) Feed(delta string) {
	sp.text.WriteString(delta)
	sp.buf.WriteString(delta)
	for {
		cur := sp.buf.String()
		cut := sentenceCut(cur)
		if cut < 0 {
			return
		}
		sentence := strings.TrimSpace(cur[:cut])
		rest := cur[cut:]
		sp.buf.Reset()
		sp.buf.WriteString(rest)
		if sentence != "" {
			sp.enqueue(sentence)
		}
	}
}

// sentenceCut returns the byte index after the first complete sentence, or
// -1 when the text has no sentence boundary yet. Long fragments are cut at a
// comma so the first audio does not wait for a long sentence.
func sentenceCut(s string) int {
	runes := []rune(s)
	for i, r := range runes {
		if r == '.' || r == '!' || r == '?' || r == '…' || r == '\n' {
			if i+1 < len(runes) && !unicode.IsSpace(runes[i+1]) && r != '\n' {
				continue // "8.5" or "CL-500" style
			}
			if r == '.' && i >= 1 && unicode.IsDigit(runes[i-1]) && i+1 < len(runes) {
				// "с 10 по 16 октября." is fine (space after); numbers with dots inside stay
			}
			if i < 2 {
				continue
			}
			return len(string(runes[:i+1]))
		}
	}
	if len(runes) > 160 {
		for i := len(runes) - 1; i > 60; i-- {
			if runes[i] == ',' || runes[i] == ';' || runes[i] == ':' {
				return len(string(runes[:i+1]))
			}
		}
	}
	return -1
}

func (sp *speaker) enqueue(sentence string) {
	sp.sentences++
	select {
	case sp.queue <- sentence:
	case <-sp.ctx.Done():
	}
}

// Flush speaks whatever is left and waits for playback to be sent.
func (sp *speaker) Flush() {
	if rest := strings.TrimSpace(sp.buf.String()); rest != "" {
		sp.enqueue(rest)
		sp.buf.Reset()
	}
	close(sp.queue)
	done := make(chan struct{})
	go func() { sp.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(25 * time.Second):
		sp.errs = append(sp.errs, "tts: timed out waiting for synthesis")
	case <-sp.ctx.Done():
	}
}

func (sp *speaker) run() {
	defer sp.wg.Done()
	for sentence := range sp.queue {
		if sp.ctx.Err() != nil {
			return
		}
		sp.speak(sentence)
	}
	sp.e.emit(events.Event{Type: "audio_end", SessionID: sp.s.ID, Turn: sp.turn})
}

func (sp *speaker) speak(sentence string) {
	e := sp.e
	if e.tts == nil {
		return
	}
	if _, isBrowser := e.tts.(tts.Browser); isBrowser {
		// keyless mode: the UI speaks the sentence with window.speechSynthesis
		sp.mu.Lock()
		if sp.first.IsZero() {
			sp.first = time.Now()
		}
		sp.mu.Unlock()
		e.emit(events.Event{Type: "speak", SessionID: sp.s.ID, Turn: sp.turn, Data: map[string]any{"text": sentence, "lang": sp.lang}})
		return
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(sp.ctx, 20*time.Second)
	defer cancel()
	rc, err := e.tts.Synthesize(ctx, sentence, sp.lang)
	if err != nil {
		sp.errs = append(sp.errs, "tts: "+err.Error())
		e.emit(events.Event{Type: "tts_error", SessionID: sp.s.ID, Turn: sp.turn, Data: map[string]any{"error": err.Error(), "text": sentence}})
		// degrade gracefully: let the browser speak it
		e.emit(events.Event{Type: "speak", SessionID: sp.s.ID, Turn: sp.turn, Data: map[string]any{"text": sentence, "lang": sp.lang, "fallback": true}})
		return
	}
	defer rc.Close()
	sink := sp.s.sink()
	buf := make([]byte, 4800) // 100 ms at 24 kHz
	total := 0
	for {
		n, rerr := rc.Read(buf)
		if n > 0 {
			frame := make([]byte, n)
			copy(frame, buf[:n])
			sp.mu.Lock()
			if sp.first.IsZero() {
				sp.first = time.Now()
				sp.ttsFirst = int(time.Since(start).Milliseconds())
				e.emit(events.Event{Type: "tts_first_byte", SessionID: sp.s.ID, Turn: sp.turn, Data: map[string]any{
					"tts_ms": sp.ttsFirst, "first_audio_ms": int(sp.first.Sub(sp.t0).Milliseconds()), "sentence": sentence}})
			}
			if sp.collecting {
				sp.collect = append(sp.collect, frame...)
			}
			sp.mu.Unlock()
			if sink != nil {
				sink(frame)
			}
			total += n
		}
		if rerr != nil {
			if rerr != io.EOF && sp.ctx.Err() == nil {
				sp.errs = append(sp.errs, "tts stream: "+rerr.Error())
			}
			break
		}
	}
	e.emit(events.Event{Type: "tts_sentence", SessionID: sp.s.ID, Turn: sp.turn, Data: map[string]any{"text": sentence, "bytes": total, "ms": int(time.Since(start).Milliseconds())}})
}
