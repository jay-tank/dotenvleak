package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitignorePatternMatching(t *testing.T) {
	cases := []struct {
		name          string
		pattern       string
		sourceDir     string
		relFromSource string
		want          bool
	}{
		{"plain basename matches at any depth", ".env", "/repo", ".env", true},
		{"plain basename matches nested", ".env", "/repo", "sub/dir/.env", true},
		{"star glob", ".env.*", "/repo", ".env.local", true},
		{"star glob no match", ".env.*", "/repo", ".envrc", false},
		{"anchored root-only pattern matches only at root", "/.env", "/repo", ".env", true},
		{"anchored root-only pattern does not match nested", "/.env", "/repo", "sub/.env", false},
		{"double star prefix", "**/.env.local", "/repo", "a/b/.env.local", true},
		{"double star suffix", "secrets/**", "/repo", "secrets/deep/.env", true},
		{"unrelated pattern does not match", "*.log", "/repo", ".env", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			parsed := parseGitignore(c.pattern, c.sourceDir)
			if len(parsed) != 1 {
				t.Fatalf("expected 1 parsed pattern, got %d", len(parsed))
			}
			got := parsed[0].matches(c.relFromSource)
			if got != c.want {
				t.Errorf("pattern %q against %q = %v, want %v", c.pattern, c.relFromSource, got, c.want)
			}
		})
	}
}

func TestGitignoreNegation(t *testing.T) {
	content := ".env*\n!.env.example\n"
	patterns := parseGitignore(content, "/repo")
	if len(patterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(patterns))
	}
	if isIgnoredByChain(patterns, "/repo/.env") != true {
		t.Errorf(".env should be ignored")
	}
	// Negation should re-include the example file's basename match.
	if isIgnoredByChain(patterns, "/repo/.env.example") != false {
		t.Errorf(".env.example should be re-included by negation")
	}
}

func TestIsEffectivelyGitignoredWalksUpChain(t *testing.T) {
	root := t.TempDir()

	// Root .gitignore protects .env.local everywhere.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".env.local\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(root, "services", "api")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	protectedFile := filepath.Join(subDir, ".env.local")
	if err := os.WriteFile(protectedFile, []byte("API_KEY=sk-testnotreal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isEffectivelyGitignored(root, protectedFile) {
		t.Errorf("expected %s to be covered by root .gitignore", protectedFile)
	}

	// A sibling .env.production has no protecting pattern anywhere.
	unprotectedFile := filepath.Join(subDir, ".env.production")
	if err := os.WriteFile(unprotectedFile, []byte("API_KEY=sk-testnotreal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if isEffectivelyGitignored(root, unprotectedFile) {
		t.Errorf("expected %s to NOT be covered by any .gitignore", unprotectedFile)
	}

	// A nested .gitignore can narrow protection further down the tree.
	if err := os.WriteFile(filepath.Join(subDir, ".gitignore"), []byte(".env.production\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isEffectivelyGitignored(root, unprotectedFile) {
		t.Errorf("expected %s to now be covered by the nested .gitignore", unprotectedFile)
	}
}
