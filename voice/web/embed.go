// Package web embeds the single-page browser demo client for the voice
// gateway. It is served at "/" so the team can exercise STT, the LLM reply
// and TTS end to end over the /ws/voice WebSocket protocol documented in
// index.html, and it doubles as the reference implementation for the real
// Next.js frontend.
package web

import _ "embed"

//go:embed index.html
var Index []byte

// Call is the phone-style page (/call): dialer, ringback, hands-free call
// screen with live captions. Open it on a phone over HTTPS.
//
//go:embed call.html
var Call []byte
