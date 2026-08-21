package main

import "testing"

func TestParseEnvAssignments(t *testing.T) {
	// Fake, obviously-not-real values shaped like real secrets so the
	// key-name and placeholder heuristics can be exercised honestly.
	content := `# a comment
export DATABASE_URL=postgres://localhost/app

API_KEY=sk-thisisatestkeynotreal1234567890
QUOTED_TOKEN="ghp-testtoken-notreal-abcdef"
SINGLE_QUOTED_SECRET='alsonotreal12345'
EMPTY_SECRET=
PLACEHOLDER_PASSWORD=changeme
ANGLE_PRIVATE_KEY=<your-private-key-here>
1INVALID=nope
not a line at all
NORMAL_FLAG=true
`
	got := parseEnvAssignments(content)

	want := map[string]string{
		"DATABASE_URL":         "postgres://localhost/app",
		"API_KEY":              "sk-thisisatestkeynotreal1234567890",
		"QUOTED_TOKEN":         "ghp-testtoken-notreal-abcdef",
		"SINGLE_QUOTED_SECRET": "alsonotreal12345",
		"EMPTY_SECRET":         "",
		"PLACEHOLDER_PASSWORD": "changeme",
		"ANGLE_PRIVATE_KEY":    "<your-private-key-here>",
		"NORMAL_FLAG":          "true",
	}

	if len(got) != len(want) {
		t.Fatalf("got %d assignments, want %d: %+v", len(got), len(want), got)
	}
	for _, a := range got {
		wv, ok := want[a.Key]
		if !ok {
			t.Errorf("unexpected key parsed: %s", a.Key)
			continue
		}
		if a.Value != wv {
			t.Errorf("key %s: got value %q, want %q", a.Key, a.Value, wv)
		}
	}

	if _, present := want["1INVALID"]; present {
		t.Fatal("test setup bug")
	}
}

func TestLooksSecretKey(t *testing.T) {
	cases := map[string]bool{
		"API_KEY":        true,
		"DB_PASSWORD":    true,
		"STRIPE_SECRET":  true,
		"AUTH_TOKEN":     true,
		"PRIVATE_KEY":    true,
		"AWS_CREDENTIAL": true,
		"secret_lower":   true,
		"DATABASE_URL":   false,
		"NORMAL_FLAG":    false,
		"PORT":           false,
	}
	for key, want := range cases {
		if got := looksSecretKey(key); got != want {
			t.Errorf("looksSecretKey(%q) = %v, want %v", key, got, want)
		}
	}
}

func TestLooksLikePlaceholder(t *testing.T) {
	cases := map[string]bool{
		"":                                   true,
		"xxx":                                true,
		"XXXXXXXX":                           true,
		"changeme":                           true,
		"CHANGEME":                           true,
		"your-key-here":                      true,
		"<your-secret>":                      true,
		"placeholder":                        true,
		"sk-thisisatestkeynotreal1234567890": false,
		"ghp-testtoken-notreal-abcdef":       false,
		"postgres://localhost/app":           false,
	}
	for value, want := range cases {
		if got := looksLikePlaceholder(value); got != want {
			t.Errorf("looksLikePlaceholder(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestFindSecretLikeAssignments(t *testing.T) {
	assignments := []envAssignment{
		{Key: "API_KEY", Value: "sk-thisisatestkeynotreal1234567890"},
		{Key: "DB_PASSWORD", Value: "changeme"}, // placeholder, excluded
		{Key: "PORT", Value: "8080"},            // not secret-shaped key
		{Key: "AUTH_TOKEN", Value: "notreal-testtoken-987654321"},
	}
	got := findSecretLikeAssignments(assignments)
	if len(got) != 2 {
		t.Fatalf("got %d secret-like assignments, want 2: %+v", len(got), got)
	}
	keys := map[string]bool{}
	for _, a := range got {
		keys[a.Key] = true
	}
	if !keys["API_KEY"] || !keys["AUTH_TOKEN"] {
		t.Errorf("expected API_KEY and AUTH_TOKEN to be flagged, got %+v", got)
	}
}
