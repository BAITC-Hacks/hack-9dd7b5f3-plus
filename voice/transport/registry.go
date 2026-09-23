package transport

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

// Registry maps AudioSocket call UUIDs to caller numbers. AudioSocket carries
// only a UUID, so the Asterisk dialplan first calls
// GET /asterisk/call?uuid=<uuid>&caller=<number>&called=<did>.
type Registry struct {
	mu      sync.Mutex
	entries map[string]regEntry
	ttl     time.Duration
}

type regEntry struct {
	caller, called string
	at             time.Time
}

// NewRegistry returns a registry whose entries expire after 2 minutes.
func NewRegistry() *Registry {
	return &Registry{entries: make(map[string]regEntry), ttl: 2 * time.Minute}
}

// Put remembers who is calling for a UUID.
func (r *Registry) Put(uuid, caller, called string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for k, e := range r.entries {
		if now.Sub(e.at) > r.ttl {
			delete(r.entries, k)
		}
	}
	r.entries[strings.ToLower(uuid)] = regEntry{caller: caller, called: called, at: now}
}

// Take returns and forgets the caller registered for uuid.
func (r *Registry) Take(uuid string) (caller, called string, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := strings.ToLower(uuid)
	e, found := r.entries[key]
	delete(r.entries, key)
	if !found || time.Since(e.at) > r.ttl {
		return "", "", false
	}
	return e.caller, e.called, true
}

// ServeHTTP handles the dialplan registration call.
func (r *Registry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	uuid := q.Get("uuid")
	if uuid == "" {
		http.Error(w, "uuid required", http.StatusBadRequest)
		return
	}
	r.Put(uuid, q.Get("caller"), q.Get("called"))
	w.WriteHeader(http.StatusNoContent)
}

// NormalizeCaller turns a caller ID into E.164 when it looks like a
// Kazakhstan number (8XXXXXXXXXX or 7XXXXXXXXXX -> +7XXXXXXXXXX).
func NormalizeCaller(s string) string {
	var digits strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	switch {
	case d == "":
		return ""
	case len(d) == 11 && d[0] == '8':
		return "+7" + d[1:]
	case len(d) == 11 && d[0] == '7':
		return "+" + d
	case len(d) == 10 && d[0] == '7':
		return "+7" + d
	case strings.HasPrefix(strings.TrimSpace(s), "+"):
		return "+" + d
	}
	return d
}
