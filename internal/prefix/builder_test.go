package prefix

import (
	"context"
	"strings"
	"testing"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/config"
)

func testByteStableConfig() config.ByteStableConfig {
	return config.ByteStableConfig{
		NormalizeLineEndings:   "lf",
		SortToolSchemas:        true,
		DisallowDynamicContent: true,
	}
}

func TestBuildDeterministic(t *testing.T) {
	bs := testByteStableConfig()
	builder := NewImmutablePrefixBuilder(bs)

	req := BuildRequest{
		Version:      1,
		SystemPrompt: []byte("You are a helpful assistant."),
		CodingRules:  []byte("Use tabs for indentation."),
		ToolSchemas: []ToolSchema{
			{Name: "read_file", Bytes: []byte(`{"name":"read_file","description":"Read a file"}`)},
			{Name: "write_file", Bytes: []byte(`{"name":"write_file","description":"Write a file"}`)},
		},
		Sources: []Source{
			{Name: "system_prompt", Kind: SourceKindStaticFile, Path: "prompts/system.md", Required: true},
			{Name: "tool_schema", Kind: SourceKindGeneratedSchema, Path: "schemas/tools.json", Required: true},
		},
	}

	var lastHash string
	for i := 0; i < 10; i++ {
		doc, err := builder.Build(context.Background(), req)
		if err != nil {
			t.Fatalf("Build() iteration %d error: %v", i, err)
		}
		if lastHash != "" && doc.SHA256 != lastHash {
			t.Fatalf("Build() not deterministic: iteration %d hash %q != previous %q", i, doc.SHA256, lastHash)
		}
		lastHash = doc.SHA256
	}
}

func TestBuildSortsToolSchemas(t *testing.T) {
	bs := testByteStableConfig()
	builder := NewImmutablePrefixBuilder(bs)

	req := BuildRequest{
		Version:      1,
		SystemPrompt: []byte("system"),
		ToolSchemas: []ToolSchema{
			{Name: "zebra", Bytes: []byte(`{"name":"zebra"}`)},
			{Name: "alpha", Bytes: []byte(`{"name":"alpha"}`)},
			{Name: "middle", Bytes: []byte(`{"name":"middle"}`)},
		},
	}

	doc, err := builder.Build(context.Background(), req)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// The assembled bytes should have alpha before middle before zebra
	content := string(doc.Bytes)
	alphaIdx := indexOf(content, `"alpha"`)
	middleIdx := indexOf(content, `"middle"`)
	zebraIdx := indexOf(content, `"zebra"`)
	if alphaIdx >= middleIdx || middleIdx >= zebraIdx {
		t.Errorf("Tool schemas not sorted: alpha=%d, middle=%d, zebra=%d", alphaIdx, middleIdx, zebraIdx)
	}
}

func TestBuildNormalizesLineEndings(t *testing.T) {
	bs := testByteStableConfig()
	builder := NewImmutablePrefixBuilder(bs)

	req := BuildRequest{
		Version:      1,
		SystemPrompt: []byte("hello\r\nworld\r\n"),
	}

	doc, err := builder.Build(context.Background(), req)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// No CR should remain
	for _, b := range doc.Bytes {
		if b == '\r' {
			t.Errorf("Build() output contains CR byte")
		}
	}
}

func TestBuildRejectsDynamicSourceKind(t *testing.T) {
	bs := testByteStableConfig()
	builder := NewImmutablePrefixBuilder(bs)

	req := BuildRequest{
		Version:      1,
		SystemPrompt: []byte("system"),
		Sources: []Source{
			{Name: "dynamic_thing", Kind: "dynamic", Path: "dynamic/data", Required: true},
		},
	}

	_, err := builder.Build(context.Background(), req)
	if err == nil {
		t.Fatal("Build() should reject dynamic source kind")
	}
}

func TestBuildEmptyToolSchemas(t *testing.T) {
	bs := testByteStableConfig()
	builder := NewImmutablePrefixBuilder(bs)

	req := BuildRequest{
		Version:      1,
		SystemPrompt: []byte("system"),
		ToolSchemas:  nil,
	}

	doc, err := builder.Build(context.Background(), req)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	if doc.SHA256 == "" {
		t.Error("Build() produced empty hash")
	}
}

