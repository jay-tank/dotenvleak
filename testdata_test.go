package main

import (
	"os"
	"testing"
)

// TestTestdataFixtures exercises the secret-detection heuristic against the
// real fixture files checked into testdata/, so those fixtures are not
// just documentation but load-bearing test inputs.
func TestTestdataFixtures(t *testing.T) {
	leaky, err := os.ReadFile("testdata/leaky.env")
	if err != nil {
		t.Fatal(err)
	}
	secretLeaky := findSecretLikeAssignments(parseEnvAssignments(string(leaky)))
	if len(secretLeaky) == 0 {
		t.Errorf("expected testdata/leaky.env to contain secret-looking assignments")
	}

	clean, err := os.ReadFile("testdata/clean.env")
	if err != nil {
		t.Fatal(err)
	}
	secretClean := findSecretLikeAssignments(parseEnvAssignments(string(clean)))
	if len(secretClean) != 0 {
		t.Errorf("expected testdata/clean.env to contain no secret-looking assignments, got %+v", secretClean)
	}
}

func TestTestdataProtectiveGitignore(t *testing.T) {
	content, err := os.ReadFile("testdata/protective.gitignore")
	if err != nil {
		t.Fatal(err)
	}
	patterns := parseGitignore(string(content), "/repo")
	if !isIgnoredByChain(patterns, "/repo/.env") {
		t.Errorf("expected /repo/.env to be ignored by testdata/protective.gitignore")
	}
	if !isIgnoredByChain(patterns, "/repo/.env.local") {
		t.Errorf("expected /repo/.env.local to be ignored by testdata/protective.gitignore")
	}
	if isIgnoredByChain(patterns, "/repo/.env.example") {
		t.Errorf("expected /repo/.env.example to be re-included by the negation")
	}
}
