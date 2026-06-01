package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mimoneko/mimoneko/internal/security"
)

// ReportStore manages persistence of AgentDryRunReport.
type ReportStore struct {
	dir string
}

// NewReportStore creates a new ReportStore.
// root is the project root directory.
func NewReportStore(root string) *ReportStore {
	return &ReportStore{
		dir: filepath.Join(root, ".mimoneko", "agent_runs"),
	}
}

// Save saves a report to disk.
// The report is sanitized before saving (no API keys, tokens, etc.).
func (s *ReportStore) Save(report *AgentDryRunReport) error {
	// Create directory if not exists
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return fmt.Errorf("report store: create directory: %w", err)
	}

	// Sanitize report before saving
	sanitized := sanitizeReport(report)

	// Marshal to JSON
	data, err := json.MarshalIndent(sanitized, "", "  ")
	if err != nil {
		return fmt.Errorf("report store: marshal JSON: %w", err)
	}

	// Write to file
	path := s.reportPath(report.WorkflowID)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("report store: write file: %w", err)
	}

	return nil
}

// Load loads a report by workflow ID.
func (s *ReportStore) Load(workflowID string) (*AgentDryRunReport, error) {
	path := s.reportPath(workflowID)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("report store: report %q not found", workflowID)
		}
		return nil, fmt.Errorf("report store: read file: %w", err)
	}

	var report AgentDryRunReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("report store: parse JSON: %w", err)
	}

	return &report, nil
}

// List returns all reports sorted by created_at descending.
func (s *ReportStore) List() ([]*AgentDryRunReport, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("report store: read directory: %w", err)
	}

	var reports []*AgentDryRunReport
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(s.dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var report AgentDryRunReport
		if err := json.Unmarshal(data, &report); err != nil {
			continue
		}

		reports = append(reports, &report)
	}

	// Sort by created_at descending
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].CreatedAt.After(reports[j].CreatedAt)
	})

	return reports, nil
}

// Delete deletes a report by workflow ID.
func (s *ReportStore) Delete(workflowID string) error {
	path := s.reportPath(workflowID)

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("report store: report %q not found", workflowID)
		}
		return fmt.Errorf("report store: delete file: %w", err)
	}

	return nil
}

// reportPath returns the file path for a report.
func (s *ReportStore) reportPath(workflowID string) string {
	// Sanitize workflow ID to prevent path traversal
	safeID := strings.ReplaceAll(workflowID, "/", "_")
	safeID = strings.ReplaceAll(safeID, "\\", "_")
	safeID = strings.ReplaceAll(safeID, "..", "_")
	return filepath.Join(s.dir, safeID+".json")
}

// sanitizeReport creates a sanitized copy of the report for persistence.
// Removes any potential secrets from the report.
func sanitizeReport(report *AgentDryRunReport) *AgentDryRunReport {
	sanitized := *report

	// Sanitize goal
	sanitized.Goal = security.SanitizeText(report.Goal)

	// Sanitize provider/model (keep as-is, they don't contain secrets)
	sanitized.Provider = report.Provider
	sanitized.Model = report.Model

	// Sanitize error message
	if sanitized.ErrorMessage != "" {
		sanitized.ErrorMessage = security.SanitizeText(report.ErrorMessage)
	}

	// Sanitize planner plan
	if sanitized.PlannerPlan != nil {
		planCopy := *sanitized.PlannerPlan
		planCopy.Goal = security.SanitizeText(planCopy.Goal)
		planCopy.Summary = security.SanitizeText(planCopy.Summary)
		sanitized.PlannerPlan = &planCopy
	}

	// Sanitize coder intent
	if sanitized.CoderIntent != nil {
		intentCopy := *sanitized.CoderIntent
		intentCopy.Goal = security.SanitizeText(intentCopy.Goal)
		intentCopy.PlanSummary = security.SanitizeText(intentCopy.PlanSummary)
		sanitized.CoderIntent = &intentCopy
	}

	// Sanitize reviewer review
	if sanitized.ReviewerReview != nil {
		reviewCopy := *sanitized.ReviewerReview
		reviewCopy.Goal = security.SanitizeText(reviewCopy.Goal)
		reviewCopy.Summary = security.SanitizeText(reviewCopy.Summary)
		sanitized.ReviewerReview = &reviewCopy
	}

	// Sanitize validator suggestions
	if sanitized.ValidatorSuggestions != nil {
		suggestionsCopy := *sanitized.ValidatorSuggestions
		suggestionsCopy.Goal = security.SanitizeText(suggestionsCopy.Goal)
		suggestionsCopy.Summary = security.SanitizeText(suggestionsCopy.Summary)
		sanitized.ValidatorSuggestions = &suggestionsCopy
	}

	return &sanitized
}
