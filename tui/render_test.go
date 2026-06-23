package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/dru89/sesh/provider"
)

func TestClampLine(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		w       int
		wantMax int // max allowed display width of result
	}{
		{"narrow line untouched", "hello", 20, 5},
		{"exact width untouched", "hello", 5, 5},
		{"wide line truncated", "hello world this is long", 10, 10},
		{"nonpositive width returns input", "hello", 0, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampLine(tt.in, tt.w)
			if w := lipgloss.Width(got); w > tt.wantMax {
				t.Errorf("clampLine(%q, %d) width = %d, want <= %d (%q)", tt.in, tt.w, w, tt.wantMax, got)
			}
		})
	}
}

// TestViewNeverOverflows is the regression guard for the bug where multi-line
// titles and long directories pushed the prompt and first results off the top
// of the screen. The rendered view must never produce a line wider than the
// terminal (which would wrap) nor more lines than the terminal height (which
// would scroll). It exercises both list-only and detail modes.
func TestViewNeverOverflows(t *testing.T) {
	now := time.Now()
	sessions := make([]provider.Session, 0, 30)
	for i := 0; i < 30; i++ {
		sessions = append(sessions, provider.Session{
			Agent: "claude",
			ID:    "0123456789abcdef-session-id-very-long",
			// A raw multi-line first prompt — the exact shape that caused overflow.
			Title:     "Help me debug this\n\nIt happens when the input\nspans several lines and keeps going well past the edge of any terminal width you could imagine",
			Directory: "/Users/someone/Developer/github.example.com/some-org/some-really-deeply-nested/repository/main",
			LastUsed:  now.Add(-time.Duration(i) * time.Hour),
			Created:   now.Add(-time.Duration(i) * time.Hour),
		})
	}

	for _, dims := range []struct{ w, h int }{
		{40, 12}, {60, 20}, {80, 24}, {120, 40}, {30, 8},
	} {
		for _, detail := range []bool{false, true} {
			m := newModel(sessions, PickOptions{
				SessionText: func(agent, id string) string {
					return strings.Repeat("Lorem ipsum dolor sit amet. ", 200)
				},
			})
			m.width = dims.w
			m.height = dims.h
			m.textInput.Width = dims.w - 16
			m.showDetail = detail

			out := m.View()
			lines := strings.Split(out, "\n")
			if len(lines) > m.height {
				t.Errorf("w=%d h=%d detail=%v: rendered %d lines, exceeds height %d",
					dims.w, dims.h, detail, len(lines), m.height)
			}
			for i, line := range lines {
				if w := lipgloss.Width(line); w > dims.w {
					t.Errorf("w=%d h=%d detail=%v: line %d width %d exceeds %d: %q",
						dims.w, dims.h, detail, i, w, dims.w, line)
				}
			}
		}
	}
}
