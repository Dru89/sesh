package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ClaudeDesktop reads Claude Code sessions started from the Claude desktop
// app's Claude Code tab. The app runs the regular Claude Code engine against
// real project directories, so transcripts land in the standard
// ~/.claude/projects store — but these sessions never appear in
// ~/.claude/history.jsonl (that file only records prompts typed in the
// terminal), which is all the Claude provider reads. Session metadata,
// including the app-generated title, lives under the desktop app's Electron
// userData directory in claude-code-sessions/, a sibling of the
// local-agent-mode-sessions/ tree the ClaudeCowork provider reads.
type ClaudeDesktop struct {
	baseDir       string // Electron userData dir holding claude-code-sessions/
	claudeDir     string // ~/.claude, the shared transcript store
	resumeCommand string // override for resume command template
}

// ClaudeDesktopOption configures the ClaudeDesktop provider.
type ClaudeDesktopOption func(*ClaudeDesktop)

// WithClaudeDesktopResumeCommand overrides the default resume command template.
// Use {{ID}} as a placeholder for the session ID.
func WithClaudeDesktopResumeCommand(cmd string) ClaudeDesktopOption {
	return func(c *ClaudeDesktop) {
		c.resumeCommand = cmd
	}
}

// NewClaudeDesktop creates a ClaudeDesktop provider rooted at the platform's
// Electron userData directory for the Claude desktop app (the same base the
// ClaudeCowork provider uses):
//   - macOS:   ~/Library/Application Support/Claude
//   - Windows: %AppData%\Claude
//   - Linux:   ~/.config/Claude
func NewClaudeDesktop(opts ...ClaudeDesktopOption) *ClaudeDesktop {
	c := &ClaudeDesktop{}
	if cfgDir, err := os.UserConfigDir(); err == nil {
		c.baseDir = filepath.Join(cfgDir, "Claude")
	}
	if home, err := os.UserHomeDir(); err == nil {
		c.claudeDir = filepath.Join(home, ".claude")
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *ClaudeDesktop) Name() string { return "claude-desktop" }

// claudeDesktopMetadata is the shape of a local_<uuid>.json session metadata file
// under claude-code-sessions/. Same family as the Cowork metadata but with real
// project paths (cwd/originCwd) instead of userSelectedFolders.
type claudeDesktopMetadata struct {
	SessionID       string `json:"sessionId"`
	CLISessionID    string `json:"cliSessionId"`
	Title           string `json:"title"`
	Cwd             string `json:"cwd"`
	OriginCwd       string `json:"originCwd"`
	CreatedAt       int64  `json:"createdAt"`
	LastActivityAt  int64  `json:"lastActivityAt"`
	LastFocusedAt   int64  `json:"lastFocusedAt"`
	ScheduledTaskID string `json:"scheduledTaskId"`
	Model           string `json:"model"`
	IsArchived      bool   `json:"isArchived"`
}

// ListSessions walks <baseDir>/claude-code-sessions/<uuid>/<uuid>/local_<uuid>.json
// metadata files and builds a Session for each.
func (c *ClaudeDesktop) ListSessions(ctx context.Context) ([]Session, error) {
	if c.baseDir == "" {
		return nil, nil
	}
	root := filepath.Join(c.baseDir, "claude-code-sessions")
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

// parseMetadata reads and validates a single metadata file, returning ok=false
// for malformed or incomplete records.
func (c *ClaudeDesktop) parseMetadata(path string) (Session, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Session{}, false
	}

	// The desktop app writes this directory live, so a file caught mid-write
	// parses as invalid; skip it silently, as the other providers do for
	// malformed records.
	var meta claudeDesktopMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return Session{}, false
	}

	if meta.SessionID == "" {
		return Session{}, false
	}

	// Archived sessions are hidden in the app; exclude them like the Cowork
	// and OpenCode providers exclude theirs.
	if meta.IsArchived {
		return Session{}, false
	}

	// The cliSessionId is the underlying Claude Code session UUID — the one
	// that names the transcript in ~/.claude/projects and that
	// `claude --resume` accepts. Prefer it so resume works from a terminal.
	id := meta.CLISessionID
	if id == "" {
		id = meta.SessionID
	}

	title := meta.Title
	if title == "" {
		title = id
	}

	// Unlike Cowork sessions, cwd here is the real project directory the
	// user attached, not an app sandbox path.
	directory := meta.Cwd
	if directory == "" {
		directory = meta.OriginCwd
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
		meta.ScheduledTaskID,
		meta.Model,
	}

	return Session{
		Agent:        "claude-desktop",
		ID:           id,
		Title:        title,
		Created:      created,
		LastUsed:     lastUsed,
		Directory:    directory,
		SearchText:   strings.Join(searchParts, " "),
		CuratedTitle: meta.Title != "",
	}, true
}

// ResumeCommand resumes the session in a terminal. Desktop sessions are
// ordinary Claude Code sessions in the shared ~/.claude/projects store, so
// `claude --resume` picks them up like any CLI session.
func (c *ClaudeDesktop) ResumeCommand(session Session) string {
	var cmd string
	if c.resumeCommand != "" {
		cmd = strings.ReplaceAll(c.resumeCommand, "{{ID}}", session.ID)
	} else {
		cmd = fmt.Sprintf("claude --resume %s", Q(session.ID))
	}
	return CdAndRun(session.Directory, cmd)
}

// SessionText returns the conversation text for a session from the shared
// ~/.claude/projects transcript store, keyed by the cliSessionId that
// ListSessions exposes as the session ID.
func (c *ClaudeDesktop) SessionText(ctx context.Context, sessionID string) string {
	return transcriptTextFromProjects(c.claudeDir, sessionID)
}

var _ Provider = (*ClaudeDesktop)(nil)
