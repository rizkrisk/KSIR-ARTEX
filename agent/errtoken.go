package agent

import (
	"strings"

	actool "github.com/Autumn-27/norma/tool"
)

// RejectionType classifies why a tool call was rejected by the target.
type RejectionType string

const (
	RejectionWAF     RejectionType = "waf"
	RejectionSyntax  RejectionType = "syntax"
	RejectionAuth    RejectionType = "auth"
	RejectionTimeout RejectionType = "timeout"
	RejectionScope   RejectionType = "scope" // do not retry
	RejectionUnknown RejectionType = "unknown"
)

// ParseRejection reads a tool Result and classifies the type of rejection.
func ParseRejection(result actool.Result) RejectionType {
	text := result.Flatten()
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "waf") || strings.Contains(lower, "blocked") ||
		strings.Contains(lower, "cloudflare") || strings.Contains(lower, "modsecurity") ||
		strings.Contains(lower, "access denied") || strings.Contains(lower, "forbidden"):
		return RejectionWAF
	case strings.Contains(lower, "syntax error") || strings.Contains(lower, "parse error") ||
		strings.Contains(lower, "unexpected token"):
		return RejectionSyntax
	case strings.Contains(lower, "401") || strings.Contains(lower, "403") ||
		strings.Contains(lower, "authentication") || strings.Contains(lower, "unauthorized"):
		return RejectionAuth
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "timed out") ||
		strings.Contains(lower, "connection reset"):
		return RejectionTimeout
	case strings.Contains(lower, "out of scope") || strings.Contains(lower, "not authorized") ||
		strings.Contains(lower, "intercept"):
		return RejectionScope
	default:
		return RejectionUnknown
	}
}
