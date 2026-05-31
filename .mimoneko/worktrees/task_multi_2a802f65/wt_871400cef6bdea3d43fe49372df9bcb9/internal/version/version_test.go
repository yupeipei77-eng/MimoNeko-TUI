package version

import "testing"

func TestString(t *testing.T) {
	original := Version
	t.Cleanup(func() {
		Version = original
	})

	Version = "test-version"
	if got := String(); got != "NekoMIMO test-version" {
		t.Fatalf("String() = %q, want NekoMIMO test-version", got)
	}
}
