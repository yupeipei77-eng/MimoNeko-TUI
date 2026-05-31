package contextengine

import "fmt"

// BudgetLevel represents the severity of token budget usage.
type BudgetLevel string

const (
	BudgetOK    BudgetLevel = "ok"
	BudgetWARN  BudgetLevel = "warn"
	BudgetBLOCK BudgetLevel = "block"
)

// BudgetThresholds defines the ratios at which warnings and blocks occur.
type BudgetThresholds struct {
	WarnRatio  float64
	BlockRatio float64
}

// BudgetStatus describes the result of a budget check.
type BudgetStatus struct {
	Level       BudgetLevel
	Used        int
	Budget      int
	Utilization float64
}

// String returns a human-readable representation of the budget status.
func (s BudgetStatus) String() string {
	return fmt.Sprintf("level=%s used=%d budget=%d utilization=%.2f", s.Level, s.Used, s.Budget, s.Utilization)
}

// BudgetGuard checks token usage against configurable thresholds.
type BudgetGuard struct {
	thresholds BudgetThresholds
}

// NewBudgetGuard creates a new guard. If thresholds have zero values,
// defaults are applied (warn=0.8, block=1.0). Returns an error if
// WarnRatio >= BlockRatio after defaults are applied.
func NewBudgetGuard(thresholds BudgetThresholds) (*BudgetGuard, error) {
	warn := thresholds.WarnRatio
	if warn <= 0 {
		warn = 0.8
	}
	block := thresholds.BlockRatio
	if block <= 0 {
		block = 1.0
	}

	if warn >= block {
		return nil, fmt.Errorf("budget warn_ratio (%.2f) must be less than block_ratio (%.2f)", warn, block)
	}

	return &BudgetGuard{thresholds: BudgetThresholds{
		WarnRatio:  warn,
		BlockRatio: block,
	}}, nil
}

// Check evaluates token usage against the budget and returns a BudgetStatus.
func (g *BudgetGuard) Check(used int, budget int) BudgetStatus {
	if budget <= 0 {
		return BudgetStatus{Level: BudgetOK, Used: used, Budget: budget, Utilization: 0}
	}

	utilization := float64(used) / float64(budget)

	var level BudgetLevel
	switch {
	case utilization >= g.thresholds.BlockRatio:
		level = BudgetBLOCK
	case utilization >= g.thresholds.WarnRatio:
		level = BudgetWARN
	default:
		level = BudgetOK
	}

	return BudgetStatus{
		Level:       level,
		Used:        used,
		Budget:      budget,
		Utilization: utilization,
	}
}
