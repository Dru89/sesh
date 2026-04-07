package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/dru89/sesh/provider"
	"github.com/dru89/sesh/summary"
	"github.com/dru89/sesh/tui"
)

// config is the user configuration loaded from ~/.config/sesh/config.json.
type config struct {
	Providers map[string]providerConfig `json:"providers"`
	Summary   summary.Config            `json:"summary"`
}

type providerConfig struct {
	// ResumeCommand overrides the default resume command for a built-in provider.
	// Use {{ID}} as a placeholder for the session ID.
	ResumeCommand string `json:"resume_command,omitempty"`

	// Enabled controls whether this provider is active (default: true).
	// Set to false to disable a built-in provider.
	Enabled *bool `json:"enabled,omitempty"`

	// ListCommand is the command to run to list sessions (external providers only).
	ListCommand []string `json:"list_command,omitempty"`
}

func (pc providerConfig) isEnabled() bool {
	if pc.Enabled == nil {
		return true
	}
	return *pc.Enabled
}

// jsonSession extends provider.Session with the resume command for JSON output.
type jsonSession struct {
	provider.Session
	ResumeCommand string `json:"resume_command"`
}

func main() {
	// Check for subcommands before flag parsing.
	if len(os.Args) > 1 && os.Args[1] == "index" {
		runIndex(os.Args[2:])
		return
	}

	jsonMode := flag.Bool("json", false, "Output session list as JSON and exit")
	agentFilter := flag.String("agent", "", "Filter by agent name")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: sesh [options] [query]\n\n")
		fmt.Fprintf(os.Stderr, "A unified session browser for coding agents.\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  index    Generate summaries for all sessions\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nConfig: ~/.config/sesh/config.json\n")
		fmt.Fprintf(os.Stderr, "\nShell wrapper (add to your shell rc):\n")
		fmt.Fprintf(os.Stderr, "  sesh() { local cmd; cmd=$(command sesh \"$@\") || return $?; eval \"$cmd\"; }\n")
	}
	flag.Parse()
	query := strings.Join(flag.Args(), " ")

	cfg := loadConfig()
	providers := buildProviders(cfg)
	cache := summary.NewCache()

	// Collect sessions from all providers.
	ctx := context.Background()
	all := collectSessions(ctx, providers, *agentFilter)

	// Apply cached summaries to sessions.
	applySummaries(all, cache)

	// Sort by last used, newest first.
	sort.Slice(all, func(i, j int) bool {
		return all[i].LastUsed.After(all[j].LastUsed)
	})

	// JSON mode: dump and exit.
	if *jsonMode {
		providerMap := providersByName(providers)
		var out []jsonSession
		for _, s := range all {
			var cmd string
			if p, ok := providerMap[s.Agent]; ok {
				cmd = p.ResumeCommand(s)
			}
			out = append(out, jsonSession{Session: s, ResumeCommand: cmd})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(os.Stderr, "sesh: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(all) == 0 {
		fmt.Fprintf(os.Stderr, "sesh: no sessions found\n")
		os.Exit(1)
	}

	// Kick off lazy background summary generation for unsummarized sessions.
	if cfg.Summary.IsEnabled() {
		go lazyIndex(ctx, cfg.Summary, cache, all, providers)
	}

	// Run the TUI picker.
	result, err := tui.Pick(all, query)
	if err != nil {
		// Save any summaries generated in the background before exiting.
		cache.Save()
		os.Exit(130)
	}

	// Save any summaries generated in the background.
	cache.Save()

	// Find the provider and output the resume command.
	providerMap := providersByName(providers)
	if p, ok := providerMap[result.Session.Agent]; ok {
		fmt.Println(p.ResumeCommand(result.Session))
	} else {
		fmt.Fprintf(os.Stderr, "sesh: unknown provider %q\n", result.Session.Agent)
		os.Exit(1)
	}
}

// runIndex handles the `sesh index` subcommand.
func runIndex(args []string) {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	agentFilter := fs.String("agent", "", "Only index sessions for a specific agent")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: sesh index [options]\n\n")
		fmt.Fprintf(os.Stderr, "Generate summaries for all sessions that don't have one.\n")
		fmt.Fprintf(os.Stderr, "Requires summary.command to be configured.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}
	fs.Parse(args)

	cfg := loadConfig()
	if !cfg.Summary.IsEnabled() {
		fmt.Fprintf(os.Stderr, "sesh: summary generation not configured\n")
		fmt.Fprintf(os.Stderr, "sesh: add a \"summary\" section to ~/.config/sesh/config.json:\n")
		fmt.Fprintf(os.Stderr, "  {\n    \"summary\": {\n      \"command\": [\"llm\", \"-m\", \"haiku\"]\n    }\n  }\n")
		os.Exit(1)
	}

	providers := buildProviders(cfg)
	cache := summary.NewCache()
	ctx := context.Background()

	all := collectSessions(ctx, providers, *agentFilter)
	providerMap := providersByName(providers)

	// Find sessions that need summaries.
	var refs []summary.SessionRef
	for _, s := range all {
		refs = append(refs, summary.SessionRef{ID: s.ID, LastUsed: s.LastUsed})
	}
	need := cache.NeedsSummary(refs)

	if len(need) == 0 {
		fmt.Fprintf(os.Stderr, "All %d sessions already have summaries.\n", len(all))
		return
	}

	fmt.Fprintf(os.Stderr, "Generating summaries for %d/%d sessions...\n", len(need), len(all))

	// Build batch items by fetching session text from providers.
	needMap := make(map[string]bool, len(need))
	for _, n := range need {
		needMap[n.ID] = true
	}

	var items []summary.BatchItem
	for _, s := range all {
		if !needMap[s.ID] {
			continue
		}
		p, ok := providerMap[s.Agent]
		if !ok {
			continue
		}
		text := p.SessionText(ctx, s.ID)
		if text == "" {
			// Use title + search text as fallback.
			text = s.Title
		}
		items = append(items, summary.BatchItem{
			ID:       s.ID,
			LastUsed: s.LastUsed,
			Text:     text,
		})
	}

	gen := summary.NewGenerator(cfg.Summary)
	succeeded := gen.GenerateBatch(ctx, items, cache, func(i, total int, id string, err error) {
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [%d/%d] %s: error: %v\n", i, total, id, err)
		} else {
			fmt.Fprintf(os.Stderr, "  [%d/%d] %s: done\n", i, total, id)
		}
	})

	if err := cache.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "sesh: warning: failed to save cache: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "Generated %d summaries (%d failed).\n", succeeded, len(items)-succeeded)
}

// lazyIndex generates summaries for unsummarized sessions in the background.
// It runs during the TUI picker and saves results to the cache.
// Errors are silently ignored — the user will see summaries next time.
func lazyIndex(ctx context.Context, cfg summary.Config, cache *summary.Cache, sessions []provider.Session, providers []provider.Provider) {
	providerMap := providersByName(providers)

	var refs []summary.SessionRef
	for _, s := range sessions {
		refs = append(refs, summary.SessionRef{ID: s.ID, LastUsed: s.LastUsed})
	}
	need := cache.NeedsSummary(refs)
	if len(need) == 0 {
		return
	}

	// Limit background generation to avoid hogging resources.
	limit := 10
	if len(need) > limit {
		need = need[:limit]
	}

	needMap := make(map[string]bool, len(need))
	for _, n := range need {
		needMap[n.ID] = true
	}

	var items []summary.BatchItem
	for _, s := range sessions {
		if !needMap[s.ID] {
			continue
		}
		p, ok := providerMap[s.Agent]
		if !ok {
			continue
		}
		text := p.SessionText(ctx, s.ID)
		if text == "" {
			text = s.Title
		}
		items = append(items, summary.BatchItem{
			ID:       s.ID,
			LastUsed: s.LastUsed,
			Text:     text,
		})
	}

	gen := summary.NewGenerator(cfg)
	gen.GenerateBatch(ctx, items, cache, nil)
}

// collectSessions gathers sessions from all providers, with warnings on failure.
func collectSessions(ctx context.Context, providers []provider.Provider, agentFilter string) []provider.Session {
	var (
		mu  sync.Mutex
		all []provider.Session
		wg  sync.WaitGroup
	)
	for _, p := range providers {
		if agentFilter != "" && p.Name() != agentFilter {
			continue
		}
		wg.Add(1)
		go func(p provider.Provider) {
			defer wg.Done()
			sessions, err := p.ListSessions(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "sesh: warning: %s: %v\n", p.Name(), err)
				return
			}
			mu.Lock()
			all = append(all, sessions...)
			mu.Unlock()
		}(p)
	}
	wg.Wait()
	return all
}

