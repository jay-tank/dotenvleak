package main

import (
	"bufio"
	"strings"
)

// secretKeyMarkers are case-insensitive substrings that, when present in a
// KEY name, make dotenvleak consider that key "secret-shaped". This is a
// deliberately broad, high-recall heuristic: it will flag things like
// SECRET_LOG_LEVEL (a false positive) in exchange for rarely missing an
// actual secret. See DETAILS.md for the documented trade-off.
var secretKeyMarkers = []string{
	"PASSWORD",
	"SECRET",
	"TOKEN",
	"KEY",
	"API_KEY",
	"PRIVATE",
	"CREDENTIAL",
}

// placeholderValues are exact (case-insensitive) values treated as obvious
// placeholders rather than real secrets.
var placeholderValues = []string{
	"xxx",
	"changeme",
	"change_me",
	"change-me",
	"your-key-here",
	"your_key_here",
	"placeholder",
	"todo",
	"tbd",
	"n/a",
	"na",
}

// envAssignment is one parsed KEY=value line from a dotenv-style file.
type envAssignment struct {
	Key   string
	Value string
	Line  int // 1-based line number in the source file
}

// parseEnvAssignments extracts KEY=value pairs from dotenv-style content.
// It is intentionally simple: it skips blank lines and lines starting with
// '#' (comments), optionally strips a leading "export " prefix (common in
// shell-sourced .env files), and requires the key to look like a shell/env
// identifier (letters, digits, underscore, not starting with a digit). It
// does not attempt full shell parsing (no line continuations, no multi-line
// quoted values) — see DETAILS.md for the exact documented scope.
func parseEnvAssignments(content string) []envAssignment {
	var out []envAssignment
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		eq := strings.Index(line, "=")
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		if !isValidEnvKey(key) {
			continue
		}
		value := strings.TrimSpace(line[eq+1:])
		value = unquote(value)
		out = append(out, envAssignment{Key: key, Value: value, Line: lineNo})
	}
	return out
}

func isValidEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r == '_':
			// ok anywhere
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func unquote(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}

// looksSecretKey reports whether key contains one of secretKeyMarkers as a
// case-insensitive substring.
func looksSecretKey(key string) bool {
	upper := strings.ToUpper(key)
	for _, marker := range secretKeyMarkers {
		if strings.Contains(upper, marker) {
			return true
		}
	}
	return false
}

// looksLikePlaceholder reports whether value is empty, an exact-match known
// placeholder token (case-insensitive), or wrapped in angle brackets like
// "<your-secret>". It is a precise, low-recall heuristic on purpose: it only
// catches the conventional placeholder idioms, not every string a human
// might use as a stand-in, because guessing further starts producing false
// negatives disguised as caution (see DETAILS.md).
func looksLikePlaceholder(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return true
	}
	if strings.HasPrefix(trimmed, "<") && strings.HasSuffix(trimmed, ">") {
		return true
	}
	lower := strings.ToLower(trimmed)
	for _, ph := range placeholderValues {
		if lower == ph {
			return true
		}
	}
	// Repeated "x" characters of any length (xxx, xxxxxxxx) are a very
	// common redacted-placeholder idiom.
	if strings.Count(lower, "x") == len(lower) && len(lower) > 0 {
		return true
	}
	return false
}

// findSecretLikeAssignments returns the subset of assignments that look
// like a real secret: a secret-shaped key name with a non-placeholder
// value.
func findSecretLikeAssignments(assignments []envAssignment) []envAssignment {
	var out []envAssignment
	for _, a := range assignments {
		if looksSecretKey(a.Key) && !looksLikePlaceholder(a.Value) {
			out = append(out, a)
		}
	}
	return out
}
