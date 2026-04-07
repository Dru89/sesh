package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/dru89/sesh/provider"
)

// FallbackSearchFunc is called when fuzzy search returns no results.
// It receives the query and all sessions, and returns a ranked subset.
// This runs in a goroutine — it can take time (e.g. LLM call).
type FallbackSearchFunc func(ctx context.Context, query string, sessions []provider.Session) []provider.Session

// Result is returned by Pick when the user selects a session.
type Result struct {
	Session provider.Session
}

// PickOptions configures the session picker.
type PickOptions struct {
	InitialQuery   string
	FallbackSearch FallbackSearchFunc
}

// Pick launches the fzf-style TUI picker and returns the selected session.
// The TUI renders to stderr so stdout stays clean for the shell wrapper to
// capture the resume command.
func Pick(sessions []provider.Session, opts PickOptions) (*Result, error) {
	m := newModel(sessions, opts)
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr), tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	fm := final.(model)
	if fm.selected == nil {
		return nil, fmt.Errorf("cancelled")
	}
	return &Result{Session: *fm.selected}, nil
}

// --- Messages ---

// fallbackResultMsg is sent when the AI fallback search completes.
type fallbackResultMsg struct {
	sessions []provider.Session
}

// --- Model ---

type model struct {
	all            []provider.Session
	filtered       []provider.Session
	query          string
	cursor         int
	width          int
	height         int
	selected       *provider.Session
	fallbackSearch FallbackSearchFunc
	fallbackCtx    context.Context
	fallbackCancel context.CancelFunc
	searching      bool   // true while AI fallback is running
	searchMode     string // "fuzzy" or "ai"
}

func newModel(sessions []provider.Session, opts PickOptions) model {
	ctx, cancel := context.WithCancel(context.Background())
	m := model{
		all:            sessions,
		query:          opts.InitialQuery,
		fallbackSearch: opts.FallbackSearch,
		fallbackCtx:    ctx,
		fallbackCancel: cancel,
		searchMode:     "fuzzy",
	}
	m.filter()
	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case fallbackResultMsg:
		m.searching = false
		m.searchMode = "ai"
		m.filtered = msg.sessions
		if m.cursor >= len(m.filtered) {
			if len(m.filtered) > 0 {
				m.cursor = len(m.filtered) - 1
			} else {
				m.cursor = 0
			}
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.fallbackCancel()
			return m, tea.Quit

		case tea.KeyEnter:
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				s := m.filtered[m.cursor]
				m.selected = &s
			}
			m.fallbackCancel()
			return m, tea.Quit

		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}

		case tea.KeyDown:
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}

		case tea.KeyBackspace, tea.KeyDelete:
			if len(m.query) > 0 {
				m.query = m.query[:len(m.query)-1]
				return m, m.filterWithFallback()
			}

		case tea.KeyRunes:
			m.query += string(msg.Runes)
			return m, m.filterWithFallback()
		}
	}
	return m, nil
}

// filterWithFallback runs fuzzy search, and if it returns no results and a
// fallback is configured, kicks off an async AI search.
func (m *model) filterWithFallback() tea.Cmd {
	m.searchMode = "fuzzy"
	m.searching = false

	if m.query == "" {
		m.filtered = m.all
		m.clampCursor()
		return nil
	}

	source := sessionSource(m.all)
	matches := fuzzy.FindFrom(m.query, source)
	m.filtered = make([]provider.Session, len(matches))
	for i, match := range matches {
		m.filtered[i] = m.all[match.Index]
	}
	m.clampCursor()

	// If fuzzy found nothing and we have a fallback, trigger it.
	if len(m.filtered) == 0 && m.fallbackSearch != nil && len(m.query) >= 3 {
		m.searching = true
		query := m.query
		all := m.all
		fn := m.fallbackSearch
		ctx := m.fallbackCtx
		return func() tea.Msg {
			results := fn(ctx, query, all)
			return fallbackResultMsg{sessions: results}
		}
	}

	return nil
}

func (m *model) filter() {
	if m.query == "" {
		m.filtered = m.all
	} else {
		source := sessionSource(m.all)
		matches := fuzzy.FindFrom(m.query, source)
		m.filtered = make([]provider.Session, len(matches))
		for i, match := range matches {
			m.filtered[i] = m.all[match.Index]
		}
	}
	m.clampCursor()
}

