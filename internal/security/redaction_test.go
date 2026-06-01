package security

import (
	"strings"
	"testing"
)

func TestSanitizeTextOpenAIKey(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard openai key",
			input: "Using key sk-abcdefghijklmnopqrstuvwxyz",
			want:  "Using key sk-****wxyz",
		},
		{
			name:  "openai key in url",
			input: "https://api.openai.com/v1?key=sk-abcdefghijklmnopqrstuvwxyz",
			want:  "https://api.openai.com/v1?key=sk-****wxyz",
		},
		{
			name:  "short key not redacted",
			input: "sk-short",
			want:  "sk-short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeText(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeTextMiMoKey(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard mimo key",
			input: "Using key tp-cabcdefghijklmnopqrstuvwxyz",
			want:  "Using key tp-c****wxyz",
		},
		{
			name:  "mimo key in value",
			input: "value is tp-cabcdefghijklmnopqrstuvwxyz here",
			want:  "value is tp-c****wxyz here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeText(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeTextBearerToken(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standalone bearer token",
			input: "Bearer abcdefghijklmnopqrstuvwxyz",
			want:  "Bearer ****wxyz",
		},
		{
			name:  "bearer in header",
			input: "Authorization: Bearer abcdefghijklmnopqrstuvwxyz",
			want:  "Authorization: ****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeText(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeTextAuthHeader(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "basic auth header",
			input: "Authorization: Basic dXNlcjpwYXNz",
			want:  "Authorization: ****",
		},
		{
			name:  "bearer auth header",
			input: "Authorization: Bearer abcdefghijklmnopqrstuvwxyz",
			want:  "Authorization: ****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeText(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeTextCookie(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "session cookie",
			input: "Cookie: sessionid=abcdefghijklmnopqrstuvwxyz",
			want:  "Cookie: ****",
		},
		{
			name:  "multiple cookies",
			input: "Cookie: sessionid=abc; token=xyz",
			want:  "Cookie: ****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeText(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeTextJWT(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(string) bool
		desc  string
	}{
		{
			name:  "standard jwt",
			input: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			check: func(s string) bool {
				return strings.Contains(s, "****.****.") && !strings.Contains(s, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9")
			},
			desc: "should redact JWT segments",
		},
		{
			name:  "jwt standalone",
			input: "jwt: abcdefghijklmnopqrst.abcdefghijklmnopqrst.abcdefghijklmnopqrst",
			check: func(s string) bool { return strings.Contains(s, "****.****.") },
			desc:  "should redact three-segment tokens",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeText(tt.input)
			if !tt.check(got) {
				t.Errorf("SanitizeText(%q) = %q, %s", tt.input, got, tt.desc)
			}
		})
	}
}

func TestSanitizeTextGenericSecret(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "api_key assignment",
			input: "api_key=abcdefghijklmnopqrstuvwxyz",
			want:  "api_key=<redacted>",
		},
		{
			name:  "apikey assignment",
			input: "apikey=abcdefghijklmnopqrstuvwxyz",
			want:  "apikey=<redacted>",
		},
		{
			name:  "token assignment",
			input: "token=abcdefghijklmnopqrstuvwxyz",
			want:  "token=<redacted>",
		},
		{
			name:  "secret assignment",
			input: "secret=abcdefghijklmnopqrstuvwxyz",
			want:  "secret=<redacted>",
		},
		{
			name:  "password assignment",
			input: "password=abcdefghijklmnopqrstuvwxyz",
			want:  "password=<redacted>",
		},
		{
			name:  "with spaces",
			input: "api_key = abcdefghijklmnop",
			want:  "api_key=<redacted>",
		},
		{
			name:  "colon separator",
			input: "api_key: abcdefghijklmnop",
			want:  "api_key=<redacted>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeText(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeTextKnownSecret(t *testing.T) {
	secret := "my-super-secret-value-12345678"
	input := "Using value: my-super-secret-value-12345678"
	want := "Using value: <redacted>"

	got := SanitizeText(input, secret)
	if got != want {
		t.Errorf("SanitizeText(%q, %q) = %q, want %q", input, secret, got, want)
	}
}

func TestSanitizeTextRegisterKnownSecret(t *testing.T) {
	defer ClearKnownSecrets()

	secret := "registered-secret-value-12345678"
	RegisterKnownSecret(secret)

	input := "Value is registered-secret-value-12345678"
	want := "Value is <redacted>"

	got := SanitizeText(input)
	if got != want {
		t.Errorf("SanitizeText(%q) = %q, want %q", input, got, want)
	}
}

func TestSanitizeTextMixedContent(t *testing.T) {
	input := `curl -X POST https://api.openai.com/v1/chat/completions
Authorization: Bearer sk-abcdefghijklmnopqrstuvwxyz
Content-Type: application/json
api_key: sk-abcdefghijklmnopqrstuvwxyz`

	got := SanitizeText(input)

	if strings.Contains(got, "sk-abcdefghijklmnopqrstuvwxyz") {
		t.Errorf("SanitizeText leaked raw key: %q", got)
	}
}

func TestSanitizeTextAlreadyRedacted(t *testing.T) {
	input := "Key is <redacted> and token is sk-****abcd"
	want := "Key is <redacted> and token is sk-****abcd"

	got := SanitizeText(input)
	if got != want {
		t.Errorf("SanitizeText(%q) = %q, want %q", input, got, want)
	}
}

func TestSanitizeTextEmptyInput(t *testing.T) {
	got := SanitizeText("")
	if got != "" {
		t.Errorf("SanitizeText(\"\") = %q, want \"\"", got)
	}
}

