package pathutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ExecutablePath returns the path of the current executable.
// Returns empty string if the path cannot be determined.
func ExecutablePath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe
}

// ExecutableDir returns the directory containing the current executable.
func ExecutableDir() string {
	exe := ExecutablePath()
	if exe == "" {
		return ""
	}
	return filepath.Dir(exe)
}

// ExeName returns the base name of the current executable (lowercase, without extension).
func ExeName() string {
	exe := ExecutablePath()
	if exe == "" {
		return ""
	}
	return strings.ToLower(filepath.Base(exe))
}

// IsMimoNekoExe checks if the current executable is MimoNeko.
func IsMimoNekoExe() bool {
	name := ExeName()
	return strings.Contains(name, "mimoneko")
}

// IsNekoExe checks if the current executable is neko.
func IsNekoExe() bool {
	name := ExeName()
	return strings.Contains(name, "neko") && !strings.Contains(name, "mimoneko")
}

// IsWindows returns true if running on Windows.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// IsDarwin returns true if running on macOS.
func IsDarwin() bool {
	return runtime.GOOS == "darwin"
}

// IsLinux returns true if running on Linux.
func IsLinux() bool {
	return runtime.GOOS == "linux"
}

// GOOS returns the current operating system.
func GOOS() string {
	return runtime.GOOS
}

// AbsPath returns the absolute path, or the original path if conversion fails.
func AbsPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

// AbsPathOrDefault returns the absolute path, or the default value if conversion fails.
func AbsPathOrDefault(path, defaultPath string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return defaultPath
	}
	return abs
}

// CleanPath cleans and normalizes a path.
func CleanPath(path string) string {
	return filepath.Clean(path)
}

// JoinPath joins path elements and cleans the result.
func JoinPath(elem ...string) string {
	return filepath.Clean(filepath.Join(elem...))
}

// FileExists checks if a file exists.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// DirExists checks if a directory exists.
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// EnsureDir creates a directory if it doesn't exist.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o700)
}

// ExpandHome expands ~ to the user's home directory.
func ExpandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimLeft(strings.TrimPrefix(path, "~"), `\/`))
}

// ExpandEnv expands environment variables in a path.
// Supports both $VAR and %VAR% syntax on Windows.
func ExpandEnv(path string) string {
	if IsWindows() {
		path = expandWindowsEnv(path)
	}
	return os.ExpandEnv(path)
}

func expandWindowsEnv(value string) string {
	result := value
	start := 0
	for {
		i := strings.Index(result[start:], "%")
		if i < 0 {
			break
		}
		i += start
		j := strings.Index(result[i+1:], "%")
		if j < 0 {
			break
		}
		j += i + 1
		varName := result[i+1 : j]
		if expanded, ok := os.LookupEnv(varName); ok {
			result = result[:i] + expanded + result[j+1:]
			start = i + len(expanded)
		} else {
			start = j + 1
		}
	}
	return result
}

// Getwd returns the current working directory.
// Returns empty string if the directory cannot be determined.
func Getwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

// ResolvePath resolves a path relative to a base directory.
// If the path is absolute, it returns the cleaned absolute path.
// If the path is relative, it joins it with the base directory.
func ResolvePath(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(base, path)
}

// RelPath returns a relative path from base to target.
// Returns the target path if conversion fails.
func RelPath(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return rel
}
