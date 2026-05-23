package agent

import (
	"context"
	"time"

	"github.com/reasonforge/reasonforge/internal/contextengine"
	"github.com/reasonforge/reasonforge/internal/conversation"
	"github.com/reasonforge/reasonforge/internal/model"
	"github.com/reasonforge/reasonforge/internal/task"
	"github.com/reasonforge/reasonforge/internal/toolruntime"
)

type Dependencies struct {
	ContextEngine  contextengine.ContextEngine
	ModelRouter    model.ModelRouter
	ToolRuntime    toolruntime.ToolRuntime
	ConversationLog conversation.ConversationLog
}

type RunResult struct {
	TaskID    string
	Status    string
	StartedAt time.Time
	EndedAt   time.Time
}

type AgentRuntime interface {
	Run(ctx context.Context, contract task.TaskContract) (RunResult, error)
}
