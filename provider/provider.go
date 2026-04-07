package provider

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"
)

// Session represents a coding agent session with normalized metadata.
type Session struct {
	Agent      string    `json:"agent"`
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Summary    string    `json:"summary,omitempty"`
	Slug       string    `json:"slug,omitempty"`
	Created    time.Time `json:"created"`
	LastUsed   time.Time `json:"last_used"`
	Directory  string    `json:"directory,omitempty"`
	SearchText string    `json:"-"`
}

// Provider discovers sessions for a specific coding agent.
type Provider interface {
	// Name returns the display name of the coding agent (e.g. "opencode", "claude").
	Name() string

	// ListSessions returns all available sessions.
	ListSessions(ctx context.Context) ([]Session, error)

	// ResumeCommand returns the shell command to resume a session.
	// The returned string is eval'd by the shell wrapper, so cd + exec patterns work.
	ResumeCommand(session Session) string

	// SessionText returns the concatenated user prompt text for a session,
	// suitable for sending to a summary generator. Returns empty string if
	// the provider doesn't support text extraction.
	SessionText(ctx context.Context, sessionID string) string
}

// DisplayTitle returns the best available display title for a session.
// Prefers a generated summary over the raw title when available.
func (s Session) DisplayTitle() string {
	if s.Summary != "" {
		return s.Summary
	}
	if s.Title != "" {
		return s.Title
	}
	if s.Slug != "" {
		return s.Slug
	}
	return s.ID
}

// RelativeTime formats a time as a human-readable relative string.
func RelativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		return fmt.Sprintf("%dh ago", h)
	case d < 30*24*time.Hour:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	default:
		return t.Format("Jan 2")
	}
}

// ShellQuote quotes a string for safe use in POSIX shell commands (bash/zsh).
func ShellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if isShellSafe(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// ShellQuotePowerShell quotes a string for safe use in PowerShell.
// PowerShell single quotes use doubled quotes for escaping: 'it”s'
func ShellQuotePowerShell(s string) string {
	if s == "" {
		return "''"
	}
	if isShellSafe(s) && !strings.ContainsRune(s, '/') {
		// Forward slashes are safe in POSIX but can confuse PS in some contexts.
		// Backslash paths are fine unquoted.
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// Q quotes a string for the current platform's shell.
func Q(s string) string {
	if runtime.GOOS == "windows" {
		return ShellQuotePowerShell(s)
	}
	return ShellQuote(s)
}

// CdAndRun returns a shell command that changes to dir and runs cmd.
// Emits platform-appropriate syntax: "cd X && Y" on Unix,
// "Set-Location X; Y" on Windows.
func CdAndRun(dir, cmd string) string {
	if dir == "" {
		return cmd
	}
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("Set-Location %s; %s", Q(dir), cmd)
	}
	return fmt.Sprintf("cd %s && %s", Q(dir), cmd)
}

func isShellSafe(s string) bool {
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' ||
			c == '.' || c == '/' || c == ':' || c == '~' || c == '\\') {
			return false
		}
	}
	return true
}
