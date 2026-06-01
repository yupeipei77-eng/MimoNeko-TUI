// Package security provides unified secret redaction for display-layer output.
//
// This package implements display-layer redaction only. It does NOT:
//   - Encrypt or modify real data
//   - Change tool execution behavior
//   - Modify EventStore data structures
//   - Implement sandbox or approval logic
//
// All functions return redacted copies; input is never modified.
package security

import (
	"regexp"
	"strings"
)

// SuffixLength is the number of trailing characters to preserve for high-entropy tokens.
const SuffixLength = 4

// RedactedPlaceholder is the generic placeholder for sensitive values.
const RedactedPlaceholder = "<redacted>"

// regex patterns for high-entropy tokens
var (
	// OpenAI API Key: sk-xxxxxxxxxxxxxxxx
	openAIKeyPattern = regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`)

	// MiMo API Key: tp-cxxxxxxxxxxxxxxxx
	mimoKeyPattern = regexp.MustCompile(`tp-c[a-zA-Z0-9]{20,}`)

	// Bearer token: Bearer xxxxxxx (must be before Authorization header)
	bearerTokenPattern = regexp.MustCompile(`Bearer\s+[a-zA-Z0-9._\-]{20,}`)

	// Authorization header with value
	authHeaderPattern = regexp.MustCompile(`(?i)(Authorization:\s*)[^\n]+`)

	// Cookie header with value
	cookieHeaderPattern = regexp.MustCompile(`(?i)(Cookie:\s*)[^\n]+`)

	// JWT: xxxxx.yyyyy.zzzzz (three base64url segments)
	jwtPattern = regexp.MustCompile(`[a-zA-Z0-9_\-]{20,}\.[a-zA-Z0-9_\-]{20,}\.[a-zA-Z0-9_\-]{20,}`)

	// Generic secret patterns: key=value or key:value
	// Must be after specific patterns to avoid false positives
	// Only match when value is not empty and doesn't start with special patterns
	genericSecretPattern = regexp.MustCompile(`(?i)(api[_-]?key|apikey|token|secret|password|passwd|access[_-]?token|refresh[_-]?token)\s*[=:]\s*[a-zA-Z0-9_\-]{8,}`)
)

// known secret tokens (exact values to redact)
var knownSecrets []string

// RegisterKnownSecret adds a secret value to the global redaction list.
// This is used when a specific API key value is known (e.g., from config).
func RegisterKnownSecret(secret string) {
	secret = strings.TrimSpace(secret)
	if secret != "" && !containsSecret(knownSecrets, secret) {
		knownSecrets = append(knownSecrets, secret)
	}
}

// ClearKnownSecrets removes all registered known secrets.
// Useful for testing.
func ClearKnownSecrets() {
	knownSecrets = nil
}

// SanitizeText redacts sensitive patterns from text.
//
// Redaction rules (applied in order):
//  1. Known secrets: replaced with RedactedPlaceholder
//  2. OpenAI key (sk-xxx): sk-****abcd
//  3. MiMo key (tp-cxxx): tp-c****abcd
//  4. Authorization header: Authorization: ****
//  5. Cookie header: Cookie: ****
//  6. Bearer token: Bearer ****wxyz
//  7. JWT: ****.****.zzzz
//  8. Generic secrets (api_key=xxx): key=<redacted>
//
// Input is never modified. Returns a new string.
func SanitizeText(text string, extraSecrets ...string) string {
	if text == "" {
		return text
	}

	result := text

	// 1. Redact known secrets (exact values)
	allSecrets := append(knownSecrets, extraSecrets...)
	for _, secret := range allSecrets {
		secret = strings.TrimSpace(secret)
		if secret != "" && len(secret) >= 8 {
			result = strings.ReplaceAll(result, secret, RedactedPlaceholder)
		}
	}

	// 2. Redact API keys with suffix preservation
	result = redactWithSuffix(openAIKeyPattern, result, "sk-")
	result = redactWithSuffix(mimoKeyPattern, result, "tp-c")

	// 3. Redact headers (before Bearer/JWT to avoid conflicts)
	result = authHeaderPattern.ReplaceAllString(result, "Authorization: ****")
	result = cookieHeaderPattern.ReplaceAllString(result, "Cookie: ****")

	// 4. Redact Bearer tokens
	result = redactBearerToken(result)

	// 5. Redact JWT (preserve last segment suffix)
	result = redactJWT(result)

	// 6. Redact generic secrets
	result = redactGenericSecrets(result)

	return result
}

// SanitizeOutput redacts tool output for display.
// This is a convenience wrapper around SanitizeText for tool output.
func SanitizeOutput(output string) string {
	return SanitizeText(output)
}

// SanitizeMap creates a deep copy of the map with all string values redacted.
// The input map is never modified.
func SanitizeMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}

	result := make(map[string]string, len(input))
	for k, v := range input {
		result[k] = SanitizeText(v)
	}
	return result
}

// SanitizeEventMap creates a deep copy of an event map with sensitive values redacted.
// Handles nested maps and slices. The input is never modified.
func SanitizeEventMap(event map[string]interface{}) map[string]interface{} {
	if event == nil {
		return nil
	}

	result := make(map[string]interface{}, len(event))
	for k, v := range event {
		result[k] = sanitizeValue(v)
	}
	return result
}

// sanitizeValue recursively redacts values in nested structures.
func sanitizeValue(v interface{}) interface{} {
	switch val := v.(type) {
	case string:
		return SanitizeText(val)
	case map[string]interface{}:
		return SanitizeEventMap(val)
	case map[string]string:
		return SanitizeMap(val)
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, item := range val {
			result[i] = sanitizeValue(item)
		}
		return result
	case []string:
		result := make([]string, len(val))
		for i, item := range val {
			result[i] = SanitizeText(item)
		}
		return result
	default:
		return v
	}
}

// redactWithSuffix applies regex-based redaction preserving the last SuffixLength characters.
func redactWithSuffix(pattern *regexp.Regexp, text string, prefix string) string {
	return pattern.ReplaceAllStringFunc(text, func(match string) string {
		if len(match) <= SuffixLength {
			return RedactedPlaceholder
		}
		suffix := match[len(match)-SuffixLength:]
		return prefix + "****" + suffix
	})
}

// redactBearerToken redacts Bearer tokens while preserving the "Bearer " prefix.
func redactBearerToken(text string) string {
	return bearerTokenPattern.ReplaceAllStringFunc(text, func(match string) string {
		// Find the space after "Bearer"
		idx := strings.Index(match, " ")
		if idx < 0 {
			return "Bearer ****"
		}
		token := strings.TrimSpace(match[idx:])
		if len(token) <= SuffixLength {
			return "Bearer ****"
		}
		suffix := token[len(token)-SuffixLength:]
		return "Bearer ****" + suffix
	})
}

// redactJWT redacts JWT tokens, preserving the last segment's suffix.
func redactJWT(text string) string {
	return jwtPattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := strings.Split(match, ".")
		if len(parts) != 3 {
			return match
		}
		// Redact first two segments, preserve suffix of last
		lastSeg := parts[2]
		suffix := lastSeg
		if len(lastSeg) > SuffixLength {
			suffix = lastSeg[len(lastSeg)-SuffixLength:]
		}
		return "****.****." + suffix
	})
}

// redactGenericSecrets redacts key=value patterns for common secret key names.
func redactGenericSecrets(text string) string {
	return genericSecretPattern.ReplaceAllStringFunc(text, func(match string) string {
		// Find the key part (before = or :)
		for _, sep := range []string{"=", ":"} {
			idx := strings.Index(match, sep)
			if idx >= 0 {
				key := strings.TrimSpace(match[:idx])
				return key + "=" + RedactedPlaceholder
			}
		}
		return RedactedPlaceholder
	})
}

// containsSecret checks if a secret is already in the list.
func containsSecret(secrets []string, secret string) bool {
	for _, s := range secrets {
		if s == secret {
			return true
		}
	}
	return false
}
