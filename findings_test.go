package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderTextEmpty(t *testing.T) {
	got := RenderText(nil)
	if got != "dotenvleak: no issues found" {
		t.Errorf("got %q", got)
	}
}

func TestRenderTextAndOrdering(t *testing.T) {
	findings := []Finding{
		newDL003("b/.env"),
		newDL001("a/.env", []string{"API_KEY"}),
	}
	text := RenderText(findings)
	if !strings.HasPrefix(text, "[DL001]") {
		t.Errorf("expected a/.env (DL001) to sort first, got:\n%s", text)
	}
	if !strings.Contains(text, "2 finding(s)") {
		t.Errorf("expected finding count in output, got:\n%s", text)
	}
}

func TestRenderJSON(t *testing.T) {
	findings := []Finding{newDL002("x/.env", []string{"SECRET_TOKEN"})}
	out, err := RenderJSON(findings)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []Finding
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("invalid JSON produced: %v\n%s", err, out)
	}
	if len(decoded) != 1 || decoded[0].Rule != DL002 {
		t.Errorf("unexpected decoded findings: %+v", decoded)
	}
}

func TestRenderJSONEmptyIsArray(t *testing.T) {
	out, err := RenderJSON(nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Errorf("expected empty JSON array, got %q", out)
	}
}
