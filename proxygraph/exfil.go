package proxygraph

import (
	"encoding/base64"
	"fmt"
)

// Fragment splits data into base64-encoded chunks, one per field name.
// Fields cycle if there is more data than field slots.
// The returned map keys are field names; values are base64url-encoded chunks.
func Fragment(data []byte, fields []string) map[string]string {
	if len(fields) == 0 || len(data) == 0 {
		return map[string]string{}
	}
	// Distribute bytes evenly across fields.
	chunkSize := (len(data) + len(fields) - 1) / len(fields)
	out := make(map[string]string, len(fields))
	for i, field := range fields {
		start := i * chunkSize
		if start >= len(data) {
			out[field] = ""
			continue
		}
		end := start + chunkSize
		if end > len(data) {
			end = len(data)
		}
		out[field] = base64.URLEncoding.EncodeToString(data[start:end])
	}
	return out
}

// Reassemble reconstructs data from fragments in the order of fields.
// Missing fields are skipped; empty strings produce no bytes.
func Reassemble(fragments map[string]string, fields []string) []byte {
	var out []byte
	for _, field := range fields {
		enc, ok := fragments[field]
		if !ok || enc == "" {
			continue
		}
		decoded, err := base64.URLEncoding.DecodeString(enc)
		if err != nil {
			// surface decoding errors as a sentinel comment; caller can detect
			out = append(out, []byte(fmt.Sprintf("[ERR:%s]", field))...)
			continue
		}
		out = append(out, decoded...)
	}
	return out
}
