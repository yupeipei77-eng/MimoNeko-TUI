# Phase 4 â€?Agent Runtime

## Overview

Phase 4 implements the minimal Agent Runtime, completing the execution loop:

```
TaskContract
  -> ContextEngine.Build
  -> ModelRouter.Complete
  -> Parse ToolCall
  -> ApprovalPolicy
  -> ToolRuntime.Run
  -> ToolResponse -> ScratchpadItem
  -> Checkpoint
  -> Continue / Finish
```

This phase delivers a **Single Agent Runtime** only. Multi-agent, Worktree, Repo Indexer, Memory RAG, and UI are explicitly out of scope.

## PR Breakdown

### PR 4.1 â€?Agent Runtime Foundation

Core types and infrastructure:

- **TaskContract** â€?Concrete struct replacing the previous interface. Defines the execution boundary for every agent run (allowed/denied tools, paths, max steps, approval requirements, dry-run mode).
- **AgentRuntime interface** â€?`Run(ctx, AgentRunRequest) (AgentRunResult, error)`
- **AgentRunRequest / AgentRunResult** â€?Input/output types for the agent loop.
- **AgentState** â€?Lifecycle states: `pending`, `running`, `waiting_approval`, `succeeded`, `failed`, `cancelled`.
- **AgentStep** â€?Single iteration record (model call or tool execution).
- **ToolCall** â€?Parsed tool invocation from model output.
- **CheckpointStore** â€?Interface for persisting agent run snapshots.
- **JSONLCheckpointStore** â€?JSONL-backed implementation (append-only, consistent with project conventions).
- **ToolCall Parser** â€?Extracts `{"tool_call": {"name": "...", "args": {...}}}` blocks from model output text using balanced-brace scanning.

### PR 4.2 â€?Single Agent Loop

The core execution loop:

- **SingleAgentRuntime** â€?Implements `AgentRuntime` with the full loop:
  1. Validate TaskContract
  2. Build context via `ContextEngine.Build`
  3. Call model via `ModelRouter.Complete`
  4. Parse tool call from model output
  5. Check contract allows the tool
  6. Check approval policy
  7. Execute tool via `ToolRuntime.Run`
  8. Convert `ToolResponse` to `ScratchpadItem` and inject
  9. Save checkpoint after each step
  10. Loop until max_steps, terminal state, or error
- **max_steps / max_tool_calls** controls
- **Dry-run** support
- **ToolResponse -> Scratchpad injection**: successful tool output is written to the scratchpad as `ItemKindToolOutput`
- **Context cancellation** handling
- 12 unit tests covering: simple completion, tool call loop, denied tool, max steps, max tool calls, medium risk approval, context engine error, cancelled context, invalid contract, checkpoints saved, scratchpad injection, dry run, and default contract integration

### PR 4.3 â€?ApprovalPolicy + CLI

Safety and user interface:

- **ApprovalPolicy** â€?Risk-based approval gate with three decisions:
  - `auto_approved` â€?Low-risk tools (configurable, default: auto-approve low)
  - `requires_approval` â€?Medium-risk tools need human consent
  - `denied` â€?High-risk tools are blocked (configurable)
- **DefaultApprovalPolicy** â€?Auto-approve low, require approval for medium, block high
- **InteractiveApprovalPolicy** â€?Reads from stdin for interactive `y/N` prompts
- **Interactive approval in SingleAgentRuntime** â€?When a tool requires approval, the runtime prompts the user if stdin is available; otherwise the run enters `waiting_approval` state
- **NekoMIMO run** â€?CLI command to launch an agent run with flags:
  - `--goal` (required): task objective
  - `--dir`: project root
  - `--max-steps`: override max loop iterations
  - `--dry-run`: dry run mode (default: true)
  - `--auto-approve-medium`: auto-approve medium-risk tools
  - `--task-id`: explicit task ID
- README and docs update

## Core Types

### TaskContract

```go
type TaskContract struct {
    ID                     string
    Goal                   string
    RepoRoot               string
    AllowedPaths           []string
    DeniedPaths            []string
    AllowedTools           []string
    DeniedTools            []string
    MaxSteps               int
    MaxToolCalls           int
    MaxOutputBytes         int
    RequireApprovalForRisk []string
    DryRun                 bool
    CreatedAt              time.Time
}
```

Every agent run **must** have a TaskContract. If the user does not provide one, `DefaultContract()` generates a safe default:
- `AllowedTools: [file_read, git_diff, test_run]`
- `DeniedTools: [file_write, file_patch]`
- `RequireApprovalForRisk: [medium]`
- `MaxSteps: 5`
- `DryRun: true`

Contract is the **agent-level** boundary. ToolRuntime is the **system-level** security boundary. Both must pass.

### ApprovalPolicy

```go
type ApprovalPolicy struct {
    AutoApproveLowRisk    bool  // default: true
    AutoApproveMediumRisk bool  // default: false
    BlockHighRisk         bool  // default: true
}
```

Three possible decisions:
- `auto_approved` â€?Tool executes immediately
- `requires_approval` â€?Tool needs human approval (interactive `y/N` prompt or `waiting_approval` state)
- `denied` â€?Tool is blocked

