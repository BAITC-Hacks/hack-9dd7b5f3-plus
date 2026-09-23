package transport

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"
)

// AudioSocket message kinds (Asterisk res_audiosocket).
const (
	asHangup byte = 0x00
	asUUID   byte = 0x01
	asDTMF   byte = 0x03
	asAudio  byte = 0x10
	asError  byte = 0xff
)

// frameBytes is 20 ms of slin 8 kHz mono (16-bit little-endian).
const frameBytes = 320

// AudioSocketServer accepts calls from Asterisk's AudioSocket() dialplan app:
// a KZ SIP number -> Asterisk -> TCP -> this server, 8 kHz slin both ways.
type AudioSocketServer struct {
	Start    Starter
	Registry *Registry // optional caller-ID lookup
	Greeting bool
	Logger   *slog.Logger
}

func (s *AudioSocketServer) log() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

// ListenAndServe listens on addr until ctx is done.
func (s *AudioSocketServer) ListenAndServe(ctx context.Context, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.Serve(ctx, ln)
}

// Serve accepts connections on ln until ctx is done.
func (s *AudioSocketServer) Serve(ctx context.Context, ln net.Listener) error {
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			return err
		}
		go s.handle(ctx, conn)
	}
}

func readMessage(r io.Reader) (byte, []byte, error) {
	var hdr [3]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, nil, err
	}
	n := binary.BigEndian.Uint16(hdr[1:])
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return hdr[0], payload, nil
}

func writeMessage(w io.Writer, kind byte, payload []byte) error {
	buf := make([]byte, 3+len(payload))
	buf[0] = kind
	binary.BigEndian.PutUint16(buf[1:], uint16(len(payload)))
	copy(buf[3:], payload)
	_, err := w.Write(buf)
	return err
}

func formatUUID(b []byte) string {
	h := hex.EncodeToString(b)
	if len(h) != 32 {
		return h
	}
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

func (s *AudioSocketServer) handle(parent context.Context, conn net.Conn) {
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	kind, payload, err := readMessage(conn)
	if err != nil || kind != asUUID || len(payload) != 16 {
		s.log().Warn("audiosocket: expected UUID message", "remote", conn.RemoteAddr(), "err", err)
		return
	}
	_ = conn.SetReadDeadline(time.Time{})
	uuid := formatUUID(payload)
	var caller, called string
	if s.Registry != nil {
		caller, called, _ = s.Registry.Take(uuid)
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	sink := newASSink(ctx, conn)
	defer sink.stop()
	call := s.Start(ctx, CallInfo{
		SessionID: "ast-" + uuid, Channel: "phone", CallerID: NormalizeCaller(caller), InputFormat: "pcm_8000",
		Greeting: s.Greeting, Meta: map[string]any{"uuid": uuid, "called": called, "remote": conn.RemoteAddr().String()},
	}, sink)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := call.Run(); err != nil {
			s.log().Warn("audiosocket call ended with error", "uuid", uuid, "err", err)
		}
		cancel()
		conn.Close()
	}()
	s.log().Info("audiosocket call started", "uuid", uuid, "caller", caller)
	for {
		kind, payload, err := readMessage(conn)
		if err != nil {
			break
		}
		switch kind {
		case asAudio:
			call.PushAudio(payload)
		case asDTMF:
			s.log().Debug("audiosocket dtmf", "uuid", uuid, "digit", string(payload))
		case asError:
			s.log().Warn("audiosocket error from asterisk", "uuid", uuid, "code", fmt.Sprintf("%x", payload))
		case asHangup:
			goto end
		}
	}
end:
	call.Close()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}
}

// asSink paces agent audio to Asterisk in 20 ms frames of 320 bytes.
type asSink struct {
	conn   net.Conn
	ctx    context.Context
	cancel context.CancelFunc
	wmu    sync.Mutex
	mu     sync.Mutex
	queue  []byte
	wake   chan struct{}
	done   chan struct{}
}

func newASSink(parent context.Context, conn net.Conn) *asSink {
	ctx, cancel := context.WithCancel(parent)
	s := &asSink{conn: conn, ctx: ctx, cancel: cancel, wake: make(chan struct{}, 1), done: make(chan struct{})}
	go s.pace()
	return s
}

func (s *asSink) OutputFormat() string { return "pcm_8000" }

func (s *asSink) Play(b []byte) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	s.queue = append(s.queue, b...)
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return nil
}

func (s *asSink) Clear() {
	s.mu.Lock()
	s.queue = s.queue[:0]
	s.mu.Unlock()
}

func (s *asSink) Send(map[string]any) {}

func (s *asSink) Hangup() {
	s.write(asHangup, nil)
	s.cancel()
	s.conn.Close()
}

func (s *asSink) write(kind byte, payload []byte) error {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	_ = s.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	return writeMessage(s.conn, kind, payload)
}

// next pops one 20 ms frame (zero-padded), or nil if the queue is empty.
func (s *asSink) next() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.queue) == 0 {
		return nil
	}
	frame := make([]byte, frameBytes)
	n := copy(frame, s.queue)
	s.queue = s.queue[n:]
	return frame
}

// pace sends frames in real time: the first two frames of a burst go out
// immediately (jitter cushion), then one every 20 ms without drifting.
func (s *asSink) pace() {
	defer close(s.done)
	const step = 20 * time.Millisecond
	for {
		frame := s.next()
		if frame == nil {
			select {
			case <-s.wake:
				continue
			case <-s.ctx.Done():
				return
			}
		}
		// burst start
		if err := s.write(asAudio, frame); err != nil {
			return
		}
		if f := s.next(); f != nil {
			if err := s.write(asAudio, f); err != nil {
				return
			}
		}
		due := time.Now().Add(step)
		for {
			wait := time.Until(due)
			if wait > 0 {
				select {
				case <-time.After(wait):
				case <-s.ctx.Done():
					return
				}
			}
			f := s.next()
			if f == nil {
				break
			}
			if err := s.write(asAudio, f); err != nil {
				return
			}
			due = due.Add(step)
			if lag := time.Since(due); lag > 3*step {
				due = time.Now() // fell behind: resync instead of bursting
			}
		}
	}
}

func (s *asSink) stop() {
	s.cancel()
	<-s.done
}
