package prefix

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// NormalizeLineEndings replaces CRLF (\r\n) and standalone CR (\r) with LF (\n).
func NormalizeLineEndings(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	var b []byte
	for i := 0; i < len(data); i++ {
		if data[i] == '\r' {
			if i+1 < len(data) && data[i+1] == '\n' {
				// CRLF -> LF, skip the CR
				continue
			}
			// standalone CR -> LF
			b = append(b, '\n')
			continue
		}
		b = append(b, data[i])
	}
	return b
}

// CanonicalText normalizes line endings to LF, trims trailing whitespace
// from each line, and removes trailing blank lines.
func CanonicalText(data []byte) []byte {
	normalized := NormalizeLineEndings(data)
	if len(normalized) == 0 {
		return normalized
	}

	lines := strings.Split(string(normalized), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}

	// Remove trailing blank lines
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return []byte(strings.Join(lines, "\n"))
}

// CanonicalJSON parses JSON and re-serializes it with sorted object keys.
// If parsing fails, the original data is returned unchanged.
func CanonicalJSON(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return data
	}

	canonical, err := json.Marshal(sortKeys(v))
	if err != nil {
		return data
	}
	return canonical
}

// sortKeys recursively sorts map keys in nested JSON structures.
// Go's json.Marshal already sorts map keys, but this ensures
// the input is fully decoded and re-encoded.
func sortKeys(v any) any {
	switch val := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		sorted := make(map[string]any, len(val))
		for _, k := range keys {
			sorted[k] = sortKeys(val[k])
		}
		return sorted
	case []any:
		for i, elem := range val {
			val[i] = sortKeys(elem)
		}
		return val
	default:
		return v
	}
}

// CanonicalTools sorts tool schemas by Name, canonicalizes each schema's
// JSON bytes, and concatenates them with a single newline separator.
func CanonicalTools(schemas []ToolSchema) []byte {
	if len(schemas) == 0 {
		return nil
	}

	// Sort by name
	sorted := make([]ToolSchema, len(schemas))
	copy(sorted, schemas)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	// Canonicalize each schema and concatenate
	parts := make([][]byte, 0, len(sorted))
	for _, s := range sorted {
		canonical := CanonicalJSON(s.Bytes)
		parts = append(parts, canonical)
	}

	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result = append(result, '\n')
		result = append(result, parts[i]...)
	}
	return result
}

// StableHash returns the hex-encoded SHA-256 hash of the data.
// The output is always 64 lowercase hex characters.
func StableHash(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// EstimateTokens returns a rough token count estimate based on
// the heuristic of approximately 4 characters per token.
func EstimateTokens(data []byte) int {
	return len(data) / 4
}
