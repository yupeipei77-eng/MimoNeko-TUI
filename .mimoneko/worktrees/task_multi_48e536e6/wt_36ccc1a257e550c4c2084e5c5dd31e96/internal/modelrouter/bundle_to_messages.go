package modelrouter

import (
	"fmt"

	"github.com/nekonomimo/nekonomimo/internal/contextengine"
)

// OpenAI Message types for conversion output.
// These mirror the OpenAI Chat Completion API message format.

// Role represents the role of a message in the OpenAI API.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message represents an OpenAI-compatible chat message.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// BundleToMessages converts a ContextEngine Bundle into OpenAI-compatible messages.
//
// Conversion order (must be stable):
//  1. immutable_prefix  -> role=system
//  2. conversation_log  -> role=assistant (condensed into a single message)
//  3. scratchpad        -> role=system, marked as volatile context
//  4. current_input     -> role=user (always the last message)
//
// The order must match Bundle.Layers ordering. CurrentInput must always be
// the last user message. Scratchpad must not enter the immutable prefix.
func BundleToMessages(bundle contextengine.Bundle) []Message {
	var messages []Message

	for _, layer := range bundle.Layers {
		content := string(layer.Bytes)
		if content == "" {
			continue
		}

		switch layer.Name {
		case "immutable_prefix":
			messages = append(messages, Message{
				Role:    RoleSystem,
				Content: content,
			})
		case "conversation_log":
			messages = append(messages, Message{
				Role:    RoleAssistant,
				Content: content,
			})
		case "scratchpad":
			messages = append(messages, Message{
				Role:    RoleSystem,
				Content: fmt.Sprintf("[volatile context]\n%s", content),
			})
		case "current_input":
			messages = append(messages, Message{
				Role:    RoleUser,
				Content: content,
			})
		}
	}

	return messages
}