func (m *model) clampCursor() {
	if m.cursor >= len(m.filtered) {
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
		} else {
			m.cursor = 0
		}
	}
}

// --- Styles ---

var (
	promptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	countStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	cursorMark  = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render("▸ ")
	selStyle    = lipgloss.NewStyle().Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	timeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	idStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	dirStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	aiStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow

	agentColors = map[string]lipgloss.Color{
		"opencode": lipgloss.Color("4"), // blue
		"claude":   lipgloss.Color("5"), // magenta
	}
	defaultAgentColor = lipgloss.Color("3") // yellow
)

func renderAgent(name string) string {
	color, ok := agentColors[name]
	if !ok {
		color = defaultAgentColor
	}
	return lipgloss.NewStyle().Foreground(color).Render(name)
}

// --- View ---

func (m model) View() string {
	var b strings.Builder
	w := m.width
	if w == 0 {
		w = 80
	}

	// Prompt line.
	b.WriteString(promptStyle.Render("> "))
	b.WriteString(m.query)
	countStr := fmt.Sprintf("  %d/%d", len(m.filtered), len(m.all))
	if m.searchMode == "ai" {
		countStr += " (AI)"
	}
	b.WriteString(countStyle.Render(countStr))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", clamp(w, 1, 120)))
	b.WriteString("\n")

	// Available height for the list.
	listH := m.height - 4
	if listH < 1 {
		listH = 20
	}

	// Window around cursor.
	start, end := visibleWindow(m.cursor, len(m.filtered), listH)

	for i := start; i < end; i++ {
		s := m.filtered[i]
		isSel := i == m.cursor

		// Cursor.
		if isSel {
			b.WriteString(cursorMark)
		} else {
			b.WriteString("  ")
		}

		// Agent badge (padded to 10 chars).
		badge := renderAgent(s.Agent)
		badgePad := 10 - len(s.Agent)
		if badgePad < 1 {
			badgePad = 1
		}

		// Title.
		title := s.DisplayTitle()
		sid := truncateID(s.ID, 10)
		maxTitleW := w - 36
		if maxTitleW < 20 {
			maxTitleW = 20
		}
		if len(title) > maxTitleW {
			title = title[:maxTitleW-1] + "…"
		}

		// Time and ID.
		when := timeStyle.Render(provider.RelativeTime(s.LastUsed))
		idStr := idStyle.Render(sid)

		if isSel {
			title = selStyle.Render(title)
		} else {
			title = dimStyle.Render(title)
		}

		b.WriteString(badge)
		b.WriteString(strings.Repeat(" ", badgePad))
		b.WriteString(title)

		// Right-align time + ID.
		suffix := when + "  " + idStr
		usedW := 2 + len(s.Agent) + badgePad + lipgloss.Width(title)
		gap := w - usedW - lipgloss.Width(suffix)
		if gap < 2 {
			gap = 2
		}
		b.WriteString(strings.Repeat(" ", gap))
		b.WriteString(suffix)
		b.WriteString("\n")

		// Show directory for the selected row.
		if isSel && s.Directory != "" {
			dir := abbreviateHome(s.Directory)
			b.WriteString("  ")
			b.WriteString(strings.Repeat(" ", 10+badgePad))
			b.WriteString(dirStyle.Render(dir))
			b.WriteString("\n")
		}
	}

	if m.searching {
		b.WriteString(aiStyle.Render("  Searching with AI...") + "\n")
	} else if len(m.filtered) == 0 {
		b.WriteString(dimStyle.Render("  No matching sessions") + "\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  ↑/↓ navigate  enter select  esc quit"))

	return b.String()
}

// --- Helpers ---

type sessionSource []provider.Session

func (s sessionSource) String(i int) string { return s[i].SearchText }
func (s sessionSource) Len() int            { return len(s) }

func visibleWindow(cursor, total, height int) (start, end int) {
	if total <= height {
		return 0, total
	}
	start = cursor - height/2
	if start < 0 {
		start = 0
	}
	end = start + height
	if end > total {
		end = total
		start = end - height
	}
	return start, end
}

func abbreviateHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func truncateID(id string, maxLen int) string {
	if len(id) <= maxLen {
		return id
	}
	return id[:maxLen] + "…"
}
