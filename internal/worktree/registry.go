package worktree

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/config"
)

// Registry stores worktree metadata as append-only JSONL.
// Directory permissions: 0700, file permissions: 0600.
// No API keys are ever recorded.
type Registry struct {
	mu   sync.Mutex
	path string
	file *os.File
}

// NewRegistry creates or opens a Registry at the given path.
func NewRegistry(path string) (*Registry, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("worktree: create registry dir %q: %w", dir, err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("worktree: open registry %q: %w", path, err)
	}

	return &Registry{path: path, file: f}, nil
}

// Record appends a WorktreeInfo entry to the registry.
func (r *Registry) Record(info WorktreeInfo) error {
	// Sanitize: never record API keys
	safe := sanitizeInfo(info)

	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := json.Marshal(safe)
	if err != nil {
		return fmt.Errorf("worktree: marshal registry entry: %w", err)
	}
	data = append(data, '\n')

	if _, err := r.file.Write(data); err != nil {
		return fmt.Errorf("worktree: write registry entry: %w", err)
	}
	return nil
}

// Load reads all entries from the registry file and returns them.
// The last entry for each ID is the current state.
func (r *Registry) Load() (map[string]WorktreeInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Seek to beginning
	if _, err := r.file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("worktree: seek registry: %w", err)
	}

	result := make(map[string]WorktreeInfo)
	decoder := json.NewDecoder(r.file)
	for decoder.More() {
		var info WorktreeInfo
		if err := decoder.Decode(&info); err != nil {
			break // stop at first decode error (EOF or corruption)
		}
		result[info.ID] = info // last entry wins
	}

	// Seek to end for future appends
	if _, err := r.file.Seek(0, 2); err != nil {
		return nil, fmt.Errorf("worktree: seek registry end: %w", err)
	}

	return result, nil
}

// Close closes the registry file.
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.file.Close()
}

// Path returns the file path of the registry.
func (r *Registry) Path() string {
	return r.path
}

// DefaultRegistryPath returns the default registry path under repoRoot.
func DefaultRegistryPath(repoRoot string) string {
	return filepath.Join(repoRoot, config.DirName(), "worktrees", "registry.jsonl")
}

// generateWorktreeID creates a cryptographically random 16-byte hex string.
func generateWorktreeID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("worktree: generate id: %w", err)
	}
	return "wt_" + hex.EncodeToString(b), nil
}

// sanitizeInfo removes any sensitive values from metadata.
func sanitizeInfo(info WorktreeInfo) WorktreeInfo {
	safe := info
	if safe.Metadata != nil {
		safe.Metadata = make(map[string]string, len(info.Metadata))
		for k, v := range info.Metadata {
			// Only keep safe metadata keys
			if isSafeMetaKey(k) {
				safe.Metadata[k] = v
			} else {
				safe.Metadata[k] = "<redacted>"
			}
		}
	}
	return safe
}

// isSafeMetaKey returns true if the metadata key is safe to record.
func isSafeMetaKey(key string) bool {
	switch key {
	case "source", "goal", "base_ref":
		return true
	default:
		return false
	}
}

// nowUTC returns the current time in UTC.
func nowUTC() time.Time {
	return time.Now().UTC()
}
