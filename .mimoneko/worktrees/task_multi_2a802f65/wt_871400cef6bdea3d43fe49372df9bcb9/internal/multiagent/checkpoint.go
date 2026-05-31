package multiagent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MultiAgentCheckpoint captures a snapshot of a multi-agent run at a point in time.
// It is stored in a separate JSONL file from the single-agent checkpoints
// to avoid schema mixing.
type MultiAgentCheckpoint struct {
	RunID          string          `json:"run_id"`
	TaskID         string          `json:"task_id"`
	State          MultiAgentState `json:"state"`
	Iteration      int             `json:"iteration"`
	Phase          string          `json:"phase"` // "planner", "coder", "reviewer", "loop_start", "loop_end"
	Plan           *TaskPlan       `json:"plan,omitempty"`
	WorktreeID     string          `json:"worktree_id,omitempty"`
	Recommendation string          `json:"recommendation,omitempty"`
	Error          string          `json:"error,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

// MultiAgentCheckpointStore persists multi-agent run checkpoints.
type MultiAgentCheckpointStore interface {
	// Save appends a checkpoint for a multi-agent run.
	Save(ctx context.Context, cp MultiAgentCheckpoint) error

	// Load retrieves the latest checkpoint for a run ID.
	Load(ctx context.Context, runID string) (MultiAgentCheckpoint, error)

	// List returns all checkpoint run IDs, ordered by creation time descending.
	List(ctx context.Context) ([]string, error)
}

// JSONLMultiAgentCheckpointStore implements MultiAgentCheckpointStore using JSONL.
// Each checkpoint is appended as a single line; Load returns the latest
// checkpoint for a given run ID by scanning the file.
//
// Directory permissions: 0700
// File permissions: 0600
// Checkpoint write failure must cause run failure.
// No API keys, no sensitive diffs, no file_write content/patch old/new.
type JSONLMultiAgentCheckpointStore struct {
	mu   sync.Mutex
	path string
}

// NewJSONLMultiAgentCheckpointStore creates a new JSONL-backed checkpoint store.
func NewJSONLMultiAgentCheckpointStore(path string) (*JSONLMultiAgentCheckpointStore, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("multiagent: create checkpoint dir %q: %w", dir, err)
	}
	// Ensure file exists with correct permissions
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("multiagent: open checkpoint file %q: %w", path, err)
	}
	f.Close()

	return &JSONLMultiAgentCheckpointStore{path: path}, nil
}

// Save appends a sanitized checkpoint as a JSONL line.
func (s *JSONLMultiAgentCheckpointStore) Save(ctx context.Context, cp MultiAgentCheckpoint) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Sanitize the checkpoint before persisting
	cp = sanitizeMultiAgentCheckpoint(cp)

	data, err := json.Marshal(cp)
	if err != nil {
		return fmt.Errorf("multiagent: marshal checkpoint: %w", err)
	}

	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("multiagent: open checkpoint file: %w", err)
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "%s\n", data); err != nil {
		return fmt.Errorf("multiagent: write checkpoint: %w", err)
	}

	return nil
}

// Load reads all checkpoints and returns the latest one for the given run ID.
func (s *JSONLMultiAgentCheckpointStore) Load(ctx context.Context, runID string) (MultiAgentCheckpoint, error) {
	if err := ctx.Err(); err != nil {
		return MultiAgentCheckpoint{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	checkpoints, err := s.readAll()
	if err != nil {
		return MultiAgentCheckpoint{}, err
	}

	var latest *MultiAgentCheckpoint
	for i := range checkpoints {
		if checkpoints[i].RunID == runID {
			cp := checkpoints[i]
			if latest == nil || cp.CreatedAt.After(latest.CreatedAt) {
				latest = &cp
			}
		}
	}

	if latest == nil {
		return MultiAgentCheckpoint{}, fmt.Errorf("multiagent: checkpoint not found for run %q", runID)
	}

	return *latest, nil
}

// List returns all unique run IDs in descending creation order.
func (s *JSONLMultiAgentCheckpointStore) List(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	checkpoints, err := s.readAll()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]time.Time)
	for _, cp := range checkpoints {
		if existing, ok := seen[cp.RunID]; !ok || cp.CreatedAt.After(existing) {
			seen[cp.RunID] = cp.CreatedAt
		}
	}

	type entry struct {
		id        string
		createdAt time.Time
	}
	entries := make([]entry, 0, len(seen))
	for id, t := range seen {
		entries = append(entries, entry{id: id, createdAt: t})
	}

	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].createdAt.After(entries[j-1].createdAt); j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}

	result := make([]string, len(entries))
	for i, e := range entries {
		result[i] = e.id
	}
	return result, nil
}

func (s *JSONLMultiAgentCheckpointStore) readAll() ([]MultiAgentCheckpoint, error) {
	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("multiagent: open checkpoint file: %w", err)
	}
	defer f.Close()

	var checkpoints []MultiAgentCheckpoint
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var cp MultiAgentCheckpoint
		if err := json.Unmarshal(scanner.Bytes(), &cp); err != nil {
			continue // skip corrupt lines
		}
		checkpoints = append(checkpoints, cp)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("multiagent: read checkpoint file: %w", err)
	}
	return checkpoints, nil
}

// sanitizeMultiAgentCheckpoint redacts sensitive data before persisting.
// Rules:
//   - Error messages are checked for API key patterns
//   - Plan notes are checked for API key patterns
//   - No diff content is stored in checkpoints (only metadata)
//   - No file_write content / file_patch old/new
func sanitizeMultiAgentCheckpoint(cp MultiAgentCheckpoint) MultiAgentCheckpoint {
	sanitized := cp

	// Sanitize error message for API key patterns
	sanitized.Error = sanitizeCheckpointString(cp.Error)

	// Sanitize plan notes
	if sanitized.Plan != nil {
		plan := *sanitized.Plan
		plan.Notes = sanitizeCheckpointString(plan.Notes)
		plan.Goal = sanitizeCheckpointString(plan.Goal)
		for i := range plan.Steps {
			plan.Steps[i].Description = sanitizeCheckpointString(plan.Steps[i].Description)
			plan.Steps[i].ExpectedOutcome = sanitizeCheckpointString(plan.Steps[i].ExpectedOutcome)
		}
		sanitized.Plan = &plan
	}

	return sanitized
}

// sanitizeCheckpointString checks a string for API key patterns and redacts if found.
func sanitizeCheckpointString(s string) string {
	if s == "" {
		return s
	}
	if containsAPIKeyPattern(s) {
		return "[redacted: potential secret]"
	}
	return s
}

// DefaultMultiAgentCheckpointPath returns the default checkpoint file path.
func DefaultMultiAgentCheckpointPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".nekonomimo", "checkpoints", "multi_agent_runs.jsonl")
}
