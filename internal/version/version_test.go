package version

import "testing"

func TestString(t *testing.T) {
	original := Version
	t.Cleanup(func() {
		Version = original
	})

	Version = "test-version"
	if got := String(); got != "MimoNeko test-version" {
		t.Fatalf("String() = %q, want MimoNeko test-version", got)
	}
}
