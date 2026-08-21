package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsEnvFilename(t *testing.T) {
	cases := map[string]bool{
		".env":            true,
		".env.local":      true,
		".env.production": true,
		".env-ci":         true,
		".env_test":       true,
		"env":             false,
		"foo.env":         false,
		".environment":    false,
		"myenvfile":       false,
	}
	for name, want := range cases {
		if got := isEnvFilename(name); got != want {
			t.Errorf("isEnvFilename(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestIsTemplateFile(t *testing.T) {
	suffixes := defaultTemplateSuffixes
	cases := map[string]bool{
		".env.example":    true,
		".env.sample":     true,
		".env.template":   true,
		".env.dist":       true,
		".env.EXAMPLE":    true,
		".env":            false,
		".env.local":      false,
		".env.production": false,
	}
	for name, want := range cases {
		if got := isTemplateFile(name, suffixes); got != want {
			t.Errorf("isTemplateFile(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestFindEnvFiles(t *testing.T) {
	root := t.TempDir()

	mustWrite := func(rel, content string) {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mustWrite(".env", "API_KEY=sk-testnotreal123\n")
	mustWrite(".env.local", "API_KEY=sk-testnotreal123\n")
	mustWrite(".env.example", "API_KEY=your-key-here\n") // template, excluded
	mustWrite("src/app/.env.production", "API_KEY=sk-testnotreal123\n")
	mustWrite("node_modules/pkg/.env", "API_KEY=sk-testnotreal123\n") // excluded dir
	mustWrite("vendor/lib/.env", "API_KEY=sk-testnotreal123\n")       // excluded dir
	mustWrite("README.md", "not an env file")

	found, err := FindEnvFiles(root, defaultExcludeDirs, defaultTemplateSuffixes)
	if err != nil {
		t.Fatal(err)
	}

	rels := map[string]bool{}
	for _, f := range found {
		rels[f.RelPath] = true
	}

	wantPresent := []string{".env", ".env.local", "src/app/.env.production"}
	for _, w := range wantPresent {
		if !rels[w] {
			t.Errorf("expected %s to be found, got %+v", w, rels)
		}
	}

	wantAbsent := []string{".env.example", "node_modules/pkg/.env", "vendor/lib/.env", "README.md"}
	for _, w := range wantAbsent {
		if rels[w] {
			t.Errorf("expected %s to be excluded, but it was found", w)
		}
	}

	if len(found) != len(wantPresent) {
		t.Errorf("got %d files, want %d: %+v", len(found), len(wantPresent), found)
	}
}
