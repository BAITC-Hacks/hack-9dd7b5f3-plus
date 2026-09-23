// Package agent is the real-time conversation engine shared by every channel
// (browser, Asterisk/KZ phone number, Twilio). Per call it keeps one streaming
// STT session open, decides the reply language, asks the brain for a routed
// reply (streamed), speaks it through streaming TTS and measures every stage.
//
// Latency tricks, all in this file:
//   - STT session opened once per call; audio buffered until it is ready
//   - TTS socket pre-opened on the first partial of an utterance
//   - speculative brain run on a stable partial (stateless brains only): the
//     reply is generated during the VAD silence window and released when the
//     final transcript matches
//   - first TTS piece flushed at the first comma
//   - barge-in: caller speech cancels the reply and clears queued audio
//   - cached filler ("Секунду.") if nothing is audible after VOICE_FILLER_AFTER_MS
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"hackathon/voice/audio"
	"hackathon/voice/brain"
	"hackathon/voice/config"
	"hackathon/voice/elevenlabs"
	"hackathon/voice/lang"
	"hackathon/voice/speech"
	"hackathon/voice/voicelog"
)

// Transport is a channel adapter (browser WebSocket, Asterisk AudioSocket, Twilio).
type Transport interface {
	// OutputFormat is the ElevenLabs format the channel plays (pcm_16000, pcm_8000, ulaw_8000).
	OutputFormat() string
	// Play queues audio for playback; it must not block for long.
	Play(audio []byte) error
	// Clear drops queued audio and stops playback now (barge-in).
	Clear()
	// Send delivers a UI event (browser); phone transports may ignore it.
	Send(ev map[string]any)
	// Hangup ends the call from the agent side (after a goodbye).
	Hangup()
}

// Deps are shared, long-lived services.
type Deps struct {
	Cfg   config.Config
	STT   speech.Recognizer
	TTS   speech.Synthesizer
	Brain brain.Brain
	Log   *voicelog.Logger
}

// Options describe one call.
type Options struct {
	SessionID   string
	Channel     string // web | phone | twilio | demo
	CallerID    string
	InputFormat string // pcm_16000 | pcm_8000 | ulaw_8000
	Manual      bool   // push-to-talk: the client sends Commit
	Greeting    bool
	Lang        lang.Lang // initial reply language (optional)
	Meta        map[string]any
}

// Greeting is bilingual so the caller's first answer picks the language.
const Greeting = "Сәлеметсіз бе! Здравствуйте! Я голосовой помощник Saqta. Чем могу помочь?"

// FillerText is played when a reply is late.
func FillerText(l lang.Lang) string {
	if l == lang.KK {
		return "Бір сәт."
	}
	return "Секунду."
}

// Agent runs one call.
type Agent struct {
	deps Deps
	opt  Options
	tr   Transport
	in   audio.Format
	out  audio.Format

	ctx    context.Context
	cancel context.CancelFunc
	log    *voicelog.Session

	recMu   sync.Mutex
	rec     speech.RecognizerSession
	pending [][]byte

	mu          sync.Mutex
	policy      lang.Policy
	vad         audio.EnergyVAD
	turn        int
	utterStart  time.Time
	lastPartial string
	commitAt    time.Time
	specTimer   *time.Timer
	spec        *run
	active      *run
	playUntil   time.Time
	warm        *warmStream
	lastCommit  string
	lastCommitT time.Time
	closed      bool
}

type warmStream struct {
	lang   lang.Lang
	stream speech.Stream
	at     time.Time
}

// New prepares an agent; call Run to start it. PushAudio may be called right
// away (audio is buffered until the STT session is ready).
func New(ctx context.Context, deps Deps, opt Options, tr Transport) *Agent {
	if opt.InputFormat == "" {
		opt.InputFormat = "pcm_16000"
	}
	in, err := audio.ParseFormat(opt.InputFormat)
	if err != nil {
		in = audio.MustFormat("pcm_16000")
	}
	out, err := audio.ParseFormat(tr.OutputFormat())
	if err != nil {
		out = audio.MustFormat("pcm_16000")
	}
	a := &Agent{deps: deps, opt: opt, tr: tr, in: in, out: out}
	a.ctx, a.cancel = context.WithCancel(ctx)
	a.policy.Current = opt.Lang
	a.vad = audio.EnergyVAD{MinSpeech: 250 * time.Millisecond}
	meta := map[string]any{
		"caller": opt.CallerID, "input_format": in.Name, "output_format": out.Name,
		"manual": opt.Manual, "brain": deps.Brain.Name(),
	}
	for k, v := range opt.Meta {
		meta[k] = v
	}
	a.log = deps.Log.Open(opt.SessionID, opt.Channel, meta)
	return a
}

