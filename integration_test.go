package main

// Integration tests that build a REAL throwaway git repository (via actual
// `git` subprocess calls against a t.TempDir()) and drive dotenvleak's
// tracked/untracked and gitignore-matching logic against it end to end.
// Faking `git ls-files` output would defeat the entire point of this tool,
// since its whole value proposition is "ask git the truth, don't guess" —
// so these tests genuinely exercise `git init`, `git add`, `git commit`.
//
// All secret-shaped values used below are obviously-fake, non-functional
// placeholders (e.g. "sk-thisisatestkeynotreal1234567890") chosen only to
// satisfy the KEY=value secret-looking-content heuristic under test; none
// of them are, or resemble, any real credential.

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newRealGitRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	// Silence any global gitignore / template influence from the host.
	runGit(t, dir, "config", "core.excludesFile", "")
	return dir
}

func TestIntegration_TrackedEnvFileWithSecret_DL001(t *testing.T) {
	repo := newRealGitRepo(t)

	writeFile(t, filepath.Join(repo, ".env"), "API_KEY=sk-thisisatestkeynotreal1234567890\n")
	runGit(t, repo, "add", ".env")
	runGit(t, repo, "commit", "-q", "-m", "accidentally commit env")

	findings, err := Check(repo, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Rule != DL001 {
		t.Fatalf("expected exactly one DL001 finding, got %+v", findings)
	}
	if findings[0].Severity != Blocker {
		t.Errorf("expected DL001 to be a blocker, got %s", findings[0].Severity)
	}
}

func TestIntegration_UntrackedEnvFileNotGitignored_DL002(t *testing.T) {
	repo := newRealGitRepo(t)

	// No .gitignore at all: the file sits on disk, untracked, with secret
	// content, and nothing would stop `git add .` from picking it up.
	writeFile(t, filepath.Join(repo, ".env"), "DB_PASSWORD=notarealpassword987\n")

	findings, err := Check(repo, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Rule != DL002 {
		t.Fatalf("expected exactly one DL002 finding, got %+v", findings)
	}
}

func TestIntegration_UntrackedEnvFileProtectedByGitignore_Clean(t *testing.T) {
	repo := newRealGitRepo(t)

	writeFile(t, filepath.Join(repo, ".gitignore"), ".env\n.env.*\n")
	writeFile(t, filepath.Join(repo, ".env"), "DB_PASSWORD=notarealpassword987\n")
	writeFile(t, filepath.Join(repo, ".env.local"), "API_KEY=sk-thisisatestkeynotreal1234567890\n")

	findings, err := Check(repo, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings when .gitignore actually protects the files, got %+v", findings)
	}
}

func TestIntegration_TemplateFileNeverFlagged(t *testing.T) {
	repo := newRealGitRepo(t)

	writeFile(t, filepath.Join(repo, ".env.example"), "API_KEY=your-key-here\n")
	runGit(t, repo, "add", ".env.example")
	runGit(t, repo, "commit", "-q", "-m", "add template")

	findings, err := Check(repo, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("template files must never be flagged, got %+v", findings)
	}
}

func TestIntegration_TrackedEnvWithoutSecretContent_DL003OptIn(t *testing.T) {
	repo := newRealGitRepo(t)

	writeFile(t, filepath.Join(repo, ".env"), "LOG_LEVEL=debug\nPORT=8080\n")
	runGit(t, repo, "add", ".env")
	runGit(t, repo, "commit", "-q", "-m", "add benign env")

	// Off by default.
	findings, err := Check(repo, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings without --strict, got %+v", findings)
	}

	// Opt-in via Strict.
	strictOpts := DefaultOptions()
	strictOpts.Strict = true
	findings, err = Check(repo, strictOpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Rule != DL003 {
		t.Fatalf("expected one DL003 finding under --strict, got %+v", findings)
	}
	if findings[0].Severity != Warning {
		t.Errorf("expected DL003 to be a warning, got %s", findings[0].Severity)
	}
}

func TestIntegration_NestedGitignoreOverridesRoot(t *testing.T) {
	repo := newRealGitRepo(t)

	// Root .gitignore does NOT protect .env.production, but a nested one
	// scoped to services/api does.
	writeFile(t, filepath.Join(repo, ".gitignore"), ".env.local\n")
	writeFile(t, filepath.Join(repo, "services", "api", ".gitignore"), ".env.production\n")
	writeFile(t, filepath.Join(repo, "services", "api", ".env.production"), "SECRET_TOKEN=notrealtoken12345\n")
	// A sibling package with the same filename but no nested protection.
	writeFile(t, filepath.Join(repo, "services", "worker", ".env.production"), "SECRET_TOKEN=notrealtoken12345\n")

	findings, err := Check(repo, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected exactly one DL002 finding (the unprotected worker file), got %+v", findings)
	}
	if findings[0].Path != "services/worker/.env.production" {
		t.Errorf("expected finding for services/worker/.env.production, got %s", findings[0].Path)
	}
}

func TestIntegration_CLIEndToEnd(t *testing.T) {
	repo := newRealGitRepo(t)
	writeFile(t, filepath.Join(repo, ".env"), "API_KEY=sk-thisisatestkeynotreal1234567890\n")
	runGit(t, repo, "add", ".env")
	runGit(t, repo, "commit", "-q", "-m", "leak")

	var stdout, stderr fakeWriter
	code := run([]string{"scan", repo, "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d; stderr=%s", code, stderr.String())
	}
	if !containsSubstring(stdout.String(), "DL001") {
		t.Errorf("expected DL001 in JSON output, got %s", stdout.String())
	}
}

func TestIntegration_CLINotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr fakeWriter
	code := run([]string{"scan", dir}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2 for non-git directory, got %d", code)
	}
}
