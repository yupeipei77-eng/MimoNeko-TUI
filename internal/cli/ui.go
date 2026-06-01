package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type KV struct {
	Key   string
	Value string
}

type cliUI struct {
	emoji bool
	color bool
}

func newCLIUI() cliUI {
	return cliUI{
		emoji: SupportsEmoji(),
		color: SupportsColor(),
	}
}

func SupportsEmoji() bool {
	if envFlag("MIMONEKO_NO_EMOJI") || envFlag("NO_COLOR") || strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	return true
}

func SupportsColor() bool {
	if envFlag("NO_COLOR") || strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	return true
}

func envFlag(name string) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return false
	}
	return !strings.EqualFold(value, "0") && !strings.EqualFold(value, "false")
}

func (ui cliUI) Icon(name string) string {
	if ui.emoji {
		switch name {
		case "success":
			return "✅"
		case "warning":
			return "⚠️"
		case "error":
			return "❌"
		case "info":
			return "ℹ️"
		case "model":
			return "🤖"
		case "cache":
			return "⚡"
		case "patch":
			return "📦"
		case "secret":
			return "🔐"
		case "cat":
			return "🐱"
		case "gear":
			return "⚙️"
		}
	}
	switch name {
	case "success":
		return "[OK]"
	case "warning":
		return "[!]"
	case "error":
		return "[X]"
	case "info":
		return "[i]"
	case "model":
		return "[model]"
	case "cache":
		return "[cache]"
	case "patch":
		return "[patch]"
	case "secret":
		return "[auth]"
	case "cat":
		return "[mio]"
	case "gear":
		return "[run]"
	default:
		return ""
	}
}

func PrintHeader(w io.Writer, title string) {
	newCLIUI().PrintHeader(w, title)
}

func (ui cliUI) PrintHeader(w io.Writer, title string) {
	fmt.Fprintf(w, "%s %s\n\n", ui.Icon(headerIcon(title)), title)
}

func PrintSuccess(w io.Writer, message string) {
	ui := newCLIUI()
	fmt.Fprintf(w, "%s %s\n", ui.Icon("success"), message)
}

func PrintWarning(w io.Writer, message string) {
	ui := newCLIUI()
	fmt.Fprintf(w, "%s %s\n", ui.Icon("warning"), message)
}

func PrintInfo(w io.Writer, message string) {
	ui := newCLIUI()
	fmt.Fprintf(w, "%s %s\n", ui.Icon("info"), message)
}

func PrintError(w io.Writer, title, reason, suggestion string) {
	ui := newCLIUI()
	fmt.Fprintf(w, "%s %s\n\n", ui.Icon("error"), title)
	if strings.TrimSpace(reason) != "" {
		fmt.Fprintln(w, "Reason:")
		fmt.Fprintf(w, "%s\n\n", reason)
	}
	if strings.TrimSpace(suggestion) != "" {
		fmt.Fprintln(w, "Try:")
		fmt.Fprintf(w, "%s\n", suggestion)
	}
}

func PrintErrorDetails(w io.Writer, title, reason, suggestion, details string) {
	PrintError(w, title, reason, suggestion)
	if strings.TrimSpace(details) != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Details:")
		fmt.Fprintln(w, details)
	}
}

func PrintKV(w io.Writer, title string, rows []KV) {
	newCLIUI().PrintKV(w, title, rows)
}

func (ui cliUI) PrintKV(w io.Writer, title string, rows []KV) {
	if strings.TrimSpace(title) != "" {
		fmt.Fprintln(w, title)
	}
	width := 0
	for _, row := range rows {
		if len(row.Key) > width {
			width = len(row.Key)
		}
	}
	if width < 8 {
		width = 8
	}
	if width > 16 {
		width = 16
	}
	for _, row := range rows {
		fmt.Fprintf(w, "%-*s %s\n", width, row.Key, row.Value)
	}
}

func PrintStep(w io.Writer, current, total int, title string) {
	fmt.Fprintf(w, "Step %d/%d  %s\n", current, total, title)
}

func MaskSecret(secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "missing"
	}
	if len(secret) <= 8 {
		if len(secret) <= 4 {
			return "***"
		}
		return secret[:2] + "****" + secret[len(secret)-2:]
	}
	return secret[:4] + "****" + secret[len(secret)-4:]
}

func statusValue(ok bool, okText, failText string) string {
	ui := newCLIUI()
	if ok {
		return ui.Icon("success") + " " + okText
	}
	return ui.Icon("error") + " " + failText
}

func headerIcon(title string) string {
	switch {
	case strings.Contains(title, "Cache"):
		return "cache"
	case strings.Contains(title, "Patch"):
		return "patch"
	case strings.Contains(title, "Auth") || strings.Contains(title, "Config"):
		return "secret"
	case strings.Contains(title, "Model"):
		return "model"
	case strings.Contains(title, "Run"):
		return "cat"
	default:
		return "cat"
	}
}

func percent(value float64) string {
	return fmt.Sprintf("%.2f%%", value*100)
}

func friendlyModelError(raw string) (reason, suggestion, details string) {
	details = strings.TrimSpace(raw)
	lower := strings.ToLower(details)
	switch {
	case strings.Contains(lower, "status 401") || strings.Contains(lower, "unauthorized"):
		return "API Key may be invalid.", "mimoneko auth login", "HTTP 401"
	case strings.Contains(lower, "status 403") || strings.Contains(lower, "forbidden"):
		return "API Key may not have permission for this model.", "Check the key permissions, then run: mimoneko auth login", "HTTP 403"
	case strings.Contains(lower, "status 404") || strings.Contains(lower, "not found"):
		return "Base URL or model name may be wrong.", "Check Base URL and Model, then run: mimoneko auth login", "HTTP 404"
	case strings.Contains(lower, "status 429") || strings.Contains(lower, "too many requests"):
		return "Quota may be exhausted or requests are too fast.", "Wait a moment, check quota, then retry.", "HTTP 429"
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline exceeded"):
		return "Network or server timeout.", "Check your network, then run: mimoneko model test", "timeout"
	case details != "":
		return "The model request did not complete.", "mimoneko model test", details
	default:
		return "The request failed.", "mimoneko model test", ""
	}
}