// Run blocks until the call ends (Close, context cancel, or STT failure).
func (a *Agent) Run() error {
	defer a.shutdown()
	if a.opt.Greeting {
		go a.greet()
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if a.ctx.Err() != nil {
			return nil
		}
		rec, err := a.deps.STT.Open(a.ctx, speech.RecognizerOptions{Format: a.in.Name, Manual: a.opt.Manual})
		if err != nil {
			lastErr = err
			a.logErr("stt.open", err)
			a.tr.Send(map[string]any{"type": "error", "message": "speech recognition unavailable: " + err.Error()})
			select {
			case <-time.After(time.Duration(attempt+1) * 400 * time.Millisecond):
			case <-a.ctx.Done():
				return nil
			}
			continue
		}
		a.setRecognizer(rec)
		if attempt == 0 {
			a.tr.Send(map[string]any{"type": "session.ready", "session_id": a.opt.SessionID, "output_format": a.out.Name,
				"sample_rate": a.out.Rate, "brain": a.deps.Brain.Name(), "manual": a.opt.Manual})
		}
		lastErr = a.consume(rec)
		a.clearRecognizer()
		rec.Close()
		if a.ctx.Err() != nil {
			return nil
		}
		a.logErr("stt.closed", lastErr)
	}
	return lastErr
}

// Close ends the call.
func (a *Agent) Close() { a.cancel() }

// Done is closed when the call context ends.
func (a *Agent) Done() <-chan struct{} { return a.ctx.Done() }

func (a *Agent) consume(rec speech.RecognizerSession) error {
	for {
		select {
		case <-a.ctx.Done():
			return nil
		case ev, ok := <-rec.Events():
			if !ok {
				return errors.New("stt stream closed")
			}
			switch ev.Type {
			case elevenlabs.EventPartial:
				a.onPartial(ev)
			case elevenlabs.EventCommitted, elevenlabs.EventCommittedTimestamps:
				a.onCommitted(ev)
			case elevenlabs.EventError:
				a.logErr("stt", ev.Err)
			case elevenlabs.EventWarning:
				a.log.Log("stt.warning", map[string]any{"warning": ev.Text})
			case elevenlabs.EventClosed:
				if ev.Err != nil {
					return ev.Err
				}
				return errors.New("stt closed")
			}
		}
	}
}

func (a *Agent) setRecognizer(rec speech.RecognizerSession) {
	a.recMu.Lock()
	defer a.recMu.Unlock()
	a.rec = rec
	for _, p := range a.pending {
		_ = rec.Send(p)
	}
	a.pending = nil
}

func (a *Agent) clearRecognizer() {
	a.recMu.Lock()
	a.rec = nil
	a.recMu.Unlock()
}

func (a *Agent) recognizer() speech.RecognizerSession {
	a.recMu.Lock()
	defer a.recMu.Unlock()
	return a.rec
}

// PushAudio feeds caller audio in the call's input format.
func (a *Agent) PushAudio(frame []byte) {
	if len(frame) == 0 || a.ctx.Err() != nil {
		return
	}
	a.recMu.Lock()
	if a.rec == nil {
		if len(a.pending) < 250 { // ~5 s of 20 ms frames
			a.pending = append(a.pending, append([]byte(nil), frame...))
		}
	} else {
		_ = a.rec.Send(frame)
	}
	a.recMu.Unlock()
	a.log.RecordIn(frame, a.in)

	if a.deps.Cfg.BargeIn && a.deps.Cfg.BargeInVAD {
		samples := audio.ToPCM16(frame, a.in)
		a.mu.Lock()
		onset := a.vad.Push(samples, a.in.Rate)
		speaking := a.speakingLocked()
		a.mu.Unlock()
		if onset && speaking {
			a.bargeIn("vad")
		}
	}
}

// Commit ends the utterance now (push-to-talk release).
func (a *Agent) Commit() {
	a.mu.Lock()
	a.commitAt = time.Now()
	a.mu.Unlock()
	if rec := a.recognizer(); rec != nil {
		if err := rec.Commit(); err != nil {
			a.logErr("stt.commit", err)
		}
	}
	a.log.Log("stt.commit", nil)
}

// SubmitText runs a typed utterance through the same pipeline (text fallback).
func (a *Agent) SubmitText(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	a.mu.Lock()
	choice := a.policy.Choose(text, "")
	a.mu.Unlock()
	a.tr.Send(map[string]any{"type": "stt.final", "text": text, "language": langOr(choice.Detected, choice.Reply), "ms": 0, "source": "text"})
	a.log.Log("text.input", map[string]any{"text": text, "reply_language": choice.Reply, "lang_rule": choice.Rule})
	a.startRun(text, choice, time.Now(), 0, false)
}

