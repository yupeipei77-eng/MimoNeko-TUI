package contextengine

import "testing"

func TestPrefixFingerprintStableForSameInput(t *testing.T) {
	snapshot := NewObservableSnapshot(
		map[string]string{"system_prompt": "stable", "tool_schema": "stable"},
		map[string]string{"repo_index": "semi"},
		map[string]string{"user_input": "volatile"},
	)

	first := snapshot.PrefixFingerprint()
	second := snapshot.PrefixFingerprint()
	if first == "" {
		t.Fatal("fingerprint is empty")
	}
	if first != second {
		t.Fatalf("fingerprint changed for same input: %s != %s", first, second)
	}
}

func TestPrefixFingerprintIgnoresEntryOrder(t *testing.T) {
	first := ObservableSnapshot{
		ImmutablePrefix: []ObservableEntry{
			{Key: "tool_schema", Value: "stable"},
			{Key: "system_prompt", Value: "stable"},
		},
	}
	second := ObservableSnapshot{
		ImmutablePrefix: []ObservableEntry{
			{Key: "system_prompt", Value: "stable"},
			{Key: "tool_schema", Value: "stable"},
		},
	}

	if first.PrefixFingerprint() != second.PrefixFingerprint() {
		t.Fatalf("fingerprint should ignore entry order: %s != %s", first.PrefixFingerprint(), second.PrefixFingerprint())
	}
}

func TestVolatileContextDoesNotChangeImmutableFingerprint(t *testing.T) {
	first := NewObservableSnapshot(
		map[string]string{"system_prompt": "stable"},
		map[string]string{"repo_index": "semi"},
		map[string]string{"user_input": "first"},
	)
	second := NewObservableSnapshot(
		map[string]string{"system_prompt": "stable"},
		map[string]string{"repo_index": "semi"},
		map[string]string{"user_input": "second"},
	)

	if first.PrefixFingerprint() != second.PrefixFingerprint() {
		t.Fatalf("volatile context changed immutable fingerprint: %s != %s", first.PrefixFingerprint(), second.PrefixFingerprint())
	}
}

func TestImmutablePrefixChangeChangesFingerprint(t *testing.T) {
	first := NewObservableSnapshot(
		map[string]string{"system_prompt": "stable"},
		nil,
		nil,
	)
	second := NewObservableSnapshot(
		map[string]string{"system_prompt": "changed"},
		nil,
		nil,
	)

	if first.PrefixFingerprint() == second.PrefixFingerprint() {
		t.Fatalf("immutable prefix change did not change fingerprint: %s", first.PrefixFingerprint())
	}
}
