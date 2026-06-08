// Package toolruntime provides compatibility aliases for the tools package.
// New code should import github.com/yupeipei77-eng/MimoNeko-TUI/internal/tools directly.
package toolruntime

import (
	"context"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/tools"
)

// ToolRuntime is an alias for tools.ToolRuntime.
type ToolRuntime = tools.ToolRuntime

// ToolRequest is an alias for tools.ToolRequest.
type ToolRequest = tools.ToolRequest

// ToolResponse is an alias for tools.ToolResponse.
type ToolResponse = tools.ToolResponse

// Run calls tools.ToolRuntime.Run.
func Run(ctx context.Context, rt ToolRuntime, req ToolRequest) (ToolResponse, error) {
	return rt.Run(ctx, req)
}