// Interrupt stops the current reply (client stop button).
func (a *Agent) Interrupt() { a.bargeIn("client") }

// ClientMetrics records latency measured in the browser (e.g. release -> sound).
func (a *Agent) ClientMetrics(fields map[string]any) { a.log.Log("client.metrics", fields) }

func (a *Agent) onPartial(ev elevenlabs.STTEvent) {
	text := strings.TrimSpace(ev.Text)
	if text == "" {
		return
	}
	a.mu.Lock()
	first := a.utterStart.IsZero()
	if first {
		a.utterStart = ev.At
	}
	changed := text != a.lastPartial
	a.lastPartial = text
	speaking := a.speakingLocked()
	a.mu.Unlock()
	if !changed {
		return
	}
	a.tr.Send(map[string]any{"type": "stt.partial", "text": text})
	a.log.Log("stt.partial", map[string]any{"text": text})
	if first {
		go a.prewarm()
	}
	if speaking && a.deps.Cfg.BargeIn && wordCount(text) >= 2 {
		a.bargeIn("speech")
	}
	if a.deps.Cfg.Speculative && !a.opt.Manual && a.deps.Brain.Stateless() {
		a.scheduleSpeculation(text)
	}
}

func (a *Agent) onCommitted(ev elevenlabs.STTEvent) {
	text := strings.TrimSpace(ev.Text)
	now := time.Now()
	a.mu.Lock()
	// the timestamped variant follows the plain one for the same segment: we
	// already acted on the plain one, now refine the real end of speech
	if text != "" && text == a.lastCommit && now.Sub(a.lastCommitT) < 3*time.Second {
		r := a.active
		if a.spec != nil {
			r = nil
		}
		a.mu.Unlock()
		if ev.Type == elevenlabs.EventCommittedTimestamps && r != nil {
			a.refineSpeechEnd(r, ev.Words, now)
		}
		return
	}
	if text == "" {
		a.utterStart, a.lastPartial = time.Time{}, ""
		a.mu.Unlock()
		return
	}
	a.lastCommit, a.lastCommitT = text, now
	utterStart := a.utterStart
	a.utterStart, a.lastPartial = time.Time{}, ""
	if a.specTimer != nil {
		a.specTimer.Stop()
		a.specTimer = nil
	}
	spec := a.spec
	a.spec = nil
	commitAt := a.commitAt
	a.commitAt = time.Time{}
	// a reply that has not started speaking yet is merged with the new segment
	// (the caller only paused), so the brain sees the whole thought
	merged := false
	if r := a.active; r != nil && !r.hasAudio() && now.Sub(r.created) < 6*time.Second {
		text = r.text + " " + text
		merged = true
	}
	choice := a.policy.Choose(text, ev.Language)
	a.mu.Unlock()

	var speechEnd time.Time
	var ok bool
	if rec := a.recognizer(); rec != nil {
		speechEnd, ok = rec.SpeechEnd(ev.Words)
	}
	if !ok {
		if !commitAt.IsZero() {
			speechEnd = commitAt
		} else {
			speechEnd = now.Add(-time.Duration(a.deps.Cfg.VADSilenceSecs * float64(time.Second)))
		}
	}
	if speechEnd.After(now) {
		speechEnd = now
	}
	sttMS := msBetween(speechEnd, now)
	a.tr.Send(map[string]any{"type": "stt.final", "text": text, "language": langOr(choice.Detected, choice.Reply), "ms": round(sttMS)})
	fields := map[string]any{"text": text, "stt_language": ev.Language, "detected": choice.Detected, "reply_language": choice.Reply,
		"lang_rule": choice.Rule, "kk_share": round(choice.Mix.Share * 100), "stt_ms": round(sttMS), "merged": merged}
	if !utterStart.IsZero() {
		fields["utterance_ms"] = round(msBetween(utterStart, speechEnd))
	}
	a.log.Log("stt.final", fields)

	if spec != nil {
		if specMatches(spec.norm, normalize(text)) && spec.lang == choice.Reply && spec.ctx.Err() == nil {
			a.promote(spec, speechEnd, sttMS, choice)
			return
		}
		spec.cancel()
		a.log.Log("speculation.miss", map[string]any{"speculated": spec.text, "final": text})
	}
	a.startRun(text, choice, speechEnd, sttMS, false)
}

