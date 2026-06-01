package approval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// SnapshotStore implements persistence for ResumeSnapshot.
type SnapshotStore struct {
	mu        sync.RWMutex
	path      string
	snapshots map[string]*ResumeSnapshot // keyed by approval_id
}

// NewSnapshotStore creates a new file-backed snapshot store.
func NewSnapshotStore(path string) *SnapshotStore {
	return &SnapshotStore{
		path:      path,
		snapshots: make(map[string]*ResumeSnapshot),
	}
}

// Load reads snapshots from the JSON file.
func (s *SnapshotStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.snapshots = make(map[string]*ResumeSnapshot)
			return nil
		}
		return fmt.Errorf("snapshot: read file: %w", err)
	}

	if len(data) == 0 {
		s.snapshots = make(map[string]*ResumeSnapshot)
		return nil
	}

	var snapshots []*ResumeSnapshot
	if err := json.Unmarshal(data, &snapshots); err != nil {
		return fmt.Errorf("snapshot: parse JSON: %w", err)
	}

	s.snapshots = make(map[string]*ResumeSnapshot, len(snapshots))
	for _, snap := range snapshots {
		s.snapshots[snap.ApprovalID] = snap
	}

	return nil
}

// Save writes all snapshots to the JSON file.
func (s *SnapshotStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.saveLocked()
}

// Get returns a snapshot by approval ID.
func (s *SnapshotStore) Get(approvalID string) (*ResumeSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snap, ok := s.snapshots[approvalID]
	if !ok {
		return nil, fmt.Errorf("snapshot: not found for approval %s", approvalID)
	}
	return snap, nil
}

// Add stores a new snapshot and persists to disk.
func (s *SnapshotStore) Add(snap *ResumeSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.snapshots[snap.ApprovalID]; ok {
		return fmt.Errorf("snapshot: already exists for approval %s", snap.ApprovalID)
	}

	s.snapshots[snap.ApprovalID] = snap
	return s.saveLocked()
}

// Update updates an existing snapshot and persists to disk.
func (s *SnapshotStore) Update(snap *ResumeSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.snapshots[snap.ApprovalID]; !ok {
		return fmt.Errorf("snapshot: not found for approval %s", snap.ApprovalID)
	}

	s.snapshots[snap.ApprovalID] = snap
	return s.saveLocked()
}

// Upsert adds or updates a snapshot and persists to disk.
func (s *SnapshotStore) Upsert(snap *ResumeSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshots[snap.ApprovalID] = snap
	return s.saveLocked()
}

// List returns all snapshots sorted by CreatedAt then ApprovalID.
func (s *SnapshotStore) List() []*ResumeSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.sortedList()
}

// Count returns the total number of snapshots.
func (s *SnapshotStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.snapshots)
}

// sortedList returns a sorted slice of all snapshots.
func (s *SnapshotStore) sortedList() []*ResumeSnapshot {
	snapshots := make([]*ResumeSnapshot, 0, len(s.snapshots))
	for _, snap := range s.snapshots {
		snapshots = append(snapshots, snap)
	}

	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].CreatedAt.Equal(snapshots[j].CreatedAt) {
			return snapshots[i].ApprovalID < snapshots[j].ApprovalID
		}
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})

	return snapshots
}

// saveLocked writes to disk. Must be called with write lock held.
func (s *SnapshotStore) saveLocked() error {
	snapshots := s.sortedList()

	data, err := json.MarshalIndent(snapshots, "", "  ")
	if err != nil {
		return fmt.Errorf("snapshot: marshal JSON: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("snapshot: create directory: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0600); err != nil {
		return fmt.Errorf("snapshot: write file: %w", err)
	}

	return nil
}
