# internal/pathutil

## Responsibilities

- Provide utility functions for path manipulation and resolution.
- Centralize executable path detection and identification.
- Manage environment variable access with consistent patterns.
- Handle cross-platform path operations (Windows vs Unix).

## Functions

### Executable Functions
- `ExecutablePath()` - Get current executable path
- `ExecutableDir()` - Get directory containing executable
- `ExeName()` - Get executable name (lowercase)
- `IsMimoNekoExe()` - Check if running as MimoNeko
- `IsNekoExe()` - Check if running as neko

### Path Functions
- `AbsPath(path)` - Get absolute path
- `CleanPath(path)` - Clean and normalize path
- `JoinPath(elem...)` - Join path elements
- `FileExists(path)` - Check if file exists
- `DirExists(path)` - Check if directory exists
- `EnsureDir(path)` - Create directory if needed
- `ExpandHome(path)` - Expand ~ to home directory
- `ExpandEnv(path)` - Expand environment variables
- `ResolvePath(base, path)` - Resolve relative path from base
- `RelPath(base, target)` - Get relative path

### Environment Functions
- `GetEnv(key, default)` - Get env with default
- `GetEnvTrimmed(key)` - Get trimmed env value
- `EnvIsSet(key)` - Check if env is set
- `APIKeyStatus(envVar)` - Check API key status
- `ResolveAPIKey(envVar)` - Get API key value
- `NekoRootFromEnv()` - Get MimoNeko root from env

## Boundaries

- This package should only contain pure utility functions.
- No business logic or domain-specific code.
- No dependencies on other internal packages.

## Forbidden

- Do not store state or configuration.
- Do not make network calls.
- Do not access project-specific paths (use config package for that).