// refineSpeechEnd replaces the estimated end of speech (commit time minus the
// VAD window) with the end of the last recognised word, so the reported
// "end of speech -> first audio" latency is measured from the real moment.
func (a *Agent) refineSpeechEnd(r *run, words []elevenlabs.Word, at time.Time) {
	rec := a.recognizer()
	if rec == nil {
		return
	}
	end, ok := rec.SpeechEnd(words)
	if !ok || end.After(at) {
		return
	}
	r.mu.Lock()
	if !r.speechEnd.IsZero() && r.firstOut.IsZero() {
		r.speechEnd = end
		r.sttMS = msBetween(end, r.committedAt)
	}
	r.mu.Unlock()
}

func (a *Agent) scheduleSpeculation(text string) {
	after := a.deps.Cfg.SpeculateAfter
	if after <= 0 {
		after = 250 * time.Millisecond
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.specTimer != nil {
		a.specTimer.Stop()
	}
	a.specTimer = time.AfterFunc(after, func() { a.speculate(text) })
}

func (a *Agent) speculate(text string) {
	if len([]rune(text)) < 6 {
		return
	}
	a.mu.Lock()
	if a.closed || a.lastPartial != text || (a.spec != nil && a.spec.norm == normalize(text)) {
		a.mu.Unlock()
		return
	}
	if a.active != nil && !a.active.hasAudio() {
		// a real reply is still being prepared; do not race it
		a.mu.Unlock()
		return
	}
	choice := a.policy.Peek(text, "")
	a.mu.Unlock()
	a.startRun(text, choice, time.Time{}, 0, true)
}

// run is one reply attempt.
type run struct {
	text    string
	norm    string
	lang    lang.Lang
	choice  lang.Choice
	created time.Time

	ctx         context.Context
	cancel      context.CancelFunc
	gate        *gate
	promoted    chan struct{}
	interrupted atomic.Bool

	mu             sync.Mutex
	turnNo         int
	speculative    bool // started on a partial transcript
	speechEnd      time.Time
	committedAt    time.Time
	sttMS          float64
	stream         speech.Stream
	ttsAllowed     bool
	pendingPieces  []speech.Piece
	reply          strings.Builder
	decision       *brain.Decision
	result         brain.Result
	brainStart     time.Time
	brainFirst     time.Time
	brainDone      time.Time
	ttsFirstText   time.Time
	ttsFirstAudio  time.Time
	firstOut       time.Time
	firstReply     time.Time // first audio of the reply itself (filler excluded)
	fillerNext     bool      // the next played chunk is the filler
	filler         bool
	promotedAt     time.Time
	streamInfoSeen elevenlabs.SocketInfo
}

func (r *run) hasAudio() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return !r.firstOut.IsZero()
}

func (a *Agent) startRun(text string, choice lang.Choice, speechEnd time.Time, sttMS float64, speculative bool) {
	r := &run{
		text: text, norm: normalize(text), lang: choice.Reply, choice: choice, created: time.Now(),
		promoted: make(chan struct{}), speculative: speculative, speechEnd: speechEnd, sttMS: sttMS,
	}
	if !speculative {
		r.committedAt = r.created
	}
	r.ctx, r.cancel = context.WithCancel(a.ctx)
	r.gate = &gate{open: !speculative, play: func(b []byte) { a.play(r, b) }, send: a.tr.Send}
	a.mu.Lock()
	if speculative {
		if a.spec != nil {
			a.spec.cancel()
		}
		a.spec = r
		r.ttsAllowed = a.deps.Cfg.SpeculativeTTS
	} else {
		a.turn++
		r.turnNo = a.turn
		if a.active != nil {
			a.active.interrupted.Store(true)
			a.active.cancel()
		}
		a.active = r
		r.ttsAllowed = true
		close(r.promoted)
	}
	a.mu.Unlock()
	go a.execute(r)
}

func (a *Agent) promote(r *run, speechEnd time.Time, sttMS float64, choice lang.Choice) {
	a.mu.Lock()
	a.turn++
	turn := a.turn
	if a.active != nil && a.active != r {
		a.active.interrupted.Store(true)
		a.active.cancel()
	}
	a.active = r
	a.mu.Unlock()

	r.mu.Lock()
	r.turnNo, r.speechEnd, r.sttMS, r.choice, r.promotedAt = turn, speechEnd, sttMS, choice, time.Now()
	r.committedAt = r.promotedAt
	head := msBetween(r.brainStart, r.promotedAt)
	r.mu.Unlock()
	close(r.promoted)
	r.gate.Open()
	r.flushPieces()
	a.log.Log("speculation.hit", map[string]any{"turn": turn, "text": r.text, "head_start_ms": round(head)})
}

