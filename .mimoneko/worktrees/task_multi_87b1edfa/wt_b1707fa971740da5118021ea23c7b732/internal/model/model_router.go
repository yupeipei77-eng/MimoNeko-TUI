package model

import (
	"context"
	"time"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role    Role
	Content string
}

type RouteRequest struct {
	TaskID       string
	Capability   string
	TokenBudget  int
	RequiresCache bool
}

type Route struct {
	Provider string
	Model    string
	BaseURL  string
}

type CompletionRequest struct {
	TaskID          string
	Route          Route
	ImmutablePrefix []byte
	VolatileMessages []Message
	MaxOutputTokens int
}

type CacheUsage struct {
	InputTokens  int
	CachedTokens int
}

type CompletionResponse struct {
	ID          string
	Model       string
	Text        string
	CacheUsage  CacheUsage
	CompletedAt time.Time
}

type ModelRouter interface {
	Route(ctx context.Context, req RouteRequest) (Route, error)
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}