func TestBuildTokenEstimate(t *testing.T) {
	bs := testByteStableConfig()
	builder := NewImmutablePrefixBuilder(bs)

	req := BuildRequest{
		Version:      1,
		SystemPrompt: []byte("system prompt text"),
	}

	doc, err := builder.Build(context.Background(), req)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	if doc.TokenEstimate != EstimateTokens(doc.Bytes) {
		t.Errorf("TokenEstimate = %d, want EstimateTokens(Bytes) = %d", doc.TokenEstimate, EstimateTokens(doc.Bytes))
	}
}

func TestFingerprintMatchesBuild(t *testing.T) {
	bs := testByteStableConfig()
	builder := NewImmutablePrefixBuilder(bs)

	req := BuildRequest{
		Version:      1,
		SystemPrompt: []byte("system"),
		ToolSchemas: []ToolSchema{
			{Name: "tool", Bytes: []byte(`{"name":"tool"}`)},
		},
	}

	doc, err := builder.Build(context.Background(), req)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	fp, err := builder.Fingerprint(context.Background(), req)
	if err != nil {
		t.Fatalf("Fingerprint() error: %v", err)
	}

	if fp.SHA256 != doc.SHA256 {
		t.Errorf("Fingerprint SHA256 = %q, want Build SHA256 = %q", fp.SHA256, doc.SHA256)
	}
	if fp.Version != doc.Version {
		t.Errorf("Fingerprint Version = %d, want Build Version = %d", fp.Version, doc.Version)
	}
}

func TestBuildDynamicContentNotInPrefix(t *testing.T) {
	bs := testByteStableConfig()
	builder := NewImmutablePrefixBuilder(bs)

	// Even if someone passes dynamic-looking content in the system prompt,
	// the builder should not inject timestamps, session IDs, or random values.
	req := BuildRequest{
		Version:      1,
		SystemPrompt: []byte("You are a coding assistant."),
		CodingRules:  []byte("Follow project conventions."),
		ToolSchemas:  nil,
	}

	doc1, _ := builder.Build(context.Background(), req)
	doc2, _ := builder.Build(context.Background(), req)

	if doc1.SHA256 != doc2.SHA256 {
		t.Error("Build() produced different hashes for identical input → dynamic content may have leaked in")
	}
}

// helper: find first index of substr in s
func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// Test 12: Empty CodingRules + non-empty ToolSchemas concatenation behavior
func TestBuildEmptyCodingRulesWithToolSchemas(t *testing.T) {
	bs := testByteStableConfig()
	builder := NewImmutablePrefixBuilder(bs)

	req := BuildRequest{
		Version:      1,
		SystemPrompt: []byte("system prompt"),
		CodingRules:  nil, // empty
		ToolSchemas: []ToolSchema{
			{Name: "tool_a", Bytes: []byte(`{"name":"tool_a"}`)},
			{Name: "tool_b", Bytes: []byte(`{"name":"tool_b"}`)},
		},
	}

	doc, err := builder.Build(context.Background(), req)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// The output should have system_prompt content followed by a newline
	// separator and then tool schemas (no double newline from empty coding_rules)
	content := string(doc.Bytes)

	// System prompt should be at the start
	if !strings.HasPrefix(content, "system prompt") {
		t.Errorf("Output should start with system prompt, got: %q", content[:min(30, len(content))])
	}

	// Tool schemas should appear after system prompt with exactly one \n separator
	sysEnd := len("system prompt")
	if content[sysEnd] != '\n' {
		t.Errorf("Expected newline after system prompt, got %q", content[sysEnd:sysEnd+1])
	}

	// tool_a should appear before tool_b (sorted)
	toolAIdx := indexOf(content, "tool_a")
	toolBIdx := indexOf(content, "tool_b")
	if toolAIdx >= toolBIdx {
		t.Errorf("tool_a (idx=%d) should appear before tool_b (idx=%d)", toolAIdx, toolBIdx)
	}

	// No double newline from empty CodingRules section
	if strings.Contains(content, "\n\n") {
		t.Errorf("Output should not contain double newlines from empty CodingRules: %q", content)
	}

	// Hash should be deterministic
	doc2, _ := builder.Build(context.Background(), req)
	if doc.SHA256 != doc2.SHA256 {
		t.Error("Build() not deterministic for empty CodingRules + ToolSchemas")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
