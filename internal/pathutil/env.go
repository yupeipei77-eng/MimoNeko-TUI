package pathutil

import (
	"os"
	"strings"
)

// GetEnv returns the value of an environment variable, or the default value if not set.
func GetEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}

// GetEnvTrimmed returns the trimmed value of an environment variable.
func GetEnvTrimmed(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

// GetenvOrDefault is an alias for GetEnv.
func GetenvOrDefault(key, defaultValue string) string {
	return GetEnv(key, defaultValue)
}

// EnvIsSet checks if an environment variable is set and non-empty.
func EnvIsSet(key string) bool {
	return strings.TrimSpace(os.Getenv(key)) != ""
}

// APIKeyStatus checks whether an API key environment variable is configured.
// It returns "configured" if the env var has a non-empty value, "missing" otherwise.
// It never reveals the actual key value.
func APIKeyStatus(envVar string) string {
	if envVar == "" {
		return "missing"
	}
	if EnvIsSet(envVar) {
		return "configured"
	}
	return "missing"
}

// APIKeyIsConfigured checks if an API key is configured.
func APIKeyIsConfigured(envVar string) bool {
	return APIKeyStatus(envVar) == "configured"
}

// ResolveAPIKey returns the API key value for the given environment variable.
// Returns an error if the key is not set.
func ResolveAPIKey(envVar string) (string, error) {
	if envVar == "" {
		return "", os.ErrNotExist
	}
	key := strings.TrimSpace(os.Getenv(envVar))
	if key == "" {
		return "", os.ErrNotExist
	}
	return key, nil
}

// SanitizeEnvValue returns a sanitized version of an environment variable value
// for safe logging (shows first 4 chars + asterisks).
func SanitizeEnvValue(value string) string {
	if len(value) <= 4 {
		return "***"
	}
	return value[:4] + strings.Repeat("*", len(value)-4)
}

// EnvVarNames holds standard environment variable names used by MimoNeko.
type EnvVarNames struct {
	// NekoRoot is the environment variable for the default MimoNeko project root.
	NekoRoot string
	// NekoDefaultRootFile is the environment variable for the default root file path.
	NekoDefaultRootFile string
}

// DefaultEnvVarNames returns the standard environment variable names.
func DefaultEnvVarNames() EnvVarNames {
	return EnvVarNames{
		NekoRoot:            "MimoNeko_NEKO_ROOT",
		NekoDefaultRootFile: "MimoNeko_NEKO_DEFAULT_ROOT_FILE",
	}
}

// NekoRootFromEnv returns the MimoNeko project root from environment variable.
func NekoRootFromEnv() string {
	return GetEnvTrimmed(DefaultEnvVarNames().NekoRoot)
}

// NekoDefaultRootFilePath returns the path to the default root file.
func NekoDefaultRootFilePath() string {
	envPath := GetEnvTrimmed(DefaultEnvVarNames().NekoDefaultRootFile)
	if envPath != "" {
		return envPath
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return JoinPath(configDir, "mimoneko", "neko-default-root.txt")
}
