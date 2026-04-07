package summary

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const defaultPrompt = "Summarize what was worked on in this coding session. One sentence, under 20 words. Output only the summary, nothing else."

// Config holds the summary generation settings from the user's config file.
type Config struct {
	// Command is the executable (plus args) that generates a summary.
	// Session text is passed on stdin. Summary is read from stdout.
	// If empty, summary generation is disabled.
	Command []string `json:"command"`

	// Prompt is prepended to the session text sent to the command.
	// If empty, a default prompt is used.
	Prompt string `json:"prompt,omitempty"`
}

// IsEnabled returns true if summary generation is configured.
func (c Config) IsEnabled() bool {
	return len(c.Command) > 0
}

// Generator produces summaries by shelling out to a user-configured command.
type Generator struct {
	config Config
}

// NewGenerator creates a summary generator from config.
func NewGenerator(cfg Config) *Generator {
	return &Generator{config: cfg}
}

// Generate produces a summary for the given session text.
// The text should be a concatenation of user prompts from the session.
// Returns the summary string or an error. Errors are non-fatal — callers
// should log and continue.
func (g *Generator) Generate(ctx context.Context, sessionText string) (string, error) {
	if !g.config.IsEnabled() {
		return "", fmt.Errorf("summary generation not configured")
	}

	prompt := g.config.Prompt
	if prompt == "" {
		prompt = defaultPrompt
	}

	input := prompt + "\n\n" + sessionText

	// Apply a per-summary timeout so one slow call doesn't block everything.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, g.config.Command[0], g.config.Command[1:]...)
	cmd.Stdin = strings.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return "", fmt.Errorf("summary command failed: %w: %s", err, errMsg)
		}
		return "", fmt.Errorf("summary command failed: %w", err)
	}

	summary := strings.TrimSpace(stdout.String())
	if summary == "" {
		return "", fmt.Errorf("summary command returned empty output")
	}

	// Truncate very long summaries (the command should respect the prompt,
	// but we cap it defensively).
	if len(summary) > 200 {
		summary = summary[:197] + "..."
	}

	return summary, nil
}

// GenerateBatch generates summaries for multiple sessions, calling the
// progress callback after each one. Returns the number of successful
// summaries generated.
func (g *Generator) GenerateBatch(ctx context.Context, items []BatchItem, cache *Cache, progress func(i, total int, id string, err error)) int {
	succeeded := 0
	for i, item := range items {
		if ctx.Err() != nil {
			break
		}
		summary, err := g.Generate(ctx, item.Text)
		if err == nil {
			cache.Put(item.ID, summary, item.LastUsed)
			succeeded++
		}
		if progress != nil {
			progress(i+1, len(items), item.ID, err)
		}
	}
	return succeeded
}

// BatchItem is a session to be summarized.
type BatchItem struct {
	ID       string
	LastUsed time.Time
	Text     string // concatenated user prompts
}
