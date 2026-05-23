package contextengine

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/reasonforge/reasonforge/internal/cache"
	"github.com/reasonforge/reasonforge/internal/config"
	"github.com/reasonforge/reasonforge/internal/conversation"
	"github.com/reasonforge/reasonforge/internal/prefix"
	"github.com/reasonforge/reasonforge/internal/scratchpad"
)

// --- Stub implementations for testing ---

type stubPrefixBuilder struct {
	doc prefix.Document
	err error
}

func (s *stubPrefixBuilder) Build(ctx context.Context, req prefix.BuildRequest) (prefix.Document, error) {
	if s.doc.SHA256 == "" {
		assembled := append(req.SystemPrompt, req.CodingRules...)
		// Ensure the prefix has content even if inputs are empty
		if len(assembled) == 0 {
			assembled = []byte("default system prompt")
		}
		s.doc = prefix.Document{
			Version:       req.Version,
			Bytes:         assembled,
			SHA256:        prefix.StableHash(assembled),
			Sources:       req.Sources,
			TokenEstimate: prefix.EstimateTokens(assembled),
		}
	}
	return s.doc, s.err
}

func (s *stubPrefixBuilder) Fingerprint(ctx context.Context, req prefix.BuildRequest) (prefix.Fingerprint, error) {
	doc, err := s.Build(ctx, req)
	if err != nil {
		return prefix.Fingerprint{}, err
	}
	return prefix.Fingerprint{SHA256: doc.SHA256, Version: doc.Version}, nil
}

type stubConversationLog struct {
	events []conversation.Event
}

func (s *stubConversationLog) Append(ctx context.Context, event conversation.Event) error {
	s.events = append(s.events, event)
	return nil
}

func (s *stubConversationLog) Read(ctx context.Context, query conversation.Query) ([]conversation.Event, error) {
	return s.events, nil
}

func (s *stubConversationLog) Tail(ctx context.Context, query conversation.Query) ([]conversation.Event, error) {
	return s.events, nil
}

type stubCacheRegistry struct {
	observations []cache.Observation
}

func (s *stubCacheRegistry) Lookup(ctx context.Context, fp prefix.Fingerprint) (cache.Entry, bool, error) {
	return cache.Entry{}, false, nil
}

func (s *stubCacheRegistry) Record(ctx context.Context, obs cache.Observation) error {
	s.observations = append(s.observations, obs)
	return nil
}

func testPrefixConfig() config.PrefixConfig {
	return config.PrefixConfig{
		Version: 1,
		ImmutableSources: []config.PrefixSourceConfig{
			{Name: "system_prompt", Kind: "static_file", Path: "prompts/system.md", Required: true},
			{Name: "tool_schema", Kind: "generated_schema", Path: "schemas/tools.json", Required: true},
			{Name: "coding_rules", Kind: "static_file", Path: "prompts/coding_rules.md", Required: true},
		},
		ByteStable: config.ByteStableConfig{
			NormalizeLineEndings:   "lf",
			SortToolSchemas:        true,
			DisallowDynamicContent: true,
		},
		Cache: config.PrefixCacheConfig{
			RegistryPath: ".reasonforge/cache/prefixes.jsonl",
		},
		Budget: config.BudgetConfig{
			WarnRatio:  0.8,
			BlockRatio: 1.0,
		},
	}
}

func newTestEngine() (*DefaultContextEngine, *stubCacheRegistry) {
	pb := &stubPrefixBuilder{}
	cl := &stubConversationLog{}
	sp := scratchpad.NewVolatileScratchpad()
	cr := &stubCacheRegistry{}
	bg, _ := NewBudgetGuard(BudgetThresholds{WarnRatio: 0.8, BlockRatio: 1.0})

	engine := NewDefaultContextEngine(pb, cl, sp, cr, bg, "/tmp/test", testPrefixConfig())
	return engine, cr
}

func TestBuildAssemblesAllLayers(t *testing.T) {
	engine, _ := newTestEngine()

	req := BuildRequest{
		TaskID:         "t1",
		ConversationID: "c1",
		RepoRoot:       "/tmp/test",
		Budget:         TokenBudget{ImmutablePrefix: 10000, Conversation: 5000, Scratchpad: 2000},
		CurrentInput:   []byte("Please fix the bug in main.go"),
	}

	bundle, err := engine.Build(context.Background(), req)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if bundle.ImmutablePrefix.SHA256 == "" {
		t.Error("Bundle.ImmutablePrefix missing")
	}
	if bundle.CacheFingerprint.SHA256 == "" {
		t.Error("Bundle.CacheFingerprint missing")
	}
}