func (r *run) speak(p speech.Piece) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.ttsAllowed || r.stream == nil {
		r.pendingPieces = append(r.pendingPieces, p)
		return
	}
	if r.ttsFirstText.IsZero() {
		r.ttsFirstText = time.Now()
	}
	_ = r.stream.Send(p.Text, p.Flush)
}

func (r *run) flushPieces() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ttsAllowed = true
	if r.stream == nil {
		return
	}
	for _, p := range r.pendingPieces {
		if r.ttsFirstText.IsZero() {
			r.ttsFirstText = time.Now()
		}
		_ = r.stream.Send(p.Text, p.Flush)
	}
	r.pendingPieces = nil
}

func (a *Agent) execute(r *run) {
	defer r.cancel()
	stream, err := a.openStream(r)
	if err != nil {
		if r.ctx.Err() == nil {
			a.logErr("tts.open", err)
			a.tr.Send(map[string]any{"type": "error", "message": "speech synthesis unavailable: " + err.Error()})
		}
		return
	}
	defer stream.Close()
	r.mu.Lock()
	r.stream = stream
	r.mu.Unlock()
	r.flushIfAllowed()

	pumpDone := make(chan struct{})
	go func() {
		defer close(pumpDone)
		for chunk := range stream.Audio() {
			if r.ctx.Err() != nil {
				return
			}
			r.mu.Lock()
			if r.ttsFirstAudio.IsZero() {
				r.ttsFirstAudio = time.Now()
			}
			r.mu.Unlock()
			r.gate.Audio(chunk)
		}
	}()
	go a.fillerWatch(r)

	chunker := speech.NewChunker()
	relay := a.deps.Brain.Name() == "backend" // backend events are relayed as-is; do not duplicate them
	r.mu.Lock()
	r.brainStart = time.Now()
	speechEnd, sttMS, speculative := r.speechEnd, r.sttMS, r.speculative
	r.mu.Unlock()
	res, err := a.deps.Brain.Turn(r.ctx, brain.Input{
		SessionID: a.opt.SessionID, Channel: a.opt.Channel, CallerID: a.opt.CallerID,
		Text: r.text, Lang: string(r.lang), SpeechEnd: speechEnd, STTms: sttMS, Speculative: speculative,
	}, func(ev brain.Event) {
		switch ev.Kind {
		case brain.KindDecision:
			if ev.Decision == nil {
				return
			}
			r.mu.Lock()
			r.decision = ev.Decision
			ms := msBetween(r.brainStart, time.Now())
			r.mu.Unlock()
			if !relay {
				r.gate.Event(map[string]any{"type": "router.decision", "decision": contractDecision(ev.Decision, r.choice, res0(a)), "ms": round(ms)})
			}
			a.log.Log("router.decision", map[string]any{"scenario": ev.Decision.ScenarioID, "confidence": ev.Decision.Confidence,
				"status": ev.Decision.Status, "speculative": speculative, "ms": round(ms)})
		case brain.KindDelta:
			if ev.Text == "" {
				return
			}
			r.mu.Lock()
			if r.brainFirst.IsZero() {
				r.brainFirst = time.Now()
			}
			r.reply.WriteString(ev.Text)
			r.mu.Unlock()
			if !relay {
				r.gate.Event(map[string]any{"type": "response.delta", "text": ev.Text})
			}
			for _, p := range chunker.Push(ev.Text) {
				r.speak(p)
			}
		case brain.KindRaw:
			var m map[string]any
			if json.Unmarshal(ev.Raw, &m) == nil {
				r.gate.Event(m)
			}
		}
	})
	for _, p := range chunker.Flush() {
		r.speak(p)
	}
	r.mu.Lock()
	r.result, r.brainDone = res, time.Now()
	empty := strings.TrimSpace(r.reply.String()) == ""
	r.mu.Unlock()
	if err != nil && r.ctx.Err() == nil {
		a.logErr("brain", err)
		if empty {
			fb := fallbackReply(r.lang)
			r.mu.Lock()
			r.reply.WriteString(fb)
			r.mu.Unlock()
			r.speak(speech.Piece{Text: fb, Flush: true})
		}
	}

	// a speculative reply is only spoken once the final transcript confirms it
	select {
	case <-r.promoted:
	case <-r.ctx.Done():
		return
	}
	if r.ctx.Err() != nil {
		a.finishInterrupted(r, stream)
		return
	}
	r.flushPieces()
	_ = stream.Finish()
	select {
	case <-pumpDone:
	case <-r.ctx.Done():
	}
	if r.ctx.Err() != nil {
		a.finishInterrupted(r, stream)
		return
	}
	if err := stream.Err(); err != nil {
		a.logErr("tts", err)
	}
	r.mu.Lock()
	r.streamInfoSeen = stream.Info()
	r.mu.Unlock()
	a.finish(r)
}

