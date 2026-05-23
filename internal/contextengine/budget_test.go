package contextengine

import "testing"

func TestBudgetOK(t *testing.T) {
	guard, err := NewBudgetGuard(BudgetThresholds{WarnRatio: 0.8, BlockRatio: 1.0})
	if err != nil {
		t.Fatalf("NewBudgetGuard() error: %v", err)
	}

	status := guard.Check(50, 100)
	if status.Level != BudgetOK {
		t.Errorf("Check(50, 100) level = %s, want ok", status.Level)
	}
}

func TestBudgetWARN(t *testing.T) {
	guard, _ := NewBudgetGuard(BudgetThresholds{WarnRatio: 0.8, BlockRatio: 1.0})

	status := guard.Check(85, 100)
	if status.Level != BudgetWARN {
		t.Errorf("Check(85, 100) level = %s, want warn", status.Level)
	}
}

func TestBudgetBLOCK(t *testing.T) {
	guard, _ := NewBudgetGuard(BudgetThresholds{WarnRatio: 0.8, BlockRatio: 1.0})

	status := guard.Check(105, 100)
	if status.Level != BudgetBLOCK {
		t.Errorf("Check(105, 100) level = %s, want block", status.Level)
	}
}

func TestBudgetZeroBudget(t *testing.T) {
	guard, _ := NewBudgetGuard(BudgetThresholds{WarnRatio: 0.8, BlockRatio: 1.0})

	status := guard.Check(1000, 0)
	if status.Level != BudgetOK {
		t.Errorf("Check(1000, 0) level = %s, want ok", status.Level)
	}
}

func TestDefaultThresholds(t *testing.T) {
	guard, err := NewBudgetGuard(BudgetThresholds{})
	if err != nil {
		t.Fatalf("NewBudgetGuard() with empty thresholds error: %v", err)
	}

	// Should use defaults: warn=0.8, block=1.0
	status := guard.Check(50, 100)
	if status.Level != BudgetOK {
		t.Errorf("Check(50, 100) with defaults = %s, want ok", status.Level)
	}

	status = guard.Check(85, 100)
	if status.Level != BudgetWARN {
		t.Errorf("Check(85, 100) with defaults = %s, want warn", status.Level)
	}

	status = guard.Check(100, 100)
	if status.Level != BudgetBLOCK {
		t.Errorf("Check(100, 100) with defaults = %s, want block", status.Level)
	}
}

func TestInvalidThresholds(t *testing.T) {
	_, err := NewBudgetGuard(BudgetThresholds{WarnRatio: 1.0, BlockRatio: 0.8})
	if err == nil {
		t.Error("NewBudgetGuard() should reject warn >= block")
	}
}

func TestBudgetUtilization(t *testing.T) {
	guard, _ := NewBudgetGuard(BudgetThresholds{WarnRatio: 0.8, BlockRatio: 1.0})

	status := guard.Check(75, 100)
	if status.Utilization != 0.75 {
		t.Errorf("Utilization = %f, want 0.75", status.Utilization)
	}
}

// Test 11: Budget WARN exact boundary at 80%
func TestBudgetWARNExactBoundary(t *testing.T) {
	guard, _ := NewBudgetGuard(BudgetThresholds{WarnRatio: 0.8, BlockRatio: 1.0})

	// Exactly 80% should be WARN (>= threshold)
	status := guard.Check(80, 100)
	if status.Level != BudgetWARN {
		t.Errorf("Check(80, 100) level = %s, want warn (exact 80%% boundary)", status.Level)
	}
	if status.Utilization != 0.8 {
		t.Errorf("Utilization = %f, want 0.8", status.Utilization)
	}

	// Just below 80% should be OK
	status = guard.Check(79, 100)
	if status.Level != BudgetOK {
		t.Errorf("Check(79, 100) level = %s, want ok (just below 80%%)", status.Level)
	}

	// Just above 80% should be WARN
	status = guard.Check(81, 100)
	if status.Level != BudgetWARN {
		t.Errorf("Check(81, 100) level = %s, want warn (just above 80%%)", status.Level)
	}

	// Exactly 100% should be BLOCK (>= block_ratio)
	status = guard.Check(100, 100)
	if status.Level != BudgetBLOCK {
		t.Errorf("Check(100, 100) level = %s, want block (exact 100%% boundary)", status.Level)
	}
}
