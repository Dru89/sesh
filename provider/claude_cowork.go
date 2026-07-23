package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// ClaudeCowork reads Claude Cowork sessions — the local agent-mode
// sessions run by the Claude desktop app. They are stored separately from
// Claude Code CLI sessions (handled by the Claude provider): Cowork keeps
// its own metadata and transcript files under the desktop app's Electron
// userData directory. This covers Cowork sessions only — the desktop app's
// Chat tab conversations live in separate web storage (the claude.ai
// IndexedDB) and are not surfaced here.
type ClaudeCowork struct {
	baseDir       string
	resumeCommand string // override for resume command template
}

// ClaudeCoworkOption configures the ClaudeCowork provider.
type ClaudeCoworkOption func(*ClaudeCowork)

// WithClaudeCoworkResumeCommand overrides the default resume command template.
// Use {{ID}} as a placeholder for the session ID.
func WithClaudeCoworkResumeCommand(cmd string) ClaudeCoworkOption {
	return func(c *ClaudeCowork) {
		c.resumeCommand = cmd
	}
}

// NewClaudeCowork creates a ClaudeCowork provider rooted at the platform's
// Electron userData directory for the Claude desktop app:
//   - macOS:   ~/Library/Application Support/Claude
//   - Windows: %AppData%\Claude
//   - Linux:   ~/.config/Claude
func NewClaudeCowork(opts ...ClaudeCoworkOption) *ClaudeCowork {
	c := &ClaudeCowork{}
	if cfgDir, err := os.UserConfigDir(); err == nil {
		c.baseDir = filepath.Join(cfgDir, "Claude")
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *ClaudeCowork) Name() string { return "claude-cowork" }

// claudeCoworkMetadata is the shape of a local_<uuid>.json session metadata file.
type claudeCoworkMetadata struct {
	SessionID           string   `json:"sessionId"`
	CLISessionID        string   `json:"cliSessionId"`
	Title               string   `json:"title"`
	UserSelectedFolders []string `json:"userSelectedFolders"`
	CreatedAt           int64    `json:"createdAt"`
	LastActivityAt      int64    `json:"lastActivityAt"`
	LastFocusedAt       int64    `json:"lastFocusedAt"`
	InitialMessage      string   `json:"initialMessage"`
	ScheduledTaskID     string   `json:"scheduledTaskId"`
	SessionType         string   `json:"sessionType"`
	Model               string   `json:"model"`
	IsArchived          bool     `json:"isArchived"`
}

// ListSessions walks <baseDir>/local-agent-mode-sessions/<uuid>/<uuid>/local_<uuid>.json
// metadata files and builds a Session for each. It does not read
// <baseDir>/claude-code-sessions — those are the app's own VM/terminal
// Claude Code sessions, already surfaced by the built-in Claude provider.
func (c *ClaudeCowork) ListSessions(ctx context.Context) ([]Session, error) {
	if c.baseDir == "" {
		return nil, nil
	}
	root := filepath.Join(c.baseDir, "local-agent-mode-sessions")
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil, nil
	}

	matches, err := filepath.Glob(filepath.Join(root, "*", "*", "local_*.json"))
	if err != nil {
		return nil, fmt.Errorf("glob session metadata: %w", err)
	}

	var sessions []Session
	for _, path := range matches {
		session, ok := c.parseMetadata(path)
		if !ok {
			continue
		}
		sessions = append(sessions, session)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastUsed.After(sessions[j].LastUsed)
	})

	return sessions, nil
}

