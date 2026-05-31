package modelrouter

import (
	"strings"
	"testing"

	"github.com/nekonomimo/nekonomimo/internal/contextengine"
)

func TestBundleToMessagesOrder(t *testing.T) {
	bundle := contextengine.Bundle{
		Layers: []contextengine.ContextLayer{
			{Name: "immutable_prefix", Bytes: []byte("system prompt"), Tokens: 10},
			{Name: "conversation_log", Bytes: []byte("assistant message"), Tokens: 5},
			{Name: "scratchpad", Bytes: []byte("scratchpad data"), Tokens: 3},
			{Name: "current_input", Bytes: []byte("user query"), Tokens: 2},
		},
	}

	messages := BundleToMessages(bundle)

	if len(messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(messages))
	}

	// Order must match: immutable_prefix, conversation_log, scratchpad, current_input
	if messages[0].Role != RoleSystem {
		t.Errorf("message 0 role = %q, want system", messages[0].Role)
	}
	if messages[0].Content != "system prompt" {
		t.Errorf("message 0 content = %q, want %q", messages[0].Content, "system prompt")
	}
	if messages[1].Role != RoleAssistant {
		t.Errorf("message 1 role = %q, want assistant", messages[1].Role)
	}
	if messages[2].Role != RoleSystem {
		t.Errorf("message 2 role = %q, want system (scratchpad)", messages[2].Role)
	}
	if !strings.HasPrefix(messages[2].Content, "[volatile context]") {
		t.Errorf("scratchpad content should be prefixed with [volatile context], got %q", messages[2].Content)
	}
	if messages[3].Role != RoleUser {
		t.Errorf("message 3 role = %q, want user", messages[3].Role)
	}
}

func TestBundleToMessagesCurrentInputIsLast(t *testing.T) {
	bundle := contextengine.Bundle{
		Layers: []contextengine.ContextLayer{
			{Name: "immutable_prefix", Bytes: []byte("prefix"), Tokens: 5},
			{Name: "current_input", Bytes: []byte("user input"), Tokens: 2},
		},
	}

	messages := BundleToMessages(bundle)
	last := messages[len(messages)-1]

	if last.Role != RoleUser {
		t.Errorf("last message role = %q, want user", last.Role)
	}
	if last.Content != "user input" {
		t.Errorf("last message content = %q, want %q", last.Content, "user input")
	}
}

func TestBundleToMessagesImmutablePrefixIsSystem(t *testing.T) {
	bundle := contextengine.Bundle{
		Layers: []contextengine.ContextLayer{
			{Name: "immutable_prefix", Bytes: []byte("system instructions"), Tokens: 5},
		},
	}

	messages := BundleToMessages(bundle)
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].Role != RoleSystem {
		t.Errorf("immutable_prefix role = %q, want system", messages[0].Role)
	}
}

func TestBundleToMessagesScratchpadNotInImmutablePrefix(t *testing.T) {
	bundle := contextengine.Bundle{
		Layers: []contextengine.ContextLayer{
			{Name: "immutable_prefix", Bytes: []byte("prefix"), Tokens: 5},
			{Name: "scratchpad", Bytes: []byte("volatile data"), Tokens: 3},
		},
	}

	messages := BundleToMessages(bundle)

	// Find scratchpad message
	var scratchpadMsg *Message
	for i := range messages {
		if strings.Contains(messages[i].Content, "volatile data") {
			scratchpadMsg = &messages[i]
			break
		}
	}

	if scratchpadMsg == nil {
		t.Fatal("scratchpad message not found")
	}

	// Scratchpad should be system role, marked as volatile context, NOT part of immutable prefix
	if scratchpadMsg.Role != RoleSystem {
		t.Errorf("scratchpad role = %q, want system", scratchpadMsg.Role)
	}
	if !strings.Contains(scratchpadMsg.Content, "[volatile context]") {
		t.Errorf("scratchpad content should contain [volatile context] marker, got %q", scratchpadMsg.Content)
	}

	// The first message (immutable_prefix) should NOT contain volatile data
	if strings.Contains(messages[0].Content, "volatile data") {
		t.Error("scratchpad data leaked into immutable_prefix message")
	}
}

func TestBundleToMessagesEmptyLayersSkipped(t *testing.T) {
	bundle := contextengine.Bundle{
		Layers: []contextengine.ContextLayer{
			{Name: "immutable_prefix", Bytes: []byte("prefix"), Tokens: 5},
			{Name: "conversation_log", Bytes: []byte(""), Tokens: 0},
			{Name: "scratchpad", Bytes: nil, Tokens: 0},
			{Name: "current_input", Bytes: []byte("input"), Tokens: 2},
		},
	}

	messages := BundleToMessages(bundle)
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages (empty layers skipped), got %d", len(messages))
	}
}

func TestBundleToMessagesDoesNotChangeLayerOrder(t *testing.T) {
	// Test that the converter respects the original Bundle.Layers order
	// and does NOT reorder
	bundle := contextengine.Bundle{
		Layers: []contextengine.ContextLayer{
			{Name: "immutable_prefix", Bytes: []byte("A"), Tokens: 1},
			{Name: "conversation_log", Bytes: []byte("B"), Tokens: 1},
			{Name: "scratchpad", Bytes: []byte("C"), Tokens: 1},
			{Name: "current_input", Bytes: []byte("D"), Tokens: 1},
		},
	}

	messages := BundleToMessages(bundle)
	expected := []string{"A", "B", "C", "D"}
	for i, msg := range messages {
		// Strip [volatile context] prefix for comparison
		content := msg.Content
		if i == 2 {
			content = strings.TrimPrefix(content, "[volatile context]\n")
		}
		if content != expected[i] {
			t.Errorf("message %d content = %q, want %q", i, content, expected[i])
		}
	}
}
