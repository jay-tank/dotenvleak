package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Severity levels for a Finding.
type Severity string

const (
	Blocker Severity = "blocker"
	Warning Severity = "warning"
)

// RuleID identifies which dotenvleak check produced a Finding.
type RuleID string

const (
	// DL001: a .env* file with secret-looking content is currently
	// tracked by git.
	DL001 RuleID = "DL001"
	// DL002: a .env* file with secret-looking content is untracked, and
	// no applicable .gitignore pattern actually excludes it.
	DL002 RuleID = "DL002"
	// DL003: a .env* file exists with no clearly secret-looking content
	// detected. Opt-in via --strict.
	DL003 RuleID = "DL003"
)

// Finding is one reported issue.
type Finding struct {
	Rule     RuleID   `json:"rule"`
	Severity Severity `json:"severity"`
	Path     string   `json:"path"`
	Message  string   `json:"message"`
	Keys     []string `json:"secretKeys,omitempty"`
}

func newDL001(relPath string, keys []string) Finding {
	return Finding{
		Rule:     DL001,
		Severity: Blocker,
		Path:     relPath,
		Message:  fmt.Sprintf("%s is tracked by git and contains secret-looking value(s) for: %s", relPath, strings.Join(keys, ", ")),
		Keys:     keys,
	}
}

func newDL002(relPath string, keys []string) Finding {
	return Finding{
		Rule:     DL002,
		Severity: Blocker,
		Path:     relPath,
		Message:  fmt.Sprintf("%s is untracked but NOT excluded by any applicable .gitignore, and contains secret-looking value(s) for: %s (a `git add .` away from being leaked)", relPath, strings.Join(keys, ", ")),
		Keys:     keys,
	}
}

func newDL003(relPath string) Finding {
	return Finding{
		Rule:     DL003,
		Severity: Warning,
		Path:     relPath,
		Message:  fmt.Sprintf("%s is a .env-style file with no clearly secret-looking content detected; worth a human glance (heuristic is not exhaustive)", relPath),
	}
}

// RenderText renders findings as human-readable text.
func RenderText(findings []Finding) string {
	if len(findings) == 0 {
		return "dotenvleak: no issues found"
	}
	sorted := sortedFindings(findings)
	var b strings.Builder
	for _, f := range sorted {
		fmt.Fprintf(&b, "[%s] %s: %s\n", f.Rule, strings.ToUpper(string(f.Severity)), f.Message)
	}
	fmt.Fprintf(&b, "\n%d finding(s)", len(sorted))
	return strings.TrimRight(b.String(), "\n")
}

// RenderJSON renders findings as JSON.
func RenderJSON(findings []Finding) (string, error) {
	sorted := sortedFindings(findings)
	if sorted == nil {
		sorted = []Finding{}
	}
	out, err := json.MarshalIndent(sorted, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func sortedFindings(findings []Finding) []Finding {
	sorted := make([]Finding, len(findings))
	copy(sorted, findings)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Path != sorted[j].Path {
			return sorted[i].Path < sorted[j].Path
		}
		return sorted[i].Rule < sorted[j].Rule
	})
	return sorted
}