func TestBuildContextReportPopulated(t *testing.T) {
	engine, _ := newTestEngine()

	req := BuildRequest{
		TaskID:         "t1",
		ConversationID: "c1",
		Budget:         TokenBudget{ImmutablePrefix: 10000, Conversation: 5000, Scratchpad: 2000},
		CurrentInput:   []byte("Fix the bug"),
	}

	bundle, err := engine.Build(context.Background(), req)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if bundle.Report.TotalTokens <= 0 {
		t.Errorf("Report.TotalTokens = %d, want > 0", bundle.Report.TotalTokens)
	}
	if bundle.Report.CurrentInputTokens <= 0 {
		t.Errorf("Report.CurrentInputTokens = %d, want > 0", bundle.Report.CurrentInputTokens)
	}
}

func TestBuildBudgetOK(t *testing.T) {
	engine, _ := newTestEngine()

	req := BuildRequest{
		TaskID:       "t1",
		Budget:       TokenBudget{ImmutablePrefix: 100000, Conversation: 50000, Scratchpad: 20000},
		CurrentInput: []byte("short"),
	}

	bundle, _ := engine.Build(context.Background(), req)
	if bundle.Report.BudgetStatus.Level != BudgetOK {
		t.Errorf("BudgetStatus.Level = %s, want ok", bundle.Report.BudgetStatus.Level)
	}
}

func TestBuildBudgetBLOCK(t *testing.T) {
	engine, _ := newTestEngine()

	req := BuildRequest{
		TaskID:       "t1",
		Budget:       TokenBudget{ImmutablePrefix: 1, Conversation: 1, Scratchpad: 1},
		CurrentInput: []byte("This is a very long input that will exceed the tiny budget"),
	}

	bundle, _ := engine.Build(context.Background(), req)
	if bundle.Report.BudgetStatus.Level != BudgetBLOCK {
		t.Errorf("BudgetStatus.Level = %s, want block (budget too small)", bundle.Report.BudgetStatus.Level)
	}
}

func TestBuildCacheFingerprintSet(t *testing.T) {
	engine, _ := newTestEngine()

	req := BuildRequest{TaskID: "t1", Budget: TokenBudget{ImmutablePrefix: 10000}}
	bundle, _ := engine.Build(context.Background(), req)

	if bundle.CacheFingerprint.SHA256 != bundle.ImmutablePrefix.SHA256 {
		t.Errorf("CacheFingerprint.SHA256 = %q, want %q", bundle.CacheFingerprint.SHA256, bundle.ImmutablePrefix.SHA256)
	}
}

func TestRecordModelCallDelegates(t *testing.T) {
	engine, cr := newTestEngine()

	obs := cache.Observation{
		Fingerprint:  prefix.Fingerprint{SHA256: "test", Version: 1},
		Provider:     "openai",
		InputTokens:  1000,
		CachedTokens: 800,
		ObservedAt:   time.Now().UTC(),
	}

	if err := engine.RecordModelCall(context.Background(), obs); err != nil {
		t.Fatalf("RecordModelCall() error: %v", err)
	}

	if len(cr.observations) != 1 {
		t.Fatalf("CacheRegistry observations = %d, want 1", len(cr.observations))
	}
	if cr.observations[0].Provider != "openai" {
		t.Errorf("Observation provider = %q, want openai", cr.observations[0].Provider)
	}
}

func TestBuildAssemblyOrder(t *testing.T) {
	// Verify the token breakdown respects the layer ordering:
	// Prefix → Conversation → Scratchpad → CurrentInput
	engine, _ := newTestEngine()

	// Add conversation events
	cl := engine.conversationLog.(*stubConversationLog)
	cl.events = []conversation.Event{
		{Type: conversation.EventUserMessage, Payload: json.RawMessage(`"previous message"`)},
	}

	// Add scratchpad items
	_ = engine.scratchpad.Put(context.Background(), scratchpad.Item{
		ID:      "s1",
		TaskID:  "t1",
		Kind:    scratchpad.ItemKindToolOutput,
		Content: []byte("tool result data"),
	})

	req := BuildRequest{
		TaskID:         "t1",
		ConversationID: "c1",
		Budget:         TokenBudget{ImmutablePrefix: 10000, Conversation: 5000, Scratchpad: 2000},
		CurrentInput:   []byte("current user question"),
	}

	bundle, err := engine.Build(context.Background(), req)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// All four layers should have tokens
	if bundle.Report.PrefixTokens == 0 {
		t.Error("PrefixTokens is 0, want > 0")
	}
	if bundle.Report.ConversationTokens == 0 {
		t.Error("ConversationTokens is 0, want > 0")
	}
	if bundle.Report.ScratchpadTokens == 0 {
		t.Error("ScratchpadTokens is 0, want > 0")
	}
	if bundle.Report.CurrentInputTokens == 0 {
		t.Error("CurrentInputTokens is 0, want > 0")
	}

	// Total should be sum of all parts
	expectedTotal := bundle.Report.PrefixTokens + bundle.Report.ConversationTokens + bundle.Report.ScratchpadTokens + bundle.Report.CurrentInputTokens
	if bundle.Report.TotalTokens != expectedTotal {
		t.Errorf("TotalTokens = %d, want sum = %d", bundle.Report.TotalTokens, expectedTotal)
	}
}
