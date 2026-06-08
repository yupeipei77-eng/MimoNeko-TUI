package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReportStoreSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewReportStore(dir)

	report := &AgentDryRunReport{
		Goal:             "test goal",
		WorkflowID:       "wf_test_123",
		RunID:            "run_test_456",
		Provider:         "mimo",
		Model:            "mimo-v2.5-pro",
		Status:           WorkflowStatusCompleted,
		NoFileWrites:     true,
		NoPatchGenerated: true,
		NoToolsExecuted:  true,
		NoTestsExecuted:  true,
		CreatedAt:        time.Now().UTC(),
		CompletedAt:      time.Now().UTC(),
	}

	// Save
	if err := store.Save(report); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load
	loaded, err := store.Load("wf_test_123")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Goal != report.Goal {
		t.Errorf("Goal = %q, want %q", loaded.Goal, report.Goal)
	}
	if loaded.WorkflowID != report.WorkflowID {
		t.Errorf("WorkflowID = %q, want %q", loaded.WorkflowID, report.WorkflowID)
	}
	if loaded.Provider != report.Provider {
		t.Errorf("Provider = %q, want %q", loaded.Provider, report.Provider)
	}
}

func TestReportStoreLoadNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewReportStore(dir)

	_, err := store.Load("nonexistent")
	if err == nil {
		t.Error("Load() should return error for nonexistent report")
	}
}

func TestReportStoreList(t *testing.T) {
	dir := t.TempDir()
	store := NewReportStore(dir)

	// Save multiple reports
	for i := 0; i < 3; i++ {
		report := &AgentDryRunReport{
			Goal:             "test goal",
			WorkflowID:       "wf_" + string(rune('a'+i)),
			RunID:            "run_test",
			Provider:         "mimo",
			Model:            "mimo-v2.5-pro",
			Status:           WorkflowStatusCompleted,
			NoFileWrites:     true,
			NoPatchGenerated: true,
			NoToolsExecuted:  true,
			NoTestsExecuted:  true,
			CreatedAt:        time.Now().Add(time.Duration(i) * time.Hour).UTC(),
			CompletedAt:      time.Now().UTC(),
		}
		if err := store.Save(report); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}

	// List
	reports, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(reports) != 3 {
		t.Errorf("List() returned %d reports, want 3", len(reports))
	}

	// Verify ordering (newest first)
	if reports[0].CreatedAt.Before(reports[1].CreatedAt) {
		t.Error("Reports should be sorted by created_at descending")
	}
}

func TestReportStoreListEmpty(t *testing.T) {
	dir := t.TempDir()
	store := NewReportStore(dir)

	reports, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(reports) != 0 {
		t.Errorf("List() returned %d reports, want 0", len(reports))
	}
}

func TestReportStoreDelete(t *testing.T) {
	dir := t.TempDir()
	store := NewReportStore(dir)

	report := &AgentDryRunReport{
		Goal:             "test goal",
		WorkflowID:       "wf_test",
		RunID:            "run_test",
		Provider:         "mimo",
		Model:            "mimo-v2.5-pro",
		Status:           WorkflowStatusCompleted,
		NoFileWrites:     true,
		NoPatchGenerated: true,
		NoToolsExecuted:  true,
		NoTestsExecuted:  true,
		CreatedAt:        time.Now().UTC(),
		CompletedAt:      time.Now().UTC(),
	}

	store.Save(report)
	store.Delete("wf_test")

	_, err := store.Load("wf_test")
	if err == nil {
		t.Error("Load() should return error after Delete()")
	}
}

func TestReportStoreDeleteNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewReportStore(dir)

	err := store.Delete("nonexistent")
	if err == nil {
		t.Error("Delete() should return error for nonexistent report")
	}
}

func TestReportStoreSanitizesAPIKey(t *testing.T) {
	dir := t.TempDir()
	store := NewReportStore(dir)

	report := &AgentDryRunReport{
		Goal:             "using API_KEY=sk-abcdefghijklmnopqrstuvwxyz",
		WorkflowID:       "wf_test",
		RunID:            "run_test",
		Provider:         "mimo",
		Model:            "mimo-v2.5-pro",
		Status:           WorkflowStatusCompleted,
		ErrorMessage:     "error with tp-cabcdefghijklmnopqrstuvwxyz",
		NoFileWrites:     true,
		NoPatchGenerated: true,
		NoToolsExecuted:  true,
		NoTestsExecuted:  true,
		CreatedAt:        time.Now().UTC(),
		CompletedAt:      time.Now().UTC(),
	}

	store.Save(report)

	// Read file directly to verify sanitization
	path := filepath.Join(dir, ".mimoneko", "agent_runs", "wf_test.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	content := string(data)
	if containsString(content, "sk-abcdefghijklmnopqrstuvwxyz") {
		t.Errorf("Report file leaked API key: %s", content)
	}
	if containsString(content, "tp-cabcdefghijklmnopqrstuvwxyz") {
		t.Errorf("Report file leaked MiMo key: %s", content)
	}
}

func TestReportStorePathTraversalRejected(t *testing.T) {
	dir := t.TempDir()
	store := NewReportStore(dir)

	// Try path traversal
	report := &AgentDryRunReport{
		Goal:             "test",
		WorkflowID:       "../../../etc/passwd",
		RunID:            "run_test",
		Provider:         "mimo",
		Model:            "mimo-v2.5-pro",
		Status:           WorkflowStatusCompleted,
		NoFileWrites:     true,
		NoPatchGenerated: true,
		NoToolsExecuted:  true,
		NoTestsExecuted:  true,
		CreatedAt:        time.Now().UTC(),
		CompletedAt:      time.Now().UTC(),
	}

	store.Save(report)

	// Verify file is saved in safe location
	expectedPath := filepath.Join(dir, ".mimoneko", "agent_runs", "______etc_passwd.json")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		// Check if it was saved with sanitized name
		entries, _ := os.ReadDir(filepath.Join(dir, ".mimoneko", "agent_runs"))
		if len(entries) == 0 {
			t.Error("Report should be saved with sanitized name")
		}
	}
}

func TestReportStoreCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	store := NewReportStore(dir)

	report := &AgentDryRunReport{
		Goal:             "test",
		WorkflowID:       "wf_test",
		RunID:            "run_test",
		Provider:         "mimo",
		Model:            "mimo-v2.5-pro",
		Status:           WorkflowStatusCompleted,
		NoFileWrites:     true,
		NoPatchGenerated: true,
		NoToolsExecuted:  true,
		NoTestsExecuted:  true,
		CreatedAt:        time.Now().UTC(),
		CompletedAt:      time.Now().UTC(),
	}

	if err := store.Save(report); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify directory was created
	agentRunsDir := filepath.Join(dir, ".mimoneko", "agent_runs")
	if _, err := os.Stat(agentRunsDir); os.IsNotExist(err) {
		t.Error("agent_runs directory should be created")
	}
}

func TestReportStoreInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	store := NewReportStore(dir)

	// Create invalid JSON file
	agentRunsDir := filepath.Join(dir, ".mimoneko", "agent_runs")
	os.MkdirAll(agentRunsDir, 0700)
	os.WriteFile(filepath.Join(agentRunsDir, "wf_test.json"), []byte("invalid json"), 0600)

	_, err := store.Load("wf_test")
	if err == nil {
		t.Error("Load() should return error for invalid JSON")
	}
}

func TestReportStoreNoAPIKeyInFile(t *testing.T) {
	dir := t.TempDir()
	store := NewReportStore(dir)

	report := &AgentDryRunReport{
		Goal:             "test with sk-abcdefghijklmnopqrstuvwxyz and tp-cabcdefghijklmnopqrstuvwxyz",
		WorkflowID:       "wf_test",
		RunID:            "run_test",
		Provider:         "mimo",
		Model:            "mimo-v2.5-pro",
		Status:           WorkflowStatusCompleted,
		ErrorMessage:     "Bearer abcdefghijklmnopqrstuvwxyz",
		NoFileWrites:     true,
		NoPatchGenerated: true,
		NoToolsExecuted:  true,
		NoTestsExecuted:  true,
		CreatedAt:        time.Now().UTC(),
		CompletedAt:      time.Now().UTC(),
	}

	store.Save(report)

	// Load and verify
	loaded, _ := store.Load("wf_test")

	if containsString(loaded.Goal, "sk-abcdefghijklmnopqrstuvwxyz") {
		t.Errorf("Goal leaked API key: %q", loaded.Goal)
	}
	if containsString(loaded.Goal, "tp-cabcdefghijklmnopqrstuvwxyz") {
		t.Errorf("Goal leaked MiMo key: %q", loaded.Goal)
	}
	if containsString(loaded.ErrorMessage, "abcdefghijklmnopqrstuvwxyz") {
		t.Errorf("ErrorMessage leaked token: %q", loaded.ErrorMessage)
	}
}

func TestFormatReportSummary(t *testing.T) {
	report := &AgentDryRunReport{
		Goal:             "test goal",
		WorkflowID:       "wf_test",
		RunID:            "run_test",
		Provider:         "mimo",
		Model:            "mimo-v2.5-pro",
		Status:           WorkflowStatusCompleted,
		NoFileWrites:     true,
		NoPatchGenerated: true,
		NoToolsExecuted:  true,
		NoTestsExecuted:  true,
		CreatedAt:        time.Now().UTC(),
		CompletedAt:      time.Now().UTC(),
	}

	// Verify JSON serialization
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent() error = %v", err)
	}

	var loaded AgentDryRunReport
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if loaded.NoFileWrites != true {
		t.Error("NoFileWrites should be true")
	}
	if loaded.NoPatchGenerated != true {
		t.Error("NoPatchGenerated should be true")
	}
	if loaded.NoToolsExecuted != true {
		t.Error("NoToolsExecuted should be true")
	}
	if loaded.NoTestsExecuted != true {
		t.Error("NoTestsExecuted should be true")
	}
}