### AgentStep

Each step is either a model call (`Type="model"`) or a tool execution (`Type="tool"`):

```go
type AgentStep struct {
    StepID        string
    Index         int
    Type          string     // "model" or "tool"
    State         AgentState
    ModelProvider string
    Model         string
    ModelText     string
    ToolCall      *ToolCall
    ToolResponse  *tools.ToolResponse
    Error         string
    StartedAt     time.Time
    FinishedAt    time.Time
}
```

### ToolCall Format

Model output contains a JSON block:

```json
{"tool_call": {"name": "file_read", "args": {"path": "README.md"}}}
```

The parser uses balanced-brace scanning to find candidate JSON objects and unmarshals them. This is intentionally minimal for Phase 4; future phases may adopt structured function calling APIs.

### CheckpointStore

```go
type CheckpointStore interface {
    Save(ctx context.Context, cp Checkpoint) error
    Load(ctx context.Context, runID string) (Checkpoint, error)
    List(ctx context.Context) ([]string, error)
}
```

`JSONLCheckpointStore` persists checkpoints as JSONL lines at `.nekonomimo/logs/checkpoints.jsonl`, consistent with the project's append-only convention.

## Agent Loop Flow

```
1. Validate TaskContract
2. Generate RunID, set state=running
3. Save initial checkpoint
4. Loop:
   a. Check context cancellation
   b. ContextEngine.Build(current input + scratchpad)
   c. ModelRouter.Complete(bundle)
   d. Record model step
   e. Parse ToolCall from model output
   f. If no ToolCall -> set state=succeeded, break
   g. Check TaskContract.IsToolAllowed() -> denied? fail
   h. Check MaxToolCalls -> exceeded? fail
   i. Lookup tool in registry -> not found? fail
   j. ApprovalPolicy.Check(toolName, riskLevel, contractCheck)
      - denied -> fail
      - requires_approval -> prompt user or enter waiting_approval
      - auto_approved -> proceed
   k. ToolRuntime.Run(tool_request)
   l. Record tool step
   m. If success: inject ToolResponse -> ScratchpadItem
   n. Save checkpoint
   o. Prepare next input from tool output
   p. Check MaxSteps
5. Handle post-loop state (max_steps reached -> failed)
6. Save final checkpoint
7. Return AgentRunResult
```

## CLI Usage

```sh
# Run with default contract (dry-run, safe tools only)
NekoMIMO run --goal "Read the README and summarize it"

# Run with more steps and auto-approve medium-risk tools
NekoMIMO run --goal "Fix the bug in main.go" --max-steps 10 --auto-approve-medium --dry-run=false

# Run in a specific project directory
NekoMIMO run --goal "Run the tests" --dir /path/to/project
```

## Design Constraints

- Agent **must not** bypass ContextEngine.
- Agent **must not** bypass ModelRouter.
- Agent **must not** bypass ToolRuntime.
- Agent **must not** directly read/write files.
- No arbitrary `shell_exec`.
- No unregistered tool construction.
- No medium-risk tool execution without ApprovalPolicy.
- No real external model API calls in tests.
- Contract-level and system-level security must both pass.
- Default contract starts in dry-run mode for safety.

## File Layout

```
internal/agent/
  agent_runtime.go      # AgentRuntime interface, AgentState, AgentStep, ToolCall, Dependencies
  toolcall_parser.go    # ParseToolCall, HasToolCall, ExtractModelText, FormatToolCall
  checkpoint.go         # CheckpointStore, JSONLCheckpointStore
  single_agent.go       # SingleAgentRuntime with full agent loop
  approval.go           # ApprovalPolicy, DefaultApprovalPolicy, InteractiveApprovalPolicy
internal/task/
  task_contract.go      # TaskContract struct, DefaultContract, validation
internal/cli/
  cli.go                # + "run" command with --goal, --dry-run, --max-steps, --auto-approve-medium
```

## Test Coverage

- `internal/task/task_contract_test.go` â€?6 tests: DefaultContract, IsToolAllowed, IsPathAllowed, RequiresApproval, Validate, unique IDs
- `internal/agent/agent_runtime_test.go` â€?8 tests: ParseToolCall variants, HasToolCall, ExtractModelText, AgentState.IsTerminal, ID generation
- `internal/agent/checkpoint_test.go` â€?7 tests: SaveAndLoad, LoadNotFound, MultipleCheckpointsSameRun, List, CancelledContext, DefaultCheckpointPath, CreatesDirectory
- `internal/agent/approval_test.go` â€?12 tests: policy decisions for low/medium/high risk, contract denied, auto-approve medium, unknown risk, BlockHighRisk=false, low not auto-approved, interactive approval (non-interactive, approve, deny, empty)
- `internal/agent/single_agent_test.go` â€?13 tests: SimpleCompletion, ToolCallLoop, DeniedTool, MaxSteps, MaxToolCalls, MediumRiskRequiresApproval, ContextEngineError, CancelledContext, InvalidContract, CheckpointsSaved, ToolResponseToScratchpad, DryRun, DefaultContractIntegration
