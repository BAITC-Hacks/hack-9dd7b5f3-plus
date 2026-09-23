package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// resolver looks up a config value by one or more alias names. The process
// environment is checked in full (every alias, in priority order) before
// falling back to the parsed .env file, so an environment variable always
// overrides the same key coming from a file.
type resolver struct {
	file map[string]string
}

// get returns the first non-empty value found for names, checking the
// process environment for every alias before checking the .env file for
// every alias. An empty string means none of the names were set.
func (r resolver) get(names ...string) string {
	for _, n := range names {
		if v, ok := os.LookupEnv(n); ok && v != "" {
			return v
		}
	}
	for _, n := range names {
		if v, ok := r.file[n]; ok && v != "" {
			return v
		}
	}
	return ""
}

// getFloat parses a float value, returning def if unset or invalid.
func (r resolver) getFloat(def float64, names ...string) float64 {
	v := r.get(names...)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return def
	}
	return f
}

// getInt parses an integer value, returning def if unset or invalid.
func (r resolver) getInt(def int, names ...string) int {
	v := r.get(names...)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return n
}

// getBool parses 1/true/yes/on as true and 0/false/no/off as false
// (case-insensitive); anything else, including unset, returns def.
func (r resolver) getBool(def bool, names ...string) bool {
	v := strings.ToLower(strings.TrimSpace(r.get(names...)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

// getDurationMS parses an integer number of milliseconds into a
// time.Duration, returning def if unset or invalid.
func (r resolver) getDurationMS(def time.Duration, names ...string) time.Duration {
	v := r.get(names...)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return time.Duration(n) * time.Millisecond
}

// getList parses a comma-separated list, trimming whitespace and dropping
// empty entries. It returns def if unset or if nothing remains after
// trimming.
func (r resolver) getList(def []string, names ...string) []string {
	v := r.get(names...)
	if v == "" {
		return def
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}

// normalizeKey trims, upper-cases and turns '-' into '_' so that keys like
// "eleven-labs" resolve to the canonical "ELEVEN_LABS".
func normalizeKey(k string) string {
	k = strings.TrimSpace(k)
	k = strings.ToUpper(k)
	k = strings.ReplaceAll(k, "-", "_")
	return k
}
