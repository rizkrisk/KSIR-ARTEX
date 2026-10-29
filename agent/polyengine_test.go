package agent

import (
	"context"
	"testing"

	actool "github.com/Autumn-27/norma/tool"
)

func TestPolyEngineStopsOnScope(t *testing.T) {
	pe := NewPolyEngine()
	attempt := PolyAttempt{
		ToolName:  "test_tool",
		Args:      map[string]any{"payload": "test"},
		RawResult: "out of scope",
		Rejection: RejectionScope,
	}
	_, err := pe.Mutate(context.Background(), attempt)
	if err == nil {
		t.Fatal("expected error for scope rejection, got nil")
	}
}

func TestPolyEngineMutatesWAFRejection(t *testing.T) {
	pe := NewPolyEngine()
	attempt := PolyAttempt{
		ToolName:    "deploy_payload",
		Args:        map[string]any{"payload_body": "SELECT * FROM users"},
		RawResult:   "WAF blocked",
		Rejection:   RejectionWAF,
		MutationGen: 0,
	}
	result, err := pe.Mutate(context.Background(), attempt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Args["payload_body"] == attempt.Args["payload_body"] {
		t.Fatal("expected payload to be mutated, got same payload")
	}
	if result.MutationGen != 1 {
		t.Fatalf("expected MutationGen=1, got %d", result.MutationGen)
	}
}

func TestPolyEngineDeterministicFirst(t *testing.T) {
	pe := NewPolyEngine()
	attempt := PolyAttempt{
		ToolName:    "deploy_payload",
		Args:        map[string]any{"payload_body": "eval(cmd)"},
		Rejection:   RejectionWAF,
		MutationGen: 0,
	}
	result, err := pe.Mutate(context.Background(), attempt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have been mutated by deterministic WAFFLED layer
	if result.MutationGen != 1 {
		t.Fatalf("expected MutationGen=1, got %d", result.MutationGen)
	}
}

func TestPolyEngineMaxGens(t *testing.T) {
	pe := NewPolyEngine()
	attempt := PolyAttempt{
		ToolName:    "deploy_payload",
		Args:        map[string]any{"payload_body": "test"},
		Rejection:   RejectionWAF,
		MutationGen: pe.MaxGens, // already at limit
	}
	_, err := pe.Mutate(context.Background(), attempt)
	if err == nil {
		t.Fatal("expected error when MaxGens reached, got nil")
	}
}

func TestParseRejectionWAF(t *testing.T) {
	cases := []struct {
		text string
		want RejectionType
	}{
		{"WAF blocked your request", RejectionWAF},
		{"Access denied by Cloudflare", RejectionWAF},
		{"syntax error near unexpected", RejectionSyntax},
		{"401 unauthorized", RejectionAuth},
		{"connection timed out", RejectionTimeout},
		{"out of scope", RejectionScope},
		{"some random error", RejectionUnknown},
	}
	for _, c := range cases {
		r := ParseRejection(actool.Text(c.text))
		if r != c.want {
			t.Errorf("ParseRejection(%q) = %q, want %q", c.text, r, c.want)
		}
	}
}
