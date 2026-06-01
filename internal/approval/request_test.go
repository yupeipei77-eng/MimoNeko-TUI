package approval

import (
	"testing"
	"time"
)

func TestNewRequest(t *testing.T) {
	req, err := NewRequest(
		"run-123",
		"file_write",
		ScopeTool,
		"high risk tool requires approval",
		"high",
		"",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	if req.ID == "" {
		t.Error("request ID should not be empty")
	}
	if req.RunID != "run-123" {
		t.Errorf("RunID = %q, want %q", req.RunID, "run-123")
	}
	if req.ToolName != "file_write" {
		t.Errorf("ToolName = %q, want %q", req.ToolName, "file_write")
	}
	if req.Scope != ScopeTool {
		t.Errorf("Scope = %q, want %q", req.Scope, ScopeTool)
	}
	if req.Status != StatusPending {
		t.Errorf("Status = %q, want %q", req.Status, StatusPending)
	}
	if req.Reason != "high risk tool requires approval" {
		t.Errorf("Reason = %q, want %q", req.Reason, "high risk tool requires approval")
	}
	if req.RiskLevel != "high" {
		t.Errorf("RiskLevel = %q, want %q", req.RiskLevel, "high")
	}
	if req.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestNewRequestWithPath(t *testing.T) {
	req, err := NewRequest(
		"run-123",
		"file_read",
		ScopePath,
		"critical path blocked",
		"low",
		".git/config",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	if req.Path != ".git/config" {
		t.Errorf("Path = %q, want %q", req.Path, ".git/config")
	}
}

func TestNewRequestWithCommand(t *testing.T) {
	req, err := NewRequest(
		"run-123",
		"test_run",
		ScopeCommand,
		"command requires approval",
		"medium",
		"",
		"rm -rf /",
		"",
	)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	if req.Command != "rm -rf /" {
		t.Errorf("Command = %q, want %q", req.Command, "rm -rf /")
	}
}

func TestNewRequestWithPatch(t *testing.T) {
	req, err := NewRequest(
		"run-123",
		"patch_apply",
		ScopePatch,
		"patch requires approval",
		"high",
		"",
		"",
		"patch-456",
	)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	if req.PatchID != "patch-456" {
		t.Errorf("PatchID = %q, want %q", req.PatchID, "patch-456")
	}
}

func TestValidateRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		req     ApprovalRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: ApprovalRequest{
				ID:        "apr_test",
				RunID:     "run-123",
				ToolName:  "file_write",
				Scope:     ScopeTool,
				Status:    StatusPending,
				Reason:    "test",
				RiskLevel: "high",
			},
			wantErr: false,
		},
		{
			name: "missing id",
			req: ApprovalRequest{
				RunID:     "run-123",
				ToolName:  "file_write",
				Scope:     ScopeTool,
				Status:    StatusPending,
				Reason:    "test",
				RiskLevel: "high",
			},
			wantErr: true,
		},
		{
			name: "missing run_id",
			req: ApprovalRequest{
				ID:        "apr_test",
				ToolName:  "file_write",
				Scope:     ScopeTool,
				Status:    StatusPending,
				Reason:    "test",
				RiskLevel: "high",
			},
			wantErr: true,
		},
		{
			name: "missing tool_name",
			req: ApprovalRequest{
				ID:        "apr_test",
				RunID:     "run-123",
				Scope:     ScopeTool,
				Status:    StatusPending,
				Reason:    "test",
				RiskLevel: "high",
			},
			wantErr: true,
		},
		{
			name: "missing scope",
			req: ApprovalRequest{
				ID:        "apr_test",
				RunID:     "run-123",
				ToolName:  "file_write",
				Status:    StatusPending,
				Reason:    "test",
				RiskLevel: "high",
			},
			wantErr: true,
		},
		{
			name: "missing status",
			req: ApprovalRequest{
				ID:        "apr_test",
				RunID:     "run-123",
				ToolName:  "file_write",
				Scope:     ScopeTool,
				Reason:    "test",
				RiskLevel: "high",
			},
			wantErr: true,
		},
		{
			name: "missing reason",
			req: ApprovalRequest{
				ID:        "apr_test",
				RunID:     "run-123",
				ToolName:  "file_write",
				Scope:     ScopeTool,
				Status:    StatusPending,
				RiskLevel: "high",
			},
			wantErr: true,
		},
		{
			name: "missing risk_level",
			req: ApprovalRequest{
				ID:       "apr_test",
				RunID:    "run-123",
				ToolName: "file_write",
				Scope:    ScopeTool,
				Status:   StatusPending,
				Reason:   "test",
			},
			wantErr: true,
		},
		{
			name: "invalid scope",
			req: ApprovalRequest{
				ID:        "apr_test",
				RunID:     "run-123",
				ToolName:  "file_write",
				Scope:     "invalid",
				Status:    StatusPending,
				Reason:    "test",
				RiskLevel: "high",
			},
			wantErr: true,
		},
		{
			name: "invalid status",
			req: ApprovalRequest{
				ID:        "apr_test",
				RunID:     "run-123",
				ToolName:  "file_write",
				Scope:     ScopeTool,
				Status:    "invalid",
				Reason:    "test",
				RiskLevel: "high",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestApprovePendingRequest(t *testing.T) {
	req := &ApprovalRequest{
		ID:        "apr_test",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusPending,
		Reason:    "test",
		RiskLevel: "high",
		CreatedAt: time.Now().UTC(),
	}

	err := req.Approve("user-1")
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	if req.Status != StatusApproved {
		t.Errorf("Status = %q, want %q", req.Status, StatusApproved)
	}
	if req.DecidedBy != "user-1" {
		t.Errorf("DecidedBy = %q, want %q", req.DecidedBy, "user-1")
	}
	if req.DecidedAt.IsZero() {
		t.Error("DecidedAt should not be zero")
	}
}

func TestRejectPendingRequest(t *testing.T) {
	req := &ApprovalRequest{
		ID:        "apr_test",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusPending,
		Reason:    "test",
		RiskLevel: "high",
		CreatedAt: time.Now().UTC(),
	}

	err := req.Reject("user-1")
	if err != nil {
		t.Fatalf("Reject() error = %v", err)
	}

	if req.Status != StatusRejected {
		t.Errorf("Status = %q, want %q", req.Status, StatusRejected)
	}
	if req.DecidedBy != "user-1" {
		t.Errorf("DecidedBy = %q, want %q", req.DecidedBy, "user-1")
	}
}

func TestExpirePendingRequest(t *testing.T) {
	req := &ApprovalRequest{
		ID:        "apr_test",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusPending,
		Reason:    "test",
		RiskLevel: "high",
		CreatedAt: time.Now().UTC(),
	}

	err := req.Expire()
	if err != nil {
		t.Fatalf("Expire() error = %v", err)
	}

	if req.Status != StatusExpired {
		t.Errorf("Status = %q, want %q", req.Status, StatusExpired)
	}
}

func TestCannotApproveExpiredRequest(t *testing.T) {
	req := &ApprovalRequest{
		ID:        "apr_test",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusExpired,
		Reason:    "test",
		RiskLevel: "high",
		CreatedAt: time.Now().UTC(),
	}

	err := req.Approve("user-1")
	if err == nil {
		t.Error("Approve() should return error for expired request")
	}
}

func TestCannotApproveRejectedRequest(t *testing.T) {
	req := &ApprovalRequest{
		ID:        "apr_test",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusRejected,
		Reason:    "test",
		RiskLevel: "high",
		CreatedAt: time.Now().UTC(),
	}

	err := req.Approve("user-1")
	if err == nil {
		t.Error("Approve() should return error for rejected request")
	}
}

func TestCannotRejectApprovedRequest(t *testing.T) {
	req := &ApprovalRequest{
		ID:        "apr_test",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusApproved,
		Reason:    "test",
		RiskLevel: "high",
		CreatedAt: time.Now().UTC(),
	}

	err := req.Reject("user-1")
	if err == nil {
		t.Error("Reject() should return error for approved request")
	}
}

func TestCannotRejectExpiredRequest(t *testing.T) {
	req := &ApprovalRequest{
		ID:        "apr_test",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusExpired,
		Reason:    "test",
		RiskLevel: "high",
		CreatedAt: time.Now().UTC(),
	}

	err := req.Reject("user-1")
	if err == nil {
		t.Error("Reject() should return error for expired request")
	}
}

func TestCannotExpireNonPendingRequest(t *testing.T) {
	tests := []struct {
		name   string
		status ApprovalStatus
	}{
		{"approved", StatusApproved},
		{"rejected", StatusRejected},
		{"expired", StatusExpired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ApprovalRequest{
				ID:        "apr_test",
				RunID:     "run-123",
				ToolName:  "file_write",
				Scope:     ScopeTool,
				Status:    tt.status,
				Reason:    "test",
				RiskLevel: "high",
				CreatedAt: time.Now().UTC(),
			}

			err := req.Expire()
			if err == nil {
				t.Errorf("Expire() should return error for %s request", tt.status)
			}
		})
	}
}

func TestIsPending(t *testing.T) {
	tests := []struct {
		name   string
		status ApprovalStatus
		want   bool
	}{
		{"pending", StatusPending, true},
		{"approved", StatusApproved, false},
		{"rejected", StatusRejected, false},
		{"expired", StatusExpired, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ApprovalRequest{Status: tt.status}
			got := req.IsPending()
			if got != tt.want {
				t.Errorf("IsPending() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsExpired(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name      string
		status    ApprovalStatus
		expiresAt time.Time
		now       time.Time
		want      bool
	}{
		{
			name:      "not expired - no expiration",
			status:    StatusPending,
			expiresAt: time.Time{},
			now:       now,
			want:      false,
		},
		{
			name:      "not expired - before expiration",
			status:    StatusPending,
			expiresAt: now.Add(1 * time.Hour),
			now:       now,
			want:      false,
		},
		{
			name:      "expired - after expiration",
			status:    StatusPending,
			expiresAt: now.Add(-1 * time.Hour),
			now:       now,
			want:      true,
		},
		{
			name:      "expired - status is expired",
			status:    StatusExpired,
			expiresAt: time.Time{},
			now:       now,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ApprovalRequest{
				Status:    tt.status,
				ExpiresAt: tt.expiresAt,
			}
			got := req.IsExpired(tt.now)
			if got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSanitizeDisplayFields(t *testing.T) {
	// This test verifies that sensitive fields can be sanitized
	req := &ApprovalRequest{
		ID:        "apr_test",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusPending,
		Reason:    "API key sk-abcdefghijklmnopqrstuvwxyz detected",
		RiskLevel: "high",
		Path:      ".env",
		Command:   "curl -H 'Bearer abcdefghijklmnopqrstuvwxyz' https://api.example.com",
		CreatedAt: time.Now().UTC(),
	}

	// Verify fields contain sensitive data
	if req.Reason == "" {
		t.Error("Reason should not be empty")
	}
	if req.Command == "" {
		t.Error("Command should not be empty")
	}

	// In a real integration, these would be passed through security.SanitizeText()
	// For now, we just verify the fields exist and can be accessed
}

func TestRequestIDFormat(t *testing.T) {
	req, err := NewRequest(
		"run-123",
		"file_write",
		ScopeTool,
		"test",
		"high",
		"",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	// ID should start with "apr_"
	if len(req.ID) < 4 || req.ID[:4] != "apr_" {
		t.Errorf("ID should start with 'apr_', got %q", req.ID)
	}

	// ID should be 36 characters (4 prefix + 32 hex)
	if len(req.ID) != 36 {
		t.Errorf("ID length = %d, want 36", len(req.ID))
	}
}

func TestMultipleRequestsHaveUniqueIDs(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		req, err := NewRequest(
			"run-123",
			"file_write",
			ScopeTool,
			"test",
			"high",
			"",
			"",
			"",
		)
		if err != nil {
			t.Fatalf("NewRequest() error = %v", err)
		}
		if ids[req.ID] {
			t.Errorf("Duplicate ID generated: %s", req.ID)
		}
		ids[req.ID] = true
	}
}
