package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dru89/sesh/provider"
)

func TestResolveCommand(t *testing.T) {
	haiku := []string{"llm", "-m", "haiku"}
	sonnet := []string{"llm", "-m", "sonnet"}

	tests := []struct {
		name       string
		candidates [][]string
		want       []string
	}{
		{"first set", [][]string{haiku, sonnet}, haiku},
		{"skip empty", [][]string{nil, sonnet}, sonnet},
		{"all empty", [][]string{nil, nil}, nil},
		{"single", [][]string{haiku}, haiku},
		{"empty slice", [][]string{[]string{}, sonnet}, sonnet},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveCommand(tt.candidates...)
			if len(got) != len(tt.want) {
				t.Errorf("resolveCommand() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("resolveCommand()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFallbackChains(t *testing.T) {
	haiku := []string{"llm", "-m", "haiku"}
	sonnet := []string{"llm", "-m", "sonnet"}
	fast := []string{"llm", "-m", "flash"}

	t.Run("single command configures everything", func(t *testing.T) {
		cfg := config{
			Index: commandConfig{Command: haiku},
		}
		assertCmd(t, "indexCommand", cfg.indexCommand(), haiku)
		assertCmd(t, "askCommand", cfg.askCommand(), haiku)
		assertCmd(t, "askFilterCommand", cfg.askFilterCommand(), haiku)
		assertCmd(t, "recapCommand", cfg.recapCommand(), haiku)
	})

	t.Run("split fast and heavy", func(t *testing.T) {
		cfg := config{
			Index: commandConfig{Command: haiku},
			Ask:   askConfig{Command: sonnet},
			Recap: commandConfig{Command: sonnet},
		}
		assertCmd(t, "indexCommand", cfg.indexCommand(), haiku)
		assertCmd(t, "askCommand", cfg.askCommand(), sonnet)
		assertCmd(t, "askFilterCommand", cfg.askFilterCommand(), haiku) // falls back to index
		assertCmd(t, "recapCommand", cfg.recapCommand(), sonnet)
	})

	t.Run("full config", func(t *testing.T) {
		cfg := config{
			Index: commandConfig{Command: haiku},
			Ask:   askConfig{Command: sonnet, FilterCommand: fast},
			Recap: commandConfig{Command: sonnet},
		}
		assertCmd(t, "indexCommand", cfg.indexCommand(), haiku)
		assertCmd(t, "askCommand", cfg.askCommand(), sonnet)
		assertCmd(t, "askFilterCommand", cfg.askFilterCommand(), fast)
		assertCmd(t, "recapCommand", cfg.recapCommand(), sonnet)
	})

	t.Run("only recap configured", func(t *testing.T) {
		cfg := config{
			Recap: commandConfig{Command: sonnet},
		}
		assertCmd(t, "indexCommand", cfg.indexCommand(), sonnet) // index -> recap
		assertCmd(t, "askCommand", cfg.askCommand(), sonnet)     // ask -> recap
		assertCmd(t, "recapCommand", cfg.recapCommand(), sonnet)
	})

	t.Run("only ask configured", func(t *testing.T) {
		cfg := config{
			Ask: askConfig{Command: sonnet},
		}
		assertCmd(t, "indexCommand", cfg.indexCommand(), sonnet) // index -> recap(nil) -> ask
		assertCmd(t, "askCommand", cfg.askCommand(), sonnet)
		assertCmd(t, "askFilterCommand", cfg.askFilterCommand(), sonnet) // filter -> index(nil) -> ask
		assertCmd(t, "recapCommand", cfg.recapCommand(), sonnet)         // recap -> ask
	})

	t.Run("nothing configured", func(t *testing.T) {
		cfg := config{}
		if cmd := cfg.indexCommand(); cmd != nil {
			t.Errorf("expected nil, got %v", cmd)
		}
		if cfg.hasAnyCommand() {
			t.Error("hasAnyCommand should be false")
		}
	})
}

func TestResumeCommandStr(t *testing.T) {
	t.Run("string form", func(t *testing.T) {
		pc := providerConfig{
			ResumeCommand: json.RawMessage(`"opencode --session {{ID}}"`),
		}
		got := pc.resumeCommandStr()
		if got != "opencode --session {{ID}}" {
			t.Errorf("got %q, want %q", got, "opencode --session {{ID}}")
		}
	})

	t.Run("array form", func(t *testing.T) {
		pc := providerConfig{
			ResumeCommand: json.RawMessage(`["ca", "opencode", "-s", "{{ID}}"]`),
		}
		got := pc.resumeCommandStr()
		// "ca" and "opencode" and "-s" are shell-safe, {{ID}} is a template marker
		want := "ca opencode -s {{ID}}"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("array with spaces in element", func(t *testing.T) {
		pc := providerConfig{
			ResumeCommand: json.RawMessage(`["/usr/local/my tools/agent", "--session", "{{ID}}"]`),
		}
		got := pc.resumeCommandStr()
		want := "'/usr/local/my tools/agent' --session {{ID}}"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("empty", func(t *testing.T) {
		pc := providerConfig{}
		got := pc.resumeCommandStr()
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("null json", func(t *testing.T) {
		pc := providerConfig{
			ResumeCommand: json.RawMessage(`null`),
		}
		got := pc.resumeCommandStr()
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestParseDateish(t *testing.T) {
	// Use a fixed "now" — Tuesday April 7, 2026 12:00 local time.
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.Local)

	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{"iso date", "2026-04-01", time.Date(2026, 4, 1, 0, 0, 0, 0, time.Local)},
		{"relative days", "3d", now.AddDate(0, 0, -3)},
		{"today", "today", time.Date(2026, 4, 7, 0, 0, 0, 0, time.Local)},
		{"yesterday", "yesterday", time.Date(2026, 4, 6, 0, 0, 0, 0, time.Local)},
		{"last week", "last week", now.AddDate(0, 0, -7)},
		// Tuesday -> Monday = 1 day back
		{"monday", "monday", time.Date(2026, 4, 6, 0, 0, 0, 0, time.Local)},
		// Tuesday -> Friday = 4 days back
		{"friday", "friday", time.Date(2026, 4, 3, 0, 0, 0, 0, time.Local)},
		// Tuesday -> Wednesday = 6 days back
		{"wednesday", "wednesday", time.Date(2026, 4, 1, 0, 0, 0, 0, time.Local)},
		// Tuesday -> Tuesday = 7 days back (not 0)
		{"tuesday same day", "tuesday", time.Date(2026, 3, 31, 0, 0, 0, 0, time.Local)},
		{"case insensitive", "Monday", time.Date(2026, 4, 6, 0, 0, 0, 0, time.Local)},
		// Fallback: unknown string returns 7 days ago.
		{"unknown", "gibberish", now.AddDate(0, 0, -7)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDateish(tt.input, now)
			if !got.Equal(tt.want) {
				t.Errorf("parseDateish(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseTimeRange(t *testing.T) {
	t.Run("days flag", func(t *testing.T) {
		start, end := parseTimeRange("", "", 7)
		if time.Since(start) > 8*24*time.Hour || time.Since(start) < 6*24*time.Hour {
			t.Errorf("start should be ~7 days ago, got %v", start)
		}
		if time.Since(end) > time.Minute {
			t.Errorf("end should be ~now, got %v", end)
		}
	})

	t.Run("default is 7 days", func(t *testing.T) {
		start, _ := parseTimeRange("", "", 0)
		if time.Since(start) > 8*24*time.Hour || time.Since(start) < 6*24*time.Hour {
			t.Errorf("default start should be ~7 days ago, got %v", start)
		}
	})

	t.Run("until date", func(t *testing.T) {
		_, end := parseTimeRange("", "2026-04-07", 7)
		// end should be end of 2026-04-07
		if end.Year() != 2026 || end.Month() != 4 || end.Day() != 7 {
			t.Errorf("end should be 2026-04-07, got %v", end)
		}
	})
}

func TestConfigHasAnyCommand(t *testing.T) {
	t.Run("index set", func(t *testing.T) {
		cfg := config{Index: commandConfig{Command: []string{"llm"}}}
		if !cfg.hasAnyCommand() {
			t.Error("expected true")
		}
	})

	t.Run("ask set", func(t *testing.T) {
		cfg := config{Ask: askConfig{Command: []string{"llm"}}}
		if !cfg.hasAnyCommand() {
			t.Error("expected true")
		}
	})

	t.Run("nothing set", func(t *testing.T) {
		cfg := config{}
		if cfg.hasAnyCommand() {
			t.Error("expected false")
		}
	})
}

func TestSummaryConfig(t *testing.T) {
	cfg := config{
		Index: commandConfig{
			Command: []string{"llm", "-m", "haiku"},
			Prompt:  "custom prompt",
		},
	}
	sc := cfg.summaryConfig()
	if len(sc.Command) != 3 || sc.Command[2] != "haiku" {
		t.Errorf("summaryConfig command = %v, want haiku", sc.Command)
	}
	if sc.Prompt != "custom prompt" {
		t.Errorf("summaryConfig prompt = %q, want %q", sc.Prompt, "custom prompt")
	}
}

// assertCmd checks that two string slices match.
func assertCmd(t *testing.T, name string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s = %v, want %v", name, got, want)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("%s[%d] = %q, want %q", name, i, got[i], want[i])
		}
	}
}

// --- findSession tests ---

func testSessions() []provider.Session {
	now := time.Now()
	return []provider.Session{
		{Agent: "opencode", ID: "ses_abc123", Title: "Auth middleware", LastUsed: now},
		{Agent: "opencode", ID: "ses_abc456", Title: "Fix tests", LastUsed: now.Add(-time.Hour)},
		{Agent: "claude", ID: "uuid-1234-5678", Title: "Refactor API", LastUsed: now.Add(-2 * time.Hour)},
		{Agent: "opencode", ID: "ses_def789", Title: "Build pipeline", LastUsed: now.Add(-24 * time.Hour)},
	}
}

func TestFindSessionExactMatch(t *testing.T) {
	sessions := testSessions()
	match, ambiguous := findSession(sessions, "ses_abc123")
	if match == nil {
		t.Fatal("expected match")
	}
	if match.ID != "ses_abc123" {
		t.Errorf("got %q, want ses_abc123", match.ID)
	}
	if ambiguous != nil {
		t.Error("expected no ambiguous candidates")
	}
}

func TestFindSessionPrefixUnique(t *testing.T) {
	sessions := testSessions()
	match, ambiguous := findSession(sessions, "ses_def")
	if match == nil {
		t.Fatal("expected match")
	}
	if match.ID != "ses_def789" {
		t.Errorf("got %q, want ses_def789", match.ID)
	}
	if ambiguous != nil {
		t.Error("expected no ambiguous candidates")
	}
}

func TestFindSessionPrefixAmbiguous(t *testing.T) {
	sessions := testSessions()
	match, ambiguous := findSession(sessions, "ses_abc")
	if match != nil {
		t.Error("expected nil match for ambiguous prefix")
	}
	if len(ambiguous) != 2 {
		t.Errorf("expected 2 ambiguous candidates, got %d", len(ambiguous))
	}
}

func TestFindSessionNotFound(t *testing.T) {
	sessions := testSessions()
	match, ambiguous := findSession(sessions, "nonexistent")
	if match != nil {
		t.Error("expected nil match")
	}
	if ambiguous != nil {
		t.Error("expected nil ambiguous")
	}
}

func TestFindSessionExactOverPrefix(t *testing.T) {
	// If an ID is an exact match, it should win even if it's also a prefix of another.
	sessions := []provider.Session{
		{ID: "ses_abc"},
		{ID: "ses_abc123"},
	}
	match, _ := findSession(sessions, "ses_abc")
	if match == nil {
		t.Fatal("expected match")
	}
	if match.ID != "ses_abc" {
		t.Errorf("got %q, want ses_abc (exact should win)", match.ID)
	}
}

// --- computeStats tests ---

func TestComputeStats(t *testing.T) {
	now := time.Now()
	sessions := []provider.Session{
		{Agent: "opencode", Summary: "summary", Directory: "/project-a", LastUsed: now},
		{Agent: "opencode", Summary: "", Directory: "/project-a", LastUsed: now.Add(-2 * time.Hour)},
		{Agent: "claude", Summary: "summary", Directory: "/project-b", LastUsed: now.Add(-3 * 24 * time.Hour)},
		{Agent: "claude", Summary: "summary", Directory: "/project-b", LastUsed: now.Add(-40 * 24 * time.Hour)},
	}

	stats := computeStats(sessions)

	if stats.Total != 4 {
		t.Errorf("Total = %d, want 4", stats.Total)
	}
	if stats.Summarized != 3 {
		t.Errorf("Summarized = %d, want 3", stats.Summarized)
	}
	if stats.AgentCounts["opencode"] != 2 {
		t.Errorf("opencode count = %d, want 2", stats.AgentCounts["opencode"])
	}
	if stats.AgentCounts["claude"] != 2 {
		t.Errorf("claude count = %d, want 2", stats.AgentCounts["claude"])
	}
	if stats.Today != 2 {
		t.Errorf("Today = %d, want 2", stats.Today)
	}
	if stats.ThisWeek != 3 {
		t.Errorf("ThisWeek = %d, want 3", stats.ThisWeek)
	}
	if stats.ThisMonth != 3 {
		t.Errorf("ThisMonth = %d, want 3", stats.ThisMonth)
	}
	if stats.DirCounts["/project-a"] != 2 {
		t.Errorf("project-a count = %d, want 2", stats.DirCounts["/project-a"])
	}
	if stats.DirCounts["/project-b"] != 2 {
		t.Errorf("project-b count = %d, want 2", stats.DirCounts["/project-b"])
	}
}

func TestComputeStatsEmpty(t *testing.T) {
	stats := computeStats(nil)
	if stats.Total != 0 {
		t.Errorf("Total = %d, want 0", stats.Total)
	}
}

// --- truncate tests ---

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{"short", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"truncated", "hello world", 8, "hello w…"},
		{"one char", "ab", 1, "…"},
		{"empty", "", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}
}

// --- detectShell tests ---

func TestDetectShell(t *testing.T) {
	tests := []struct {
		shell string
		want  string
	}{
		{"/bin/bash", "bash"},
		{"/bin/zsh", "zsh"},
		{"/usr/bin/fish", "fish"},
		{"/bin/sh", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			orig := os.Getenv("SHELL")
			os.Setenv("SHELL", tt.shell)
			defer os.Setenv("SHELL", orig)

			got := detectShell()
			if got != tt.want {
				t.Errorf("detectShell() with SHELL=%q = %q, want %q", tt.shell, got, tt.want)
			}
		})
	}
}

// --- init output tests ---

func TestInitOutputs(t *testing.T) {
	// Verify the init strings contain the essential function definition.
	if !strings.Contains(initBash, "sesh()") {
		t.Error("initBash missing function definition")
	}
	if !strings.Contains(initBash, "eval") {
		t.Error("initBash missing eval")
	}
	if !strings.Contains(initBash, "command sesh") {
		t.Error("initBash missing 'command sesh'")
	}

	if !strings.Contains(initZsh, "sesh()") {
		t.Error("initZsh missing function definition")
	}

	if !strings.Contains(initFish, "function sesh") {
		t.Error("initFish missing function definition")
	}
	if !strings.Contains(initFish, "eval") {
		t.Error("initFish missing eval")
	}

	if !strings.Contains(initPowerShell, "function sesh") {
		t.Error("initPowerShell missing function definition")
	}
	if !strings.Contains(initPowerShell, "Invoke-Expression") {
		t.Error("initPowerShell missing Invoke-Expression")
	}
	if !strings.Contains(initPowerShell, "sesh.exe") {
		t.Error("initPowerShell missing sesh.exe")
	}
}

// --- colorAgent tests ---

func TestColorAgent(t *testing.T) {
	// Just verify it returns non-empty strings and doesn't panic.
	for _, agent := range []string{"opencode", "claude", "omp", "unknown"} {
		got := colorAgent(agent)
		if got == "" {
			t.Errorf("colorAgent(%q) returned empty", agent)
		}
		if !strings.Contains(got, agent) {
			t.Errorf("colorAgent(%q) = %q, should contain agent name", agent, got)
		}
	}
}

// --- aiFilterSessions tests ---

func TestAiFilterSessions(t *testing.T) {
	sessions := testSessions()

	// Mock LLM that returns "1\n3\n" (first and third sessions).
	script := writeMockScript(t, "#!/bin/sh\necho '1\n3'")

	result := aiFilterSessions(context.Background(), []string{script}, "auth", sessions)
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	if result[0].ID != "ses_abc123" {
		t.Errorf("first result should be ses_abc123, got %s", result[0].ID)
	}
	if result[1].ID != "uuid-1234-5678" {
		t.Errorf("second result should be uuid-1234-5678, got %s", result[1].ID)
	}
}

func TestAiFilterSessionsFormats(t *testing.T) {
	sessions := testSessions()

	// LLM returns numbers in various formats.
	script := writeMockScript(t, "#!/bin/sh\necho '2.\n4 - build pipeline'")

	result := aiFilterSessions(context.Background(), []string{script}, "tests", sessions)
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	if result[0].ID != "ses_abc456" {
		t.Errorf("first result should be ses_abc456, got %s", result[0].ID)
	}
	if result[1].ID != "ses_def789" {
		t.Errorf("second result should be ses_def789, got %s", result[1].ID)
	}
}

func TestAiFilterSessionsNoResults(t *testing.T) {
	sessions := testSessions()

	// LLM returns empty output.
	script := writeMockScript(t, "#!/bin/sh\necho ''")

	result := aiFilterSessions(context.Background(), []string{script}, "nothing", sessions)
	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

func TestAiFilterSessionsCommandFailure(t *testing.T) {
	sessions := testSessions()

	script := writeMockScript(t, "#!/bin/sh\nexit 1")

	result := aiFilterSessions(context.Background(), []string{script}, "query", sessions)
	if result != nil {
		t.Errorf("expected nil on failure, got %d results", len(result))
	}
}

func TestAiFilterSessionsMaxResults(t *testing.T) {
	// Build 20 sessions.
	var sessions []provider.Session
	for i := 0; i < 20; i++ {
		sessions = append(sessions, provider.Session{
			ID:    fmt.Sprintf("ses_%d", i),
			Title: fmt.Sprintf("Session %d", i),
		})
	}

	// LLM returns all 20.
	script := writeMockScript(t, "#!/bin/sh\nseq 1 20")

	result := aiFilterSessions(context.Background(), []string{script}, "all", sessions)
	if len(result) != 10 {
		t.Errorf("expected max 10 results, got %d", len(result))
	}
}

func writeMockScript(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "mock.sh")
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}
