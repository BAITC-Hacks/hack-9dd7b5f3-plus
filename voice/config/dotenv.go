package config

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// maxWalkLevels bounds how far FindDotEnv and FindDataDir walk up the
// directory tree (in addition to checking the starting directory itself).
const maxWalkLevels = 6

// ParseDotEnv reads a .env-style document and returns its keys and values.
// Keys are normalized with normalizeKey (trimmed, upper-cased, '-' -> '_'),
// so hyphenated real-world keys like "eleven-labs" come back as
// "ELEVEN_LABS". Supported syntax:
//
//   - blank lines and "# comment" lines are ignored
//   - an optional leading "export " before KEY=VALUE
//   - VALUE may be double- or single-quoted; quotes are stripped and the
//     content is used verbatim (no escape processing)
//   - an unquoted VALUE may carry an inline " #comment" which is stripped
//   - CRLF and LF line endings are both accepted
//
// Lines without an '=' are ignored. Later lines override earlier ones for
// the same normalized key.
func ParseDotEnv(r io.Reader) map[string]string {
	result := make(map[string]string)

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))

		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := normalizeKey(line[:eq])
		if key == "" {
			continue
		}
		result[key] = parseDotEnvValue(strings.TrimSpace(line[eq+1:]))
	}

	return result
}

// parseDotEnvValue strips surrounding quotes (verbatim), or, for an
// unquoted value, strips a trailing " #comment".
func parseDotEnvValue(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	if i := strings.Index(v, " #"); i >= 0 {
		return strings.TrimSpace(v[:i])
	}
	return v
}

// FindDotEnv walks up from start (the starting directory itself, then up to
// maxWalkLevels parents) and returns the path of the first ".env" file it
// finds, or "" if none exists within that range.
func FindDotEnv(start string) string {
	return findUpward(start, func(dir string) (string, bool) {
		candidate := filepath.Join(dir, ".env")
		if isFile(candidate) {
			return candidate, true
		}
		return "", false
	})
}

// FindDataDir walks up from start (the starting directory itself, then up
// to maxWalkLevels parents) looking for a "data/scenarios.json" file, and
// returns the absolute path of the containing "data" directory, or "" if
// none exists within that range.
func FindDataDir(start string) string {
	return findUpward(start, func(dir string) (string, bool) {
		dataDir := filepath.Join(dir, "data")
		if isFile(filepath.Join(dataDir, "scenarios.json")) {
			return dataDir, true
		}
		return "", false
	})
}

// findUpward runs check against start and each ancestor up to
// maxWalkLevels levels above it, in absolute form, returning the first
// match.
func findUpward(start string, check func(dir string) (string, bool)) string {
	if start == "" {
		return ""
	}
	dir := start
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}

	for i := 0; i <= maxWalkLevels; i++ {
		if found, ok := check(dir); ok {
			return found
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// isFile reports whether path exists and is a regular file (not a
// directory).
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
