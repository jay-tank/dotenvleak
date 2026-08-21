package main

import (
	"bytes"
	"strings"
	"testing"
)

// fakeWriter is a minimal io.Writer used to capture CLI output in tests.
type fakeWriter struct {
	bytes.Buffer
}

func containsSubstring(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

func TestRunUsageError(t *testing.T) {
	var stdout, stderr fakeWriter
	code := run(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2 for no args, got %d", code)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Errorf("expected usage message, got %s", stderr.String())
	}
}

func TestRunUnknownFlag(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr fakeWriter
	code := run([]string{"scan", dir, "--bogus"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2 for unknown flag, got %d", code)
	}
}

func TestRunMissingDirectory(t *testing.T) {
	var stdout, stderr fakeWriter
	code := run([]string{"scan", "/path/does/not/exist/dotenvleak-test"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2 for missing directory, got %d", code)
	}
}

func TestRunCleanRepoExitsZero(t *testing.T) {
	repo := newRealGitRepo(t)
	writeFile(t, repo+"/.env.example", "API_KEY=your-key-here\n")

	var stdout, stderr fakeWriter
	code := run([]string{"scan", repo}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0 for clean repo, got %d; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "no issues found") {
		t.Errorf("expected clean message, got %s", stdout.String())
	}
}