// finishInterrupted logs a cancelled reply as an interrupted turn only if the
// caller heard part of it (barge-in); replies superseded before any audio
// (merged utterances, missed speculation) leave no trace in history.
func (a *Agent) finishInterrupted(r *run, stream speech.Stream) {
	if !r.hasAudio() {
		return
	}
	r.mu.Lock()
	r.streamInfoSeen = stream.Info()
	r.mu.Unlock()
	a.finish(r)
}

func (r *run) flushIfAllowed() {
	r.mu.Lock()
	allowed := r.ttsAllowed
	r.mu.Unlock()
	if allowed {
		r.flushPieces()
	}
}

func res0(a *Agent) string { return a.deps.Brain.Name() }

func (a *Agent) openStream(r *run) (speech.Stream, error) {
	if s := a.takeWarm(r.lang); s != nil {
		return s, nil
	}
	return a.deps.TTS.Open(r.ctx, r.lang, a.out.Name)
}

func (a *Agent) prewarm() {
	a.mu.Lock()
	l := a.policy.Current
	if l == lang.Unknown {
		l = lang.RU
	}
	if a.closed || (a.warm != nil && a.warm.lang == l && time.Since(a.warm.at) < 12*time.Second) {
		a.mu.Unlock()
		return
	}
	old := a.warm
	a.warm = nil
	a.mu.Unlock()
	if old != nil {
		old.stream.Close()
	}
	s, err := a.deps.TTS.Open(a.ctx, l, a.out.Name)
	if err != nil {
		a.logErr("tts.prewarm", err)
		return
	}
	a.mu.Lock()
	if a.warm != nil || a.closed {
		a.mu.Unlock()
		s.Close()
		return
	}
	a.warm = &warmStream{lang: l, stream: s, at: time.Now()}
	a.mu.Unlock()
}

func (a *Agent) takeWarm(l lang.Lang) speech.Stream {
	a.mu.Lock()
	w := a.warm
	a.warm = nil
	a.mu.Unlock()
	if w == nil {
		return nil
	}
	if w.lang != l || time.Since(w.at) > 15*time.Second || w.stream.Err() != nil {
		w.stream.Close()
		return nil
	}
	return w.stream
}

func (a *Agent) play(r *run, b []byte) {
	if r.ctx.Err() != nil {
		return
	}
	now := time.Now()
	r.mu.Lock()
	first := r.firstOut.IsZero()
	if first {
		r.firstOut = now
	}
	if r.fillerNext {
		r.fillerNext = false
	} else if r.firstReply.IsZero() {
		r.firstReply = now
	}
	speechEnd, turn := r.speechEnd, r.turnNo
	r.mu.Unlock()
	a.mu.Lock()
	if a.playUntil.Before(now) {
		a.playUntil = now
	}
	a.playUntil = a.playUntil.Add(a.out.Duration(len(b)))
	a.mu.Unlock()
	if first {
		e2e := msBetween(speechEnd, now)
		a.tr.Send(map[string]any{"type": "tts.start", "turn": turn, "format": a.out.Name, "sample_rate": a.out.Rate, "end_to_audio_ms": round(e2e)})
		a.log.Log("audio.first_out", map[string]any{"turn": turn, "end_to_audio_ms": round(e2e)})
	}
	_ = a.tr.Play(b)
	a.log.RecordOut(b, a.out)
}

func (a *Agent) fillerWatch(r *run) {
	after := a.deps.Cfg.FillerAfter
	if after <= 0 {
		return
	}
	select {
	case <-r.promoted:
	case <-r.ctx.Done():
		return
	}
	r.mu.Lock()
	start := r.speechEnd
	r.mu.Unlock()
	if start.IsZero() {
		start = time.Now()
	}
	if wait := time.Until(start.Add(after)); wait > 0 {
		t := time.NewTimer(wait)
		defer t.Stop()
		select {
		case <-t.C:
		case <-r.ctx.Done():
			return
		}
	}
	if r.hasAudio() {
		return
	}
	b, err := a.deps.TTS.Say(r.ctx, FillerText(r.lang), r.lang, a.out.Name)
	if err != nil || r.ctx.Err() != nil {
		return
	}
	r.mu.Lock()
	r.fillerNext = true
	r.mu.Unlock()
	played := r.gate.First(b)
	r.mu.Lock()
	r.fillerNext = false
	if played {
		r.filler = true
	}
	turn := r.turnNo
	r.mu.Unlock()
	if played {
		a.log.Log("filler", map[string]any{"turn": turn, "lang": r.lang})
	}
}

