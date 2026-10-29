package agent

import (
	"context"
	"fmt"

	"github.com/Autumn-27/artex/agent/payloads"
)

// PolyAttempt represents one tool call attempt and its result.
type PolyAttempt struct {
	ToolName    string
	Args        map[string]any
	RawResult   string
	Rejection   RejectionType
	MutationGen int
}

// PolyEngine intercepts rejected tool results and generates mutated payloads.
type PolyEngine struct {
	MaxGens int // maximum mutation generations before giving up
}

// NewPolyEngine returns a PolyEngine with sensible defaults.
func NewPolyEngine() *PolyEngine {
	return &PolyEngine{MaxGens: 5}
}

// Mutate receives a rejected attempt and returns a new approach.
// Layer 1 (deterministic): WAFFLED mechanical mutators.
// Layer 2 (would be LLM): only when layer 1 exhausts without success.
func (p *PolyEngine) Mutate(ctx context.Context, attempt PolyAttempt) (PolyAttempt, error) {
	if attempt.Rejection == RejectionScope {
		return PolyAttempt{}, fmt.Errorf("scope rejection: not retrying")
	}
	if attempt.MutationGen >= p.MaxGens {
		return PolyAttempt{}, fmt.Errorf("max mutations (%d) reached", p.MaxGens)
	}

	mutated := attempt
	mutated.MutationGen++

	// copy args so we don't mutate the original map
	mutated.Args = make(map[string]any, len(attempt.Args))
	for k, v := range attempt.Args {
		mutated.Args[k] = v
	}

	// Find payload-like args to mutate
	for k, v := range attempt.Args {
		if s, ok := v.(string); ok && len(s) > 0 {
			mutated.Args[k] = p.applyWAFFLED(s, attempt.MutationGen, attempt.Rejection)
		}
	}
	return mutated, nil
}

// applyWAFFLED applies the deterministic WAFFLED mutation pipeline.
// W-Whitespace, A-Alternate encoding, F-Fragment, F2-Function wrap, L-Line comments, E-Encoding, D-Deadcode
func (p *PolyEngine) applyWAFFLED(payload string, gen int, rejection RejectionType) string {
	switch gen % 7 {
	case 0: // W — Whitespace variation
		return payloads.WhitespaceVariant(payload)
	case 1: // A — Alternate encoding
		return payloads.AlternateEncoding(payload)
	case 2: // F — Fragment substitution
		return payloads.FragmentSubstitute(payload)
	case 3: // F2 — Function wrapping
		return payloads.FunctionWrap(payload)
	case 4: // L — Line comment injection
		return payloads.CommentInject(payload)
	case 5: // E — Double encoding
		return payloads.DoubleEncode(payload)
	case 6: // D — Deadcode injection
		return payloads.DeadcodeInject(payload)
	}
	return payload
}
