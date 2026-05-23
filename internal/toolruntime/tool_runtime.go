package toolruntime

import (
	"context"
	"encoding/json"
	"time"
)

type Schema struct {
	Name        string
	Description string
	JSONSchema  json.RawMessage
}

type Call struct {
	ID        string
	TaskID    string
	ToolName  string
	Arguments json.RawMessage
	Timeout   time.Duration
}

type Result struct {
	CallID    string
	ExitCode  int
	Stdout    []byte
	Stderr    []byte
	Metadata  map[string]string
	StartedAt time.Time
	EndedAt   time.Time
}

type ToolRuntime interface {
	Schemas(ctx context.Context) ([]Schema, error)
	Execute(ctx context.Context, call Call) (Result, error)
}
