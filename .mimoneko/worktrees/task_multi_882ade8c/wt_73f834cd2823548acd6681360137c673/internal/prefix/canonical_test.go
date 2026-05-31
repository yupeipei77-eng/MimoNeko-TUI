package prefix

import (
	"strings"
	"testing"
)

func TestNormalizeLineEndings(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"already LF", "hello\nworld", "hello\nworld"},
		{"CRLF", "hello\r\nworld", "hello\nworld"},
		{"standalone CR", "hello\rworld", "hello\nworld"},
		{"mixed", "a\r\nb\rc\r\nd", "a\nb\nc\nd"},
		{"empty", "", ""},
		{"CRLF only", "\r\n", "\n"},
		{"multiple CRLF", "a\r\n\r\nb", "a\n\nb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeLineEndings([]byte(tt.input))
			if string(got) != tt.want {
				t.Errorf("NormalizeLineEndings(%q) = %q, want %q", tt.input, string(got), tt.want)
			}
		})
	}
}

func TestCanonicalText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"trailing spaces on lines",
			"hello   \nworld  \n",
			"hello\nworld",
		},
		{
			"trailing newlines",
			"hello\n\n\n",
			"hello",
		},
		{
			"CRLF plus trailing spaces",
			"hello   \r\nworld  \r\n\r\n",
			"hello\nworld",
		},
		{
			"empty",
			"",
			"",
		},
		{
			"only whitespace lines become empty after trim",
			"   \n   \n",
			"",
		},
		{
			"internal blank lines preserved",
			"hello\n\nworld\n",
			"hello\n\nworld",
		},
		{
			"tabs trimmed",
			"hello\t\nworld\t\n",
			"hello\nworld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanonicalText([]byte(tt.input))
			if string(got) != tt.want {
				t.Errorf("CanonicalText(%q) = %q, want %q", tt.input, string(got), tt.want)
			}
		})
	}
}

func TestCanonicalJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"unsorted keys",
			`{"z":1,"a":2}`,
			`{"a":2,"z":1}`,
		},
		{
			"nested objects sorted",
			`{"outer":{"z":1,"a":2}}`,
			`{"outer":{"a":2,"z":1}}`,
		},
		{
			"arrays preserve order",
			`[3,1,2]`,
			`[3,1,2]`,
		},
		{
			"empty object",
			`{}`,
			`{}`,
		},
		{
			"empty array",
			`[]`,
			`[]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanonicalJSON([]byte(tt.input))
			if string(got) != tt.want {
				t.Errorf("CanonicalJSON(%q) = %q, want %q", tt.input, string(got), tt.want)
			}
		})
	}

	t.Run("invalid JSON passes through", func(t *testing.T) {
		input := `{not valid json`
		got := CanonicalJSON([]byte(input))
		if string(got) != input {
			t.Errorf("CanonicalJSON(invalid) = %q, want original %q", string(got), input)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		got := CanonicalJSON(nil)
		if got != nil {
			t.Errorf("CanonicalJSON(nil) = %q, want nil", string(got))
		}
	})
}

func TestCanonicalTools(t *testing.T) {
	schemas := []ToolSchema{
		{Name: "zebra", Bytes: []byte(`{"description":"z tool","name":"zebra"}`)},
		{Name: "alpha", Bytes: []byte(`{"name":"alpha","description":"a tool"}`)},
		{Name: "middle", Bytes: []byte(`{"description":"m tool","name":"middle"}`)},
	}

	result := CanonicalTools(schemas)

	// Result should have schemas in alphabetical order by name
	resultStr := string(result)
	if !strings.HasPrefix(resultStr, `{"description":"a tool","name":"alpha"}`) {
		t.Errorf("CanonicalTools: first schema should be alpha, got %s", resultStr)
	}

	// Each schema should be canonicalized (sorted keys)
	lines := strings.Split(resultStr, "\n")
	if len(lines) != 3 {
		t.Fatalf("CanonicalTools: expected 3 lines, got %d", len(lines))
	}

	// Verify alphabetical order
	if !strings.Contains(lines[0], `"alpha"`) {
		t.Errorf("first line should contain alpha, got %s", lines[0])
	}
	if !strings.Contains(lines[1], `"middle"`) {
		t.Errorf("second line should contain middle, got %s", lines[1])
	}
	if !strings.Contains(lines[2], `"zebra"`) {
		t.Errorf("third line should contain zebra, got %s", lines[2])
	}

	t.Run("empty input", func(t *testing.T) {
		got := CanonicalTools(nil)
		if got != nil {
			t.Errorf("CanonicalTools(nil) = %q, want nil", string(got))
		}
	})
}

func TestStableHash(t *testing.T) {
	t.Run("same input same hash", func(t *testing.T) {
		data := []byte("hello world")
		h1 := StableHash(data)
		h2 := StableHash(data)
		if h1 != h2 {
			t.Errorf("StableHash not deterministic: %s != %s", h1, h2)
		}
	})

	t.Run("different input different hash", func(t *testing.T) {
		h1 := StableHash([]byte("hello"))
		h2 := StableHash([]byte("world"))
		if h1 == h2 {
			t.Errorf("StableHash collision: %s == %s", h1, h2)
		}
	})

	t.Run("empty input valid hex", func(t *testing.T) {
		h := StableHash([]byte{})
		if len(h) != 64 {
			t.Errorf("StableHash empty: length %d, want 64", len(h))
		}
	})

	t.Run("hash is lowercase hex", func(t *testing.T) {
		h := StableHash([]byte("test"))
		for _, c := range h {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("StableHash: unexpected char %c in %s", c, h)
			}
		}
	})
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"a", 0},
		{"abcd", 1},
		{"abcdefgh", 2},
	}

	for _, tt := range tests {
		got := EstimateTokens([]byte(tt.input))
		if got != tt.want {
			t.Errorf("EstimateTokens(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
