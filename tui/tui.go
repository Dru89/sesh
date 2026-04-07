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

// SessionTextFunc returns the raw session text for a given agent and session ID.
// Used for the detail preview pane.
type SessionTextFunc func(agent, sessionID string) string

// PickOptions configures the session picker.
type PickOptions struct {
	InitialQuery   string
	FallbackSearch FallbackSearchFunc
	SessionText    SessionTextFunc
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
	showDetail     bool   // true when detail pane is visible
	sessionText    SessionTextFunc
	detailCache    map[string]string // agent:id -> text
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
		sessionText:    opts.SessionText,
		detailCache:    make(map[string]string),
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

		case tea.KeyTab:
			m.showDetail = !m.showDetail

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
	w := m.width
	if w == 0 {
		w = 80
	}

	if m.showDetail && len(m.filtered) > 0 && m.cursor < len(m.filtered) {
		return m.viewWithDetail(w)
	}
	return m.viewList(w, w)
}

func (m model) viewWithDetail(totalW int) string {
	// Split: ~40% list, ~60% detail.
	listW := totalW * 2 / 5
	if listW < 30 {
		listW = 30
	}
	detailW := totalW - listW - 3 // 3 for the separator column

	listView := m.viewList(listW, totalW)
	detailView := m.viewDetail(detailW)

	// Place them side by side using a vertical separator.
	listLines := strings.Split(listView, "\n")
	detailLines := strings.Split(detailView, "\n")

	// Pad to same height.
	maxLines := len(listLines)
	if len(detailLines) > maxLines {
		maxLines = len(detailLines)
	}
	for len(listLines) < maxLines {
		listLines = append(listLines, "")
	}
	for len(detailLines) < maxLines {
		detailLines = append(detailLines, "")
	}

	var b strings.Builder
	sep := dimStyle.Render(" │ ")
	for i := 0; i < maxLines; i++ {
		left := listLines[i]
		// Pad left column to listW.
		leftW := lipgloss.Width(left)
		if leftW < listW {
			left += strings.Repeat(" ", listW-leftW)
		}
		b.WriteString(left)
		b.WriteString(sep)
		b.WriteString(detailLines[i])
		b.WriteString("\n")
	}

	return b.String()
}

func (m model) viewDetail(w int) string {
	s := m.filtered[m.cursor]

	var b strings.Builder
	labelStyle := lipgloss.NewStyle().Bold(true)

	b.WriteString(labelStyle.Render("Agent:      "))
	b.WriteString(renderAgent(s.Agent))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Session ID: "))
	b.WriteString(s.ID)
	b.WriteString("\n")

	if s.Slug != "" {
		b.WriteString(labelStyle.Render("Slug:       "))
		b.WriteString(s.Slug)
		b.WriteString("\n")
	}

	b.WriteString(labelStyle.Render("Title:      "))
	title := s.Title
	if len(title) > w-12 && w > 15 {
		title = title[:w-15] + "…"
	}
	b.WriteString(title)
	b.WriteString("\n")

	if s.Summary != "" && s.Summary != s.Title {
		b.WriteString(labelStyle.Render("Summary:    "))
		b.WriteString(s.Summary)
		b.WriteString("\n")
	}

	if s.Directory != "" {
		b.WriteString(labelStyle.Render("Directory:  "))
		b.WriteString(abbreviateHome(s.Directory))
		b.WriteString("\n")
	}

	b.WriteString(labelStyle.Render("Created:    "))
	b.WriteString(s.Created.Format("Jan 2, 2006 3:04pm"))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Last Used:  "))
	b.WriteString(s.LastUsed.Format("Jan 2, 2006 3:04pm"))
	b.WriteString(" (" + provider.RelativeTime(s.LastUsed) + ")")
	b.WriteString("\n")

	// Session text preview.
	if m.sessionText != nil {
		key := s.Agent + ":" + s.ID
		text, ok := m.detailCache[key]
		if !ok {
			text = m.sessionText(s.Agent, s.ID)
			// Can't mutate m.detailCache in View (it's a value receiver),
			// but we'll just re-fetch each render. It's fast for local reads.
		}
		if text != "" {
			b.WriteString("\n")
			b.WriteString(labelStyle.Render("First messages:"))
			b.WriteString("\n")
			// Wrap text to detail width and cap at available height.
			maxChars := w * (m.height - 14)
			if maxChars < 200 {
				maxChars = 200
			}
			if len(text) > maxChars {
				text = text[:maxChars] + "…"
			}
			// Simple wrapping.
			for _, line := range strings.Split(text, "\n") {
				for len(line) > w {
					b.WriteString(dimStyle.Render(line[:w]))
					b.WriteString("\n")
					line = line[w:]
				}
				b.WriteString(dimStyle.Render(line))
				b.WriteString("\n")
			}
		}
	}

	return b.String()
}

func (m model) viewList(w int, fullW int) string {
	var b strings.Builder

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
		if m.showDetail {
			// In split view, skip the ID to save space.
			maxTitleW = w - 22
		}
		if maxTitleW < 20 {
			maxTitleW = 20
		}
		if len(title) > maxTitleW {
			title = title[:maxTitleW-1] + "…"
		}

		// Time and ID.
		when := timeStyle.Render(provider.RelativeTime(s.LastUsed))

		if isSel {
			title = selStyle.Render(title)
		} else {
			title = dimStyle.Render(title)
		}

		b.WriteString(badge)
		b.WriteString(strings.Repeat(" ", badgePad))
		b.WriteString(title)

		if m.showDetail {
			// Compact: just time, no ID.
			usedW := 2 + len(s.Agent) + badgePad + lipgloss.Width(title)
			gap := w - usedW - lipgloss.Width(when)
			if gap < 2 {
				gap = 2
			}
			b.WriteString(strings.Repeat(" ", gap))
			b.WriteString(when)
		} else {
			// Full: time + ID.
			idStr := idStyle.Render(sid)
			suffix := when + "  " + idStr
			usedW := 2 + len(s.Agent) + badgePad + lipgloss.Width(title)
			gap := w - usedW - lipgloss.Width(suffix)
			if gap < 2 {
				gap = 2
			}
			b.WriteString(strings.Repeat(" ", gap))
			b.WriteString(suffix)
		}
		b.WriteString("\n")

		// Show directory for the selected row (only in list-only mode).
		if !m.showDetail && isSel && s.Directory != "" {
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
	helpText := "  ↑/↓ navigate  enter select  tab detail  esc quit"
	b.WriteString(helpStyle.Render(helpText))

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