func (a *Agent) finish(r *run) {
	r.mu.Lock()
	m := voicelog.TurnMetrics{
		Turn: r.turnNo, Channel: a.opt.Channel, UserText: r.text, ReplyText: strings.TrimSpace(r.reply.String()),
		InLang: string(r.choice.Detected), ReplyLang: string(r.lang), LangRule: r.choice.Rule, Brain: a.deps.Brain.Name(),
		Model: r.result.Model, TTSModel: r.streamInfoSeen.Model, Voice: r.streamInfoSeen.Voice,
		Speculative: !r.promotedAt.IsZero(), Interrupted: r.interrupted.Load(), Filler: r.filler,
		STTms:        round(r.sttMS),
		BrainTTFTms:  round(msBetween(r.brainStart, r.brainFirst)),
		BrainTotalms: round(msBetween(r.brainStart, r.brainDone)),
		TTSTTFBms:    round(msBetween(r.ttsFirstText, r.ttsFirstAudio)),
		EndToAudioms: round(msBetween(r.speechEnd, r.firstOut)),
		EndToReplyms: round(msBetween(r.speechEnd, r.firstReply)),
		Totalms:      round(msBetween(r.speechEnd, time.Now())),
	}
	if r.decision != nil {
		m.Scenario, m.Confidence = r.decision.ScenarioID, r.decision.Confidence
	}
	raw := r.result.Raw
	if raw == "" {
		raw = m.ReplyText
	}
	r.mu.Unlock()

	a.log.Turn(m)
	latency := map[string]any{"stt": m.STTms, "response": m.BrainTTFTms, "tts_first_audio": m.TTSTTFBms, "total": m.EndToReplyms, "first_sound": m.EndToAudioms}
	a.tr.Send(map[string]any{"type": "voice.metrics", "turn": m.Turn, "latency_ms": latency, "metrics": m})
	if a.deps.Brain.Name() != "backend" {
		a.tr.Send(map[string]any{"type": "response.final", "text": m.ReplyText, "language": m.ReplyLang, "ms": m.BrainTTFTms})
		a.tr.Send(turnDone(m, latency))
	}
	if m.Interrupted {
		raw += " …"
	}
	a.deps.Brain.Commit(a.opt.SessionID, r.text, raw)
	if m.Scenario == "SYS_GOODBYE" && a.opt.Channel != "web" {
		go a.hangupWhenQuiet()
	}
}

func (a *Agent) greet() {
	b, err := a.deps.TTS.Say(a.ctx, Greeting, lang.Mixed, a.out.Name)
	if err != nil {
		a.logErr("greeting", err)
		return
	}
	now := time.Now()
	a.mu.Lock()
	if a.playUntil.Before(now) {
		a.playUntil = now
	}
	a.playUntil = a.playUntil.Add(a.out.Duration(len(b)))
	a.mu.Unlock()
	a.tr.Send(map[string]any{"type": "response.final", "text": Greeting, "language": "kk", "greeting": true})
	_ = a.tr.Play(b)
	a.log.Log("greeting", map[string]any{"text": Greeting, "audio_ms": round(float64(a.out.Duration(len(b))) / float64(time.Millisecond))})
	a.log.RecordOut(b, a.out)
}

func (a *Agent) bargeIn(reason string) {
	a.mu.Lock()
	r := a.active
	speaking := a.speakingLocked()
	if !speaking {
		a.mu.Unlock()
		return
	}
	if r != nil {
		r.interrupted.Store(true)
		r.cancel()
		a.active = nil
	}
	a.playUntil = time.Time{}
	a.mu.Unlock()
	a.tr.Clear()
	a.tr.Send(map[string]any{"type": "tts.clear", "reason": reason})
	a.log.Log("barge_in", map[string]any{"reason": reason})
}

func (a *Agent) speakingLocked() bool { return time.Now().Before(a.playUntil) }

// Speaking reports whether agent audio is (probably) still playing.
func (a *Agent) Speaking() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.speakingLocked()
}

func (a *Agent) hangupWhenQuiet() {
	deadline := time.Now().Add(15 * time.Second)
	for a.Speaking() && time.Now().Before(deadline) {
		select {
		case <-time.After(100 * time.Millisecond):
		case <-a.ctx.Done():
			return
		}
	}
	time.Sleep(300 * time.Millisecond)
	a.log.Log("hangup", map[string]any{"reason": "goodbye"})
	a.tr.Hangup()
}