// parseMetadata reads and validates a single metadata file, warning to
// stderr and returning ok=false for malformed or incomplete records.
func (c *ClaudeCowork) parseMetadata(path string) (Session, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Session{}, false
	}

	// The desktop app writes this directory live, so a file caught mid-write
	// parses as invalid; skip it silently, as the other providers do for
	// malformed records.
	var meta claudeCoworkMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return Session{}, false
	}

	if meta.SessionID == "" {
		return Session{}, false
	}

	// Archived sessions are hidden in the app; exclude them like the OpenCode
	// provider excludes its archived sessions.
	if meta.IsArchived {
		return Session{}, false
	}

	title := meta.Title
	if title == "" {
		title = firstLine(meta.InitialMessage, 120)
	}
	if title == "" {
		title = meta.SessionID
	}

	// Directory is the attached project folder. Folderless sessions get no
	// directory: the metadata's cwd points inside the app's own sandbox, not a
	// real project path, so surfacing it would be misleading.
	directory := ""
	if len(meta.UserSelectedFolders) > 0 {
		directory = meta.UserSelectedFolders[0]
	}

	var created time.Time
	if meta.CreatedAt != 0 {
		created = time.UnixMilli(meta.CreatedAt)
	}

	var lastUsed time.Time
	switch {
	case meta.LastActivityAt != 0:
		lastUsed = time.UnixMilli(meta.LastActivityAt)
	case meta.LastFocusedAt != 0:
		lastUsed = time.UnixMilli(meta.LastFocusedAt)
	default:
		lastUsed = created
	}

	searchParts := []string{
		meta.Title,
		directory,
		meta.InitialMessage,
		meta.ScheduledTaskID,
		meta.SessionType,
		meta.Model,
	}

	return Session{
		Agent:      "claude-cowork",
		ID:         meta.SessionID,
		Title:      title,
		Created:    created,
		LastUsed:   lastUsed,
		Directory:  directory,
		SearchText: strings.Join(searchParts, " "),
	}, true
}

// firstLine returns the first non-empty line of s, truncated to max
// characters (adding an ellipsis if truncated).
func firstLine(s string, max int) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if r := []rune(line); len(r) > max {
			return string(r[:max-3]) + "..."
		}
		return line
	}
	return ""
}

func (c *ClaudeCowork) ResumeCommand(session Session) string {
	if c.resumeCommand != "" {
		return strings.ReplaceAll(c.resumeCommand, "{{ID}}", session.ID)
	}
	// Cowork sessions are owned by the desktop app — there's no CLI resume
	// path into a running Cowork session. Best effort: bring the app to the
	// foreground so the user can find the session themselves.
	switch runtime.GOOS {
	case "darwin":
		return "open -a Claude"
	case "windows":
		return fmt.Sprintf("start %s", Q("claude://"))
	default:
		return fmt.Sprintf("xdg-open %s", Q("claude://"))
	}
}

// metadataPath locates the local_<uuid>.json metadata file for a session ID
// by globbing under the two-level session tree, mirroring ListSessions.
func (c *ClaudeCowork) metadataPath(sessionID string) (string, bool) {
	if c.baseDir == "" {
		return "", false
	}
	root := filepath.Join(c.baseDir, "local-agent-mode-sessions")
	matches, err := filepath.Glob(filepath.Join(root, "*", "*", sessionID+".json"))
	if err != nil || len(matches) == 0 {
		return "", false
	}
	return matches[0], true
}

// SessionText returns the conversation text for a session. It prefers the
// transcript named after the session's cliSessionId (the authoritative one,
// since a sandbox can hold several transcripts), then any other nested Claude
// Code transcript, then audit.jsonl — all the same JSONL shape, parsed by the
// shared extractConversationText helper.
func (c *ClaudeCowork) SessionText(ctx context.Context, sessionID string) string {
	metaPath, ok := c.metadataPath(sessionID)
	if !ok {
		return ""
	}
	sandbox := strings.TrimSuffix(metaPath, ".json")
	projects := filepath.Join(sandbox, ".claude", "projects")

	// Preferred: the transcript whose filename matches cliSessionId.
	if data, err := os.ReadFile(metaPath); err == nil {
		var meta claudeCoworkMetadata
		if json.Unmarshal(data, &meta) == nil && meta.CLISessionID != "" {
			preferred, _ := filepath.Glob(filepath.Join(projects, "*", meta.CLISessionID+".jsonl"))
			for _, path := range preferred {
				if text := extractConversationText(path); text != "" {
					return text
				}
			}
		}
	}

	// Fallback: any nested transcript, then audit.jsonl.
	matches, _ := filepath.Glob(filepath.Join(projects, "*", "*.jsonl"))
	for _, path := range matches {
		if text := extractConversationText(path); text != "" {
			return text
		}
	}
	return extractConversationText(filepath.Join(sandbox, "audit.jsonl"))
}

var _ Provider = (*ClaudeCowork)(nil)