func TestSanitizeOutput(t *testing.T) {
	input := "API response with key sk-abcdefghijklmnopqrstuvwxyz"
	want := "API response with key sk-****wxyz"

	got := SanitizeOutput(input)
	if got != want {
		t.Errorf("SanitizeOutput(%q) = %q, want %q", input, got, want)
	}
}

func TestSanitizeMap(t *testing.T) {
	input := map[string]string{
		"api_key":  "sk-abcdefghijklmnopqrstuvwxyz",
		"model":    "gpt-4",
		"base_url": "https://api.openai.com/v1",
	}

	got := SanitizeMap(input)

	if input["api_key"] != "sk-abcdefghijklmnopqrstuvwxyz" {
		t.Error("SanitizeMap modified input map")
	}

	if got["api_key"] == "sk-abcdefghijklmnopqrstuvwxyz" {
		t.Errorf("SanitizeMap did not redact api_key: %q", got["api_key"])
	}

	if got["model"] != "gpt-4" {
		t.Errorf("SanitizeMap changed model: %q", got["model"])
	}
}

func TestSanitizeMapNilInput(t *testing.T) {
	got := SanitizeMap(nil)
	if got != nil {
		t.Errorf("SanitizeMap(nil) = %v, want nil", got)
	}
}

func TestSanitizeEventMap(t *testing.T) {
	input := map[string]interface{}{
		"tool_name": "file_read",
		"api_key":   "sk-abcdefghijklmnopqrstuvwxyz",
		"nested": map[string]interface{}{
			"secret": "sk-nestedabcdefghijklmnopqrstuvwxyz",
			"safe":   "safe-value",
		},
		"slices": []interface{}{
			"sk-abcdefghijklmnopqrstuvwxyz",
			"safe-value",
		},
		"string_slice": []string{
			"tp-cabcdefghijklmnopqrstuvwxyz",
			"safe-value",
		},
	}

	got := SanitizeEventMap(input)

	if input["api_key"] != "sk-abcdefghijklmnopqrstuvwxyz" {
		t.Error("SanitizeEventMap modified input map")
	}

	if got["api_key"] == "sk-abcdefghijklmnopqrstuvwxyz" {
		t.Errorf("SanitizeEventMap did not redact api_key: %q", got["api_key"])
	}

	if got["tool_name"] != "file_read" {
		t.Errorf("SanitizeEventMap changed tool_name: %q", got["tool_name"])
	}

	nested, ok := got["nested"].(map[string]interface{})
	if !ok {
		t.Fatal("SanitizeEventMap nested is not a map")
	}
	if nested["secret"] == "sk-nestedabcdefghijklmnopqrstuvwxyz" {
		t.Errorf("SanitizeEventMap did not redact nested secret: %q", nested["secret"])
	}
	if nested["safe"] != "safe-value" {
		t.Errorf("SanitizeEventMap changed nested safe: %q", nested["safe"])
	}

	slices, ok := got["slices"].([]interface{})
	if !ok {
		t.Fatal("SanitizeEventMap slices is not a slice")
	}
	if slices[0] == "sk-abcdefghijklmnopqrstuvwxyz" {
		t.Errorf("SanitizeEventMap did not redact slice[0]: %q", slices[0])
	}
	if slices[1] != "safe-value" {
		t.Errorf("SanitizeEventMap changed slice[1]: %q", slices[1])
	}

	strSlices, ok := got["string_slice"].([]string)
	if !ok {
		t.Fatal("SanitizeEventMap string_slice is not a []string")
	}
	if strSlices[0] == "tp-cabcdefghijklmnopqrstuvwxyz" {
		t.Errorf("SanitizeEventMap did not redact string_slice[0]: %q", strSlices[0])
	}
	if strSlices[1] != "safe-value" {
		t.Errorf("SanitizeEventMap changed string_slice[1]: %q", strSlices[1])
	}
}

func TestSanitizeEventMapNilInput(t *testing.T) {
	got := SanitizeEventMap(nil)
	if got != nil {
		t.Errorf("SanitizeEventMap(nil) = %v, want nil", got)
	}
}

func TestSanitizeEventMapDoesNotMutateInput(t *testing.T) {
	input := map[string]interface{}{
		"api_key": "sk-abcdefghijklmnopqrstuvwxyz",
		"nested": map[string]interface{}{
			"secret": "sk-nestedabcdefghijklmnopqrstuvwxyz",
		},
	}

	origKey := input["api_key"]
	nested := input["nested"].(map[string]interface{})
	origSecret := nested["secret"]

	_ = SanitizeEventMap(input)

	if input["api_key"] != origKey {
		t.Error("SanitizeEventMap mutated input.api_key")
	}
	if nested["secret"] != origSecret {
		t.Error("SanitizeEventMap mutated input.nested.secret")
	}
}

func TestNoSecretLeak(t *testing.T) {
	testCases := []struct {
		name   string
		input  string
		secret string
	}{
		{
			name:   "openai key",
			input:  "Value: sk-abcdefghijklmnopqrstuvwxyz",
			secret: "sk-abcdefghijklmnopqrstuvwxyz",
		},
		{
			name:   "mimo key",
			input:  "Value: tp-cabcdefghijklmnopqrstuvwxyz",
			secret: "tp-cabcdefghijklmnopqrstuvwxyz",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeText(tc.input)
			if strings.Contains(got, tc.secret) {
				t.Errorf("SanitizeText leaked secret %q in output %q", tc.secret, got)
			}
		})
	}
}

func TestCookieRedaction(t *testing.T) {
	input := "Cookie: sessionid=abcdefghijklmnopqrstuvwxyz"
	got := SanitizeText(input)

	if strings.Contains(got, "abcdefghijklmnopqrstuvwxyz") {
		t.Errorf("SanitizeText leaked cookie: %q", got)
	}
}
