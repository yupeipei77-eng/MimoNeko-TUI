package pathutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetEnv(t *testing.T) {
	// Test with existing env var
	t.Setenv("TEST_GETENV_VAR", "test-value")
	result := GetEnv("TEST_GETENV_VAR", "default")
	if result != "test-value" {
		t.Errorf("GetEnv(TEST_GETENV_VAR, default) = %q, want %q", result, "test-value")
	}

	// Test with non-existing env var
	result = GetEnv("NONEXISTENT_VAR_12345", "default")
	if result != "default" {
		t.Errorf("GetEnv(NONEXISTENT_VAR, default) = %q, want %q", result, "default")
	}

	// Test with empty env var
	t.Setenv("TEST_GETENV_EMPTY", "")
	result = GetEnv("TEST_GETENV_EMPTY", "default")
	if result != "default" {
		t.Errorf("GetEnv(TEST_GETENV_EMPTY, default) = %q, want %q", result, "default")
	}
}

func TestGetEnvTrimmed(t *testing.T) {
	// Test with value that needs trimming
	t.Setenv("TEST_TRIMMED_VAR", "  test-value  ")
	result := GetEnvTrimmed("TEST_TRIMMED_VAR")
	if result != "test-value" {
		t.Errorf("GetEnvTrimmed(TEST_TRIMMED_VAR) = %q, want %q", result, "test-value")
	}

	// Test with empty var
	result = GetEnvTrimmed("NONEXISTENT_VAR_12345")
	if result != "" {
		t.Errorf("GetEnvTrimmed(NONEXISTENT_VAR) = %q, want empty", result)
	}
}

func TestEnvIsSet(t *testing.T) {
	t.Setenv("TEST_ENV_IS_SET", "value")
	if !EnvIsSet("TEST_ENV_IS_SET") {
		t.Error("EnvIsSet(TEST_ENV_IS_SET) = false, want true")
	}

	if EnvIsSet("NONEXISTENT_VAR_12345") {
		t.Error("EnvIsSet(NONEXISTENT_VAR) = true, want false")
	}

	t.Setenv("TEST_ENV_IS_SET_EMPTY", "")
	if EnvIsSet("TEST_ENV_IS_SET_EMPTY") {
		t.Error("EnvIsSet(TEST_ENV_IS_SET_EMPTY) = true, want false")
	}
}

func TestAPIKeyStatus(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		envValue string
		setEnv   bool
		expected string
	}{
		{"empty env var name", "", "", false, "missing"},
		{"configured key", "TEST_API_KEY", "sk-test", true, "configured"},
		{"missing key", "TEST_API_KEY_MISSING", "", false, "missing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv(tt.envVar, tt.envValue)
			}
			result := APIKeyStatus(tt.envVar)
			if result != tt.expected {
				t.Errorf("APIKeyStatus(%q) = %q, want %q", tt.envVar, result, tt.expected)
			}
		})
	}
}

func TestAPIKeyIsConfigured(t *testing.T) {
	t.Setenv("TEST_API_KEY_CONFIGURED", "sk-test")
	if !APIKeyIsConfigured("TEST_API_KEY_CONFIGURED") {
		t.Error("APIKeyIsConfigured returned false for configured key")
	}

	if APIKeyIsConfigured("NONEXISTENT_API_KEY") {
		t.Error("APIKeyIsConfigured returned true for missing key")
	}
}

func TestResolveAPIKey(t *testing.T) {
	// Test with empty env var
	_, err := ResolveAPIKey("")
	if err == nil {
		t.Error("ResolveAPIKey('') should return error")
	}

	// Test with configured key
	t.Setenv("TEST_RESOLVE_KEY", "sk-test-value")
	key, err := ResolveAPIKey("TEST_RESOLVE_KEY")
	if err != nil {
		t.Errorf("ResolveAPIKey failed: %v", err)
	}
	if key != "sk-test-value" {
		t.Errorf("ResolveAPIKey = %q, want %q", key, "sk-test-value")
	}

	// Test with missing key
	_, err = ResolveAPIKey("NONEXISTENT_KEY_12345")
	if err == nil {
		t.Error("ResolveAPIKey should return error for missing key")
	}
}

func TestSanitizeEnvValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "***"},
		{"ab", "***"},
		{"abcd", "***"},  // len == 4, so <= 4 returns "***"
		{"abcde", "abcd*"},  // len == 5, returns first 4 + 1 asterisk
		{"abcdefgh", "abcd****"},  // len == 8, returns first 4 + 4 asterisks
		{"sk-1234567890", "sk-1*********"},  // len == 12, returns first 4 + 8 asterisks
	}

	for _, tt := range tests {
		result := SanitizeEnvValue(tt.input)
		if result != tt.expected {
			t.Errorf("SanitizeEnvValue(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestDefaultEnvVarNames(t *testing.T) {
	names := DefaultEnvVarNames()
	if names.NekoRoot != "MimoNeko_NEKO_ROOT" {
		t.Errorf("NekoRoot = %q, want %q", names.NekoRoot, "MimoNeko_NEKO_ROOT")
	}
	if names.NekoDefaultRootFile != "MimoNeko_NEKO_DEFAULT_ROOT_FILE" {
		t.Errorf("NekoDefaultRootFile = %q, want %q", names.NekoDefaultRootFile, "MimoNeko_NEKO_DEFAULT_ROOT_FILE")
	}
}

func TestNekoRootFromEnv(t *testing.T) {
	// Test with set env var
	t.Setenv("MimoNeko_NEKO_ROOT", "/test/root")
	result := NekoRootFromEnv()
	if result != "/test/root" {
		t.Errorf("NekoRootFromEnv() = %q, want %q", result, "/test/root")
	}

	// Test without env var (need to unset it)
	os.Unsetenv("MimoNeko_NEKO_ROOT")
	result = NekoRootFromEnv()
	if result != "" {
		t.Errorf("NekoRootFromEnv() = %q, want empty", result)
	}
}

func TestNekoDefaultRootFilePath(t *testing.T) {
	// Test with custom env var
	t.Setenv("MimoNeko_NEKO_DEFAULT_ROOT_FILE", "/custom/path.txt")
	result := NekoDefaultRootFilePath()
	if result != "/custom/path.txt" {
		t.Errorf("NekoDefaultRootFilePath() = %q, want %q", result, "/custom/path.txt")
	}

	// Test default path (unset custom)
	os.Unsetenv("MimoNeko_NEKO_DEFAULT_ROOT_FILE")
	result = NekoDefaultRootFilePath()
	configDir, err := os.UserConfigDir()
	if err == nil {
		expected := filepath.Join(configDir, "mimoneko", "neko-default-root.txt")
		if result != expected {
			t.Errorf("NekoDefaultRootFilePath() = %q, want %q", result, expected)
		}
	}
}

func TestGetenvOrDefault(t *testing.T) {
	t.Setenv("TEST_GETENV_OR_DEFAULT", "value")
	result := GetenvOrDefault("TEST_GETENV_OR_DEFAULT", "default")
	if result != "value" {
		t.Errorf("GetenvOrDefault = %q, want %q", result, "value")
	}

	result = GetenvOrDefault("NONEXISTENT", "default")
	if result != "default" {
		t.Errorf("GetenvOrDefault = %q, want %q", result, "default")
	}
}