// applySummaries enriches sessions with cached summaries.
func applySummaries(sessions []provider.Session, cache *summary.Cache) {
	for i := range sessions {
		if s, ok := cache.Get(sessions[i].ID, sessions[i].LastUsed); ok {
			sessions[i].Summary = s
			// Also add summary to search text for fuzzy matching.
			sessions[i].SearchText += " " + s
		}
	}
}

func loadConfig() config {
	home, _ := os.UserHomeDir()
	paths := []string{
		filepath.Join(home, ".config", "sesh", "config.json"),
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		paths = append([]string{filepath.Join(xdg, "sesh", "config.json")}, paths...)
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var cfg config
		if err := json.Unmarshal(data, &cfg); err != nil {
			fmt.Fprintf(os.Stderr, "sesh: warning: invalid config %s: %v\n", p, err)
			continue
		}
		return cfg
	}

	return config{}
}

func buildProviders(cfg config) []provider.Provider {
	var providers []provider.Provider

	// Built-in: OpenCode.
	if oc, ok := cfg.Providers["opencode"]; ok {
		if oc.isEnabled() {
			var opts []provider.OpenCodeOption
			if oc.ResumeCommand != "" {
				opts = append(opts, provider.WithOpenCodeResumeCommand(oc.ResumeCommand))
			}
			providers = append(providers, provider.NewOpenCode(opts...))
		}
	} else {
		providers = append(providers, provider.NewOpenCode())
	}

	// Built-in: Claude Code.
	if cc, ok := cfg.Providers["claude"]; ok {
		if cc.isEnabled() {
			var opts []provider.ClaudeOption
			if cc.ResumeCommand != "" {
				opts = append(opts, provider.WithClaudeResumeCommand(cc.ResumeCommand))
			}
			providers = append(providers, provider.NewClaude(opts...))
		}
	} else {
		providers = append(providers, provider.NewClaude())
	}

	// External providers: anything in config that isn't a built-in name.
	builtins := map[string]bool{"opencode": true, "claude": true}
	for name, pc := range cfg.Providers {
		if builtins[name] || !pc.isEnabled() {
			continue
		}
		if len(pc.ListCommand) == 0 {
			fmt.Fprintf(os.Stderr, "sesh: warning: external provider %q has no list_command\n", name)
			continue
		}
		providers = append(providers, provider.NewExternal(provider.ExternalConfig{
			Name:          name,
			ListCommand:   pc.ListCommand,
			ResumeCommand: pc.ResumeCommand,
		}))
	}

	return providers
}

func providersByName(providers []provider.Provider) map[string]provider.Provider {
	m := make(map[string]provider.Provider, len(providers))
	for _, p := range providers {
		m[p.Name()] = p
	}
	return m
}
