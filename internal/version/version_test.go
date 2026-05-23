package version

import "testing"

func TestString(t *testing.T) {
	original := Version
	t.Cleanup(func() {
		Version = original
	})

	Version = "test-version"
	if got := String(); got != "reasonforge test-version" {
		t.Fatalf("String() = %q, want reasonforge test-version", got)
	}
}
