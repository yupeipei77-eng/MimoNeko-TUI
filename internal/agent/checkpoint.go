package agent

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

// Checkpoint represents a snapshot of an agent run at a point in time.
// It captures the full state needed to inspect, resume, or debug a run.
type Checkpoint struct {
	RunID      string      `json:"run_id"`
	TaskID     string      `json:"task_id"`
	State      AgentState  `json:"state"`
	StepIndex  int         `json:"step_index"`
	Steps      []AgentStep `json:"steps"`
	ContractID string      `json:"contract_id"`
	CreatedAt  time.Time   `json:"created_at"`
}

// CheckpointStore persists agent run checkpoints.
type CheckpointStore interface {
	// Save writes a checkpoint for the given run.
	Save(ctx context.Context, cp Checkpoint) error

	// Load retrieves the latest checkpoint for a run ID.
	Load(ctx context.Context, runID string) (Checkpoint, error)

	// List returns all checkpoint run IDs, ordered by creation time descending.
	List(ctx context.Context) ([]string, error)
}

// JSONLCheckpointStore implements CheckpointStore using a JSONL file.
// Each checkpoint is appended as a single line; Load returns the latest
// checkpoint for a given run ID by scanning the file.
type JSONLCheckpointStore struct {
	mu   sync.Mutex
	path string
}

// NewJSONLCheckpointStore creates a new JSONL-backed checkpoint store.
// The parent directory is created if it does not exist.
func NewJSONLCheckpointStore(path string) (*JSONLCheckpointStore, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("agent: create checkpoint dir %q: %w", dir, err)
	}
	// Ensure file exists
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("agent: open checkpoint file %q: %w", path, err)
	}
	f.Close()

	return &JSONLCheckpointStore{path: path}, nil
}

// Save appends a checkpoint as a JSONL line.
func (s *JSONLCheckpointStore) Save(ctx context.Context, cp Checkpoint) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(cp)
	if err != nil {
		return fmt.Errorf("agent: marshal checkpoint: %w", err)
	}

	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("agent: open checkpoint file: %w", err)
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "%s\n", data); err != nil {
		return fmt.Errorf("agent: write checkpoint: %w", err)
	}

	return nil
}

// Load reads all checkpoints and returns the latest one for the given run ID.
func (s *JSONLCheckpointStore) Load(ctx context.Context, runID string) (Checkpoint, error) {
	if err := ctx.Err(); err != nil {
		return Checkpoint{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	checkpoints, err := s.readAll()
	if err != nil {
		return Checkpoint{}, err
	}

	// Find the latest checkpoint for this run ID
	var latest *Checkpoint
	for i := range checkpoints {
		if checkpoints[i].RunID == runID {
			cp := checkpoints[i]
			if latest == nil || cp.CreatedAt.After(latest.CreatedAt) {
				latest = &cp
			}
		}
	}

	if latest == nil {
		return Checkpoint{}, fmt.Errorf("agent: checkpoint not found for run %q", runID)
	}

	return *latest, nil
}

// List returns all unique run IDs in descending creation order.
func (s *JSONLCheckpointStore) List(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	checkpoints, err := s.readAll()
	if err != nil {
		return nil, err
	}

	// Deduplicate by run ID, keeping the latest
	seen := make(map[string]time.Time)
	for _, cp := range checkpoints {
		if existing, ok := seen[cp.RunID]; !ok || cp.CreatedAt.After(existing) {
			seen[cp.RunID] = cp.CreatedAt
		}
	}

	// Sort by creation time descending
	type entry struct {
		id        string
		createdAt time.Time
	}
	entries := make([]entry, 0, len(seen))
	for id, t := range seen {
		entries = append(entries, entry{id: id, createdAt: t})
	}

	// Simple insertion sort (small N expected)
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

func (s *JSONLCheckpointStore) readAll() ([]Checkpoint, error) {
	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("agent: open checkpoint file: %w", err)
	}
	defer f.Close()

	var checkpoints []Checkpoint
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var cp Checkpoint
		if err := json.Unmarshal(scanner.Bytes(), &cp); err != nil {
			continue // skip corrupt lines
		}
		checkpoints = append(checkpoints, cp)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("agent: read checkpoint file: %w", err)
	}
	return checkpoints, nil
}

// DefaultCheckpointPath returns the default checkpoint file path under repoRoot.
func DefaultCheckpointPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".reasonforge", "logs", "checkpoints.jsonl")
}

const (
	// maxModelTextBytes is the maximum number of bytes of ModelText retained in a checkpoint.
	maxModelTextBytes = 512

	// maxToolOutputBytes is the maximum number of bytes of Stdout/Stderr retained in a checkpoint.
	maxToolOutputBytes = 1024

	// truncationMarker is appended when content is truncated.
	truncationMarker = "...[truncated]"
)

// safeCheckpointArgKeys are the keys allowed in cleartext in ToolCall.Args within checkpoints.
var safeCheckpointArgKeys = map[string]bool{
	"path":         true,
	"command_name": true,
	"max_bytes":    true,
	"create_dirs":  true,
}

// SanitizeCheckpoint returns a copy of the checkpoint with sensitive data
// redacted and large fields truncated. The original checkpoint is not modified.
//
// Rules:
//   - ModelText is truncated to 512 bytes
//   - ToolResponse.Stdout and Stderr are truncated to 1024 bytes each
//   - ToolCall.Args values are redacted unless the key is in the safe list
//   - ToolResponse.Artifacts content hashes are preserved (already safe)
func SanitizeCheckpoint(cp Checkpoint) Checkpoint {
	sanitized := cp
	sanitized.Steps = make([]AgentStep, len(cp.Steps))
	for i, step := range cp.Steps {
		sanitized.Steps[i] = sanitizeStep(step)
	}
	return sanitized
}

func sanitizeStep(step AgentStep) AgentStep {
	s := step

	// Truncate ModelText
	if len(s.ModelText) > maxModelTextBytes {
		s.ModelText = s.ModelText[:maxModelTextBytes] + truncationMarker
	}

	// Redact ToolCall.Args
	if s.ToolCall != nil {
		s.ToolCall = &ToolCall{
			Name: s.ToolCall.Name,
			Args: redactArgs(s.ToolCall.Args),
		}
	}

	// Truncate ToolResponse output fields
	if s.ToolResponse != nil {
		tr := *s.ToolResponse
		if len(tr.Stdout) > maxToolOutputBytes {
			tr.Stdout = tr.Stdout[:maxToolOutputBytes] + truncationMarker
		}
		if len(tr.Stderr) > maxToolOutputBytes {
			tr.Stderr = tr.Stderr[:maxToolOutputBytes] + truncationMarker
		}
		// Artifacts are kept (content_hash is safe, no raw content)
		s.ToolResponse = &tr
	}

	return s
}

// redactArgs returns a copy of args with values replaced by "<redacted>"
// for keys not in the safe list.
func redactArgs(args map[string]string) map[string]string {
	if args == nil {
		return nil
	}
	result := make(map[string]string, len(args))
	for k, v := range args {
		if safeCheckpointArgKeys[k] {
			result[k] = v
		} else {
			result[k] = "<redacted>"
		}
	}
	return result
}
