// Package transport adapts call channels to the conversation engine: the
// browser voice WebSocket, Asterisk AudioSocket (a Kazakhstan SIP number) and
// Twilio Media Streams. It does not import the engine; the server wires a
// Starter that creates engine calls.
package transport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

// Sink receives agent output. Its method set is identical to agent.Transport.
type Sink interface {
	OutputFormat() string    // "pcm_8000" (Asterisk) | "ulaw_8000" (Twilio) | "pcm_16000" (web)
	Play(audio []byte) error // queue audio; must not block for long
	Clear()                  // drop queued audio now (barge-in)
	Send(ev map[string]any)  // UI event; phone sinks ignore it
	Hangup()                 // end the call from the agent side
}

// Call is one running conversation (implemented by the engine).
type Call interface {
	PushAudio(frame []byte) // caller audio in CallInfo.InputFormat
	Commit()
	SubmitText(text string)
	Interrupt()
	Close()
	Run() error // blocks until the call ends
}

// CallInfo describes a new call.
type CallInfo struct {
	SessionID   string
	Channel     string // "phone" (Asterisk) | "twilio" | "web"
	CallerID    string // E.164 when known
	InputFormat string // "pcm_8000" | "ulaw_8000" | "pcm_16000"
	Manual      bool
	Greeting    bool
	Lang        string
	Meta        map[string]any
}

// Starter creates a call; the server wires it to the engine.
type Starter func(ctx context.Context, info CallInfo, sink Sink) Call

// NewID returns prefix + "-" + 12 random hex characters.
func NewID(prefix string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return prefix + "-" + hex.EncodeToString(b)
}