func (a *Agent) shutdown() {
	a.cancel()
	a.mu.Lock()
	a.closed = true
	if a.specTimer != nil {
		a.specTimer.Stop()
	}
	runs := []*run{a.active, a.spec}
	w := a.warm
	a.warm = nil
	a.mu.Unlock()
	for _, r := range runs {
		if r != nil {
			r.cancel()
		}
	}
	if w != nil {
		w.stream.Close()
	}
	a.deps.Brain.End(a.opt.SessionID)
	a.log.Close("hangup")
}

func (a *Agent) logErr(stage string, err error) {
	if err == nil {
		return
	}
	a.log.Log("error", map[string]any{"stage": stage, "error": err.Error()})
}

// gate holds a speculative reply's audio and UI events until it is confirmed.
type gate struct {
	mu     sync.Mutex
	open   bool
	played bool
	held   []gateItem
	play   func([]byte)
	send   func(map[string]any)
}

type gateItem struct {
	audio []byte
	ev    map[string]any
}

func (g *gate) Audio(b []byte) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.open {
		g.held = append(g.held, gateItem{audio: b})
		return
	}
	g.played = true
	g.play(b)
}

func (g *gate) Event(ev map[string]any) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.open {
		g.held = append(g.held, gateItem{ev: ev})
		return
	}
	g.send(ev)
}

func (g *gate) Open() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.open {
		return
	}
	g.open = true
	for _, it := range g.held {
		if it.audio != nil {
			g.played = true
			g.play(it.audio)
		} else {
			g.send(it.ev)
		}
	}
	g.held = nil
}

// First plays b only if nothing has been played yet (fillers).
func (g *gate) First(b []byte) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.open || g.played {
		return false
	}
	g.played = true
	g.play(b)
	return true
}

func contractDecision(d *brain.Decision, c lang.Choice, model string) map[string]any {
	return map[string]any{
		"scenarios":       []map[string]any{{"scenario_id": d.ScenarioID, "confidence": d.Confidence, "reason": d.Reason}},
		"alternatives":    []any{},
		"language":        langOr(c.Detected, c.Reply),
		"slots":           map[string]any{},
		"is_continuation": false,
		"reason":          d.Reason,
		"model":           model,
	}
}

func turnDone(m voicelog.TurnMetrics, latency map[string]any) map[string]any {
	scen := []map[string]any{}
	if m.Scenario != "" {
		scen = append(scen, map[string]any{"scenario_id": m.Scenario, "confidence": m.Confidence})
	}
	return map[string]any{
		"type":       "turn.done",
		"latency_ms": latency,
		"trace": map[string]any{
			"turn": m.Turn, "transcript": m.UserText, "language": m.InLang, "scenarios": scen, "alternatives": []any{},
			"reason": "", "slots": map[string]any{}, "actions": []string{}, "latency_ms": latency,
			"policy":        map[string]any{"action": "run", "scenario_id": m.Scenario, "reason": "", "stack": []string{}, "low_conf_streak": 0},
			"response_text": m.ReplyText, "response_lang": m.ReplyLang, "model": m.Model,
		},
	}
}

func fallbackReply(l lang.Lang) string {
	if l == lang.KK {
		return "Кешіріңіз, қазір жауап бере алмадым. Операторға қосайын ба?"
	}
	return "Извините, не получилось ответить. Соединить вас с оператором?"
}

func langOr(detected, reply lang.Lang) string {
	if detected != lang.Unknown {
		return string(detected)
	}
	return string(reply)
}

func normalize(s string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
		} else if !space && b.Len() > 0 {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

func wordCount(s string) int { return len(strings.Fields(normalize(s))) }

// specMatches decides whether a reply generated from a partial transcript can
// stand for the final one. Partials often end mid-word ("а полис н"), so the
// last speculated word is dropped; the final text must continue the rest and
// the speculated prefix must cover at least 80% of it.
func specMatches(spec, final string) bool {
	if spec == final {
		return true
	}
	words := strings.Fields(spec)
	if len(words) < 3 {
		return false
	}
	prefix := strings.Join(words[:len(words)-1], " ")
	if !strings.HasPrefix(final, prefix) {
		return false
	}
	return float64(len([]rune(prefix))) >= 0.8*float64(len([]rune(final)))
}

func msBetween(from, to time.Time) float64 {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return 0
	}
	return float64(to.Sub(from)) / float64(time.Millisecond)
}

func round(v float64) float64 { return float64(int64(v*10+0.5)) / 10 }
