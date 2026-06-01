package approval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// FileStore implements Store with JSON file persistence.
// Data is stored in a single JSON file with deterministic ordering.
type FileStore struct {
	mu       sync.RWMutex
	path     string
	requests map[string]*ApprovalRequest
}

// NewFileStore creates a new file-backed approval store.
// The path should be the full path to the JSON file (e.g., .mimoneko/approvals.json).
func NewFileStore(path string) *FileStore {
	return &FileStore{
		path:     path,
		requests: make(map[string]*ApprovalRequest),
	}
}

// Load reads approval requests from the JSON file.
// If the file does not exist, the store is initialized empty (no error).
// Returns an error if the file exists but cannot be parsed.
func (s *FileStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			// File does not exist, initialize empty
			s.requests = make(map[string]*ApprovalRequest)
			return nil
		}
		return fmt.Errorf("approval: read file: %w", err)
	}

	if len(data) == 0 {
		s.requests = make(map[string]*ApprovalRequest)
		return nil
	}

	var requests []*ApprovalRequest
	if err := json.Unmarshal(data, &requests); err != nil {
		return fmt.Errorf("approval: parse JSON: %w", err)
	}

	s.requests = make(map[string]*ApprovalRequest, len(requests))
	for _, req := range requests {
		s.requests[req.ID] = req
	}

	return nil
}

// Save writes all approval requests to the JSON file.
// Requests are ordered by CreatedAt then ID for deterministic output.
func (s *FileStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Convert to slice and sort
	requests := s.sortedList()

	// Marshal with indentation for human readability
	data, err := json.MarshalIndent(requests, "", "  ")
	if err != nil {
		return fmt.Errorf("approval: marshal JSON: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("approval: create directory: %w", err)
	}

	// Write with secure permissions
	if err := os.WriteFile(s.path, data, 0600); err != nil {
		return fmt.Errorf("approval: write file: %w", err)
	}

	return nil
}

// List returns all approval requests.
func (s *FileStore) List() []*ApprovalRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.sortedList()
}

// Get returns an approval request by ID.
func (s *FileStore) Get(id string) (*ApprovalRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	req, ok := s.requests[id]
	if !ok {
		return nil, fmt.Errorf("approval: request %s not found", id)
	}
	return req, nil
}

// Add stores a new approval request and persists to disk.
func (s *FileStore) Add(req *ApprovalRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.requests[req.ID]; ok {
		return fmt.Errorf("approval: request %s already exists", req.ID)
	}

	s.requests[req.ID] = req
	return s.saveLocked()
}

// Update updates an existing approval request and persists to disk.
func (s *FileStore) Update(req *ApprovalRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.requests[req.ID]; !ok {
		return fmt.Errorf("approval: request %s not found", req.ID)
	}

	s.requests[req.ID] = req
	return s.saveLocked()
}

// Delete removes an approval request and persists to disk.
func (s *FileStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.requests[id]; !ok {
		return fmt.Errorf("approval: request %s not found", id)
	}

	delete(s.requests, id)
	return s.saveLocked()
}

// Pending returns all pending approval requests.
func (s *FileStore) Pending() []*ApprovalRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ApprovalRequest
	for _, req := range s.requests {
		if req.IsPending() {
			result = append(result, req)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})

	return result
}

// Count returns the total number of approval requests.
func (s *FileStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.requests)
}

// sortedList returns a sorted slice of all requests.
// Must be called with lock held.
func (s *FileStore) sortedList() []*ApprovalRequest {
	requests := make([]*ApprovalRequest, 0, len(s.requests))
	for _, req := range s.requests {
		requests = append(requests, req)
	}

	sort.Slice(requests, func(i, j int) bool {
		if requests[i].CreatedAt.Equal(requests[j].CreatedAt) {
			return requests[i].ID < requests[j].ID
		}
		return requests[i].CreatedAt.Before(requests[j].CreatedAt)
	})

	return requests
}

// saveLocked writes to disk. Must be called with write lock held.
func (s *FileStore) saveLocked() error {
	// Convert to slice and sort
	requests := s.sortedList()

	// Marshal with indentation for human readability
	data, err := json.MarshalIndent(requests, "", "  ")
	if err != nil {
		return fmt.Errorf("approval: marshal JSON: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("approval: create directory: %w", err)
	}

	// Write with secure permissions
	if err := os.WriteFile(s.path, data, 0600); err != nil {
		return fmt.Errorf("approval: write file: %w", err)
	}

	return nil
}
