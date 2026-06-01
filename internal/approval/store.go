package approval

import (
	"fmt"
	"sync"
)

// Store defines the interface for storing and retrieving approval requests.
type Store interface {
	// List returns all approval requests.
	List() []*ApprovalRequest

	// Get returns an approval request by ID.
	Get(id string) (*ApprovalRequest, error)

	// Add stores a new approval request.
	Add(req *ApprovalRequest) error

	// Update updates an existing approval request.
	Update(req *ApprovalRequest) error
}

// MemoryStore is an in-memory implementation of Store for testing and demos.
// It does NOT persist to disk.
type MemoryStore struct {
	mu       sync.RWMutex
	requests map[string]*ApprovalRequest
}

// NewMemoryStore creates a new in-memory approval store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		requests: make(map[string]*ApprovalRequest),
	}
}

// List returns all approval requests.
func (s *MemoryStore) List() []*ApprovalRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ApprovalRequest, 0, len(s.requests))
	for _, req := range s.requests {
		result = append(result, req)
	}
	return result
}

// Get returns an approval request by ID.
func (s *MemoryStore) Get(id string) (*ApprovalRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	req, ok := s.requests[id]
	if !ok {
		return nil, fmt.Errorf("approval: request %s not found", id)
	}
	return req, nil
}

// Add stores a new approval request.
func (s *MemoryStore) Add(req *ApprovalRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.requests[req.ID]; ok {
		return fmt.Errorf("approval: request %s already exists", req.ID)
	}
	s.requests[req.ID] = req
	return nil
}

// Update updates an existing approval request.
func (s *MemoryStore) Update(req *ApprovalRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.requests[req.ID]; !ok {
		return fmt.Errorf("approval: request %s not found", req.ID)
	}
	s.requests[req.ID] = req
	return nil
}

// Pending returns all pending approval requests.
func (s *MemoryStore) Pending() []*ApprovalRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ApprovalRequest
	for _, req := range s.requests {
		if req.IsPending() {
			result = append(result, req)
		}
	}
	return result
}

// Count returns the total number of approval requests.
func (s *MemoryStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.requests)
}

// Clear removes all approval requests. Useful for testing.
func (s *MemoryStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.requests = make(map[string]*ApprovalRequest)
}
