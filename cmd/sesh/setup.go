package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dru89/sesh/summary"
)

// candidate is an LLM command sesh knows how to configure on the user's behalf.
//
// Ordering is deliberate. `llm` comes first because it is purpose-built for
// this shape of work — one API call, sub-second, no agent harness to boot. The
// agent CLIs are viable but pay several seconds of startup per summary, so they
// are a fallback rather than a preference.
//
// The candidate's presence on PATH is also the strongest available signal about
// which account the user has authenticated. Session mix is a weaker one:
// OpenCode is model-agnostic, so a session list dominated by it says nothing
// about whether an Anthropic or OpenAI credential exists.
type candidate struct {
	// Binary is what must be on PATH for this candidate to apply.
	Binary string

	// Fast is the command for high-volume, low-capability work: title
	// generation (sesh index) and the ask filter pass.
	Fast []string

	// Heavy is the command for prose generation (sesh ask, sesh recap). Empty
	// means reuse Fast — the config fallback chains handle the rest.
	Heavy []string

	// Note is shown to the user when this candidate is chosen, to explain any
	// non-obvious flags.
	Note string
}

// candidates is the detection table, in priority order.
//
// Model aliases rather than pinned IDs: an alias floats in version but holds in
// cost tier, which is the dimension that matters here (any model can write a
// fifteen-word title). A pinned ID is stable today and on a deprecation clock,
// and would eventually 404 with no one tracking the date.
var candidates = []candidate{
	{
		Binary: "llm",
		Fast:   []string{"llm", "-m", "haiku"},
		Heavy:  []string{"llm", "-m", "sonnet"},
	},
	{
		Binary: "claude",
		// --setting-sources "" is load-bearing, not tidiness. Without it,
		// `claude -p` loads project settings and CLAUDE.md from the working
		// directory and answers about *those* instead of the transcript on
		// stdin — and sesh is essentially always run from inside a project.
		// The failure is silent: exit 0, plausible English, wrong content.
		//
		// --no-session-persistence keeps summarization from writing session
		// transcripts of its own, and --strict-mcp-config skips MCP server
		// startup that a one-shot summary has no use for.
		Fast: []string{
			"claude", "-p", "--model", "haiku",
			"--no-session-persistence", "--strict-mcp-config", "--setting-sources", "",
		},
		Heavy: []string{
			"claude", "-p", "--model", "sonnet",
			"--no-session-persistence", "--strict-mcp-config", "--setting-sources", "",
		},
		Note: "uses your existing Claude Code login; ~5-10s per summary",
	},
	{
		Binary: "codex",
		Fast:   []string{"codex", "exec"},
		Note:   "verify the invocation if summaries come back malformed",
	},
}

// lookPath is exec.LookPath, indirected so tests can control which binaries
// appear installed without depending on PATH semantics that differ across
// platforms (PATHEXT on Windows, the executable bit on Unix).
var lookPath = exec.LookPath

// detectCandidate returns the first candidate whose binary is on PATH.
func detectCandidate() (candidate, bool) {
	for _, c := range candidates {
		if _, err := lookPath(c.Binary); err == nil {
			return c, true
		}
	}
	return candidate{}, false
}

const canaryTranscript = "user: rename the quokka helper in cache.go\n" +
	"assistant: renamed it and updated the two call sites\n"

// canaryAnchors are distinctive details from canaryTranscript. A valid summary
// of it should mention at least one.
//
// A bare "did it produce output" check is not enough: the `claude -p` context
// leak exits 0 with fluent English about entirely the wrong subject, and a
// model that refuses or asks for clarification also produces non-empty output.
// Neither mentions any of these.
//
// Matching on several anchors rather than one specific word is deliberate.
// Summarizers legitimately paraphrase — asked for fifteen words, one model
// returned "Renaming a helper function in cache.go and updating its call
// sites", dropping the proper noun entirely. Requiring that exact token would
// have rejected a working configuration. These are chosen to be things a
// summary of this transcript would struggle to omit *all* of, while being
// vanishingly unlikely to appear in a response about anything else.
var canaryAnchors = []string{"quokka", "cache.go", "call site"}

// displayCommand renders a command for humans, quoting arguments that would
// otherwise be invisible or ambiguous.
//
// The empty-string case is the one that matters: `--setting-sources ""` prints
// as a bare trailing flag, and anyone copying that line to run it by hand loses
// the argument — reintroducing the working-directory leak the flag exists to
// prevent.
func displayCommand(cmd []string) string {
	parts := make([]string, len(cmd))
	for i, a := range cmd {
		if a == "" || strings.ContainsAny(a, " \t\n\"'") {
			parts[i] = fmt.Sprintf("%q", a)
			continue
		}
		parts[i] = a
	}
	return strings.Join(parts, " ")
}

// summarizesCanary reports whether output looks like a summary of the canary
// transcript rather than of something else entirely.
func summarizesCanary(out string) bool {
	lower := strings.ToLower(out)
	for _, anchor := range canaryAnchors {
		if strings.Contains(lower, anchor) {
			return true
		}
	}
	return false
}

// verifyCommand runs a command against the canary transcript and reports
// whether the output looks like a real summary of it.
//
// This goes through summary.Generator rather than calling the command directly,
// so verification exercises the exact production path — same prompt assembly,
// same excerpting, same 30-second per-call timeout. A command that can't make
// that deadline here wouldn't make it during indexing either, so inheriting the
// limit is the point rather than a limitation.
func verifyCommand(ctx context.Context, command []string, env []string) error {
	cfg := summary.Config{Command: command, Env: env}
	gen := summary.NewGenerator(cfg)

	out, err := gen.Generate(ctx, canaryTranscript)
	if err != nil {
		return err
	}
	if !summarizesCanary(out) {
		return fmt.Errorf("the command ran but did not describe the test transcript — "+
			"expected a mention of one of %s, got %q",
			strings.Join(canaryAnchors, ", "), summary.CondenseError(out))
	}
	return nil
}

// configKeysFor builds the config keys to add for a candidate, limited to the
// keys that are actually missing.
//
// Two keys cover all four resolution chains: `index` serves title generation
// and, via its chain, the ask filter pass; `ask` serves prose generation and is
// inherited by recap. Writing more would be redundant, and writing only `index`
// would send heavy prose work to a fast model.
func configKeysFor(c candidate, cfg config) map[string]any {
	keys := make(map[string]any)

	if len(cfg.Index.Command) == 0 {
		keys["index"] = map[string]any{"command": c.Fast}
	}
	if len(cfg.Ask.Command) == 0 {
		heavy := c.Heavy
		if len(heavy) == 0 {
			heavy = c.Fast
		}
		keys["ask"] = map[string]any{"command": heavy}
	}

	return keys
}

// writeConfigKeys merges top-level keys into the config file, preserving every
// other key in the file.
//
// It deliberately does not round-trip through the typed config struct. Neither
// config nor providerConfig has a catch-all for unknown fields, so
// unmarshal-then-marshal silently drops anything this binary doesn't know
// about — a key added by a newer sesh, or one the user hand-wrote. For a
// command whose entire job is editing someone's config file, that is the bug
// that matters most. Working at the json.RawMessage level keeps unrecognized
// keys byte-intact.
//
// Key order is normalized (encoding/json sorts map keys) and indentation is
// unified. Content is preserved; layout is not.
func writeConfigKeys(path string, keys map[string]any) error {
	raw := make(map[string]json.RawMessage)

	existing, readErr := os.ReadFile(path)
	if readErr == nil {
		if err := json.Unmarshal(existing, &raw); err != nil {
			return fmt.Errorf("refusing to overwrite unparseable config %s: %w", path, err)
		}
		// Back up before the first modification, so a bad merge is recoverable.
		if err := os.WriteFile(path+".bak", existing, 0644); err != nil {
			return fmt.Errorf("write config backup: %w", err)
		}
	}

	for k, v := range keys {
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("marshal config key %q: %w", k, err)
		}
		raw[k] = b
	}

	// Point new configs at the schema so editors offer completion.
	if _, ok := raw["$schema"]; !ok {
		raw["$schema"] = json.RawMessage(
			`"https://raw.githubusercontent.com/dru89/sesh/main/schema.json"`)
	}

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename config: %w", err)
	}
	return nil
}

// configuredCommands returns the resolved commands to verify, labelled with the
// config slots they serve.
//
// Duplicates are collapsed: the fallback chains mean a config setting only
// `index` resolves all four slots to the same command, and running the canary
// against it four times would just be slow.
func configuredCommands(cfg config) []labelledCommand {
	type slot struct {
		name string
		cmd  []string
		env  map[string]string
	}

	idxCmd, idxEnv := cfg.indexCommand()
	askCmd, askEnv := cfg.askCommand()
	filterCmd, filterEnv := cfg.askFilterCommand()
	recapCmd, recapEnv := cfg.recapCommand()

	slots := []slot{
		{"index", idxCmd, idxEnv},
		{"ask", askCmd, askEnv},
		{"ask.filter_command", filterCmd, filterEnv},
		{"recap", recapCmd, recapEnv},
	}

	var out []labelledCommand
	index := make(map[string]int) // joined command -> position in out
	for _, s := range slots {
		if len(s.cmd) == 0 {
			continue
		}
		key := strings.Join(s.cmd, "\x00")
		if i, ok := index[key]; ok {
			out[i].Slots = append(out[i].Slots, s.name)
			continue
		}
		index[key] = len(out)
		out = append(out, labelledCommand{
			Slots:   []string{s.name},
			Command: s.cmd,
			Env:     cfg.buildEnv(s.env),
		})
	}
	return out
}

type labelledCommand struct {
	Slots   []string
	Command []string
	Env     []string
}

// setupHintLimit is how many times the picker suggests running `sesh setup`
// before giving up for good.
//
// One sighting is easy to miss: the hint is written to stderr just before the
// TUI takes the alt screen, so it is only visible once the picker exits. Three
// is a fair chance without becoming a nag — someone who ignored it three times
// has decided. There is deliberately no time component; a decayed re-prompt
// every N days is just a slower nag for what is a one-time onboarding nudge.
const setupHintLimit = 3

// setupHintMinSessions is the point at which AI titles start being worth the
// setup. Matches the existing cache-warming threshold so the two hints agree on
// what "enough sessions to bother" means.
const setupHintMinSessions = 20

type setupHintState struct {
	Shown      int  `json:"shown"`
	Suppressed bool `json:"suppressed"`
}

func setupHintPath() string {
	return filepath.Join(summary.CacheDir(), "setup-hint.json")
}

func loadSetupHintState() setupHintState {
	var s setupHintState
	data, err := os.ReadFile(setupHintPath())
	if err != nil {
		return s
	}
	// A corrupt file just means starting the count over; nothing worth warning
	// about, and at worst the user sees the hint a few extra times.
	_ = json.Unmarshal(data, &s)
	return s
}

func saveSetupHintState(s setupHintState) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(setupHintPath()), 0755); err != nil {
		return
	}
	_ = os.WriteFile(setupHintPath(), data, 0644)
}

// suppressSetupHint stops the hint permanently.
func suppressSetupHint() {
	s := loadSetupHintState()
	if s.Suppressed {
		return
	}
	s.Suppressed = true
	saveSetupHintState(s)
}

// maybeShowSetupHint suggests `sesh setup` when it would actually help.
//
// Every condition has to hold: no LLM command is configured (the caller's
// branch guarantees it), something was detected to configure, the user has
// enough sessions for titles to matter, and the hint has not been shown too
// often already. Suggesting setup to someone with no supported CLI installed,
// or three sessions, is noise.
//
// The picker path only. `sesh list`, `--json`, and `--ai-search` back other
// tools (Raycast among them) where an onboarding nudge is just chatter.
func maybeShowSetupHint(sessionCount int) {
	if sessionCount < setupHintMinSessions {
		return
	}
	state := loadSetupHintState()
	if state.Suppressed || state.Shown >= setupHintLimit {
		return
	}
	c, ok := detectCandidate()
	if !ok {
		return
	}

	fmt.Fprintf(os.Stderr, "sesh: found %s on your PATH — run 'sesh setup' to enable AI titles and search.\n", c.Binary)

	state.Shown++
	saveSetupHintState(state)
}

// runSetup handles the `sesh setup` subcommand: detect an available LLM CLI,
// propose a config, write it on confirmation, and verify the result works.
//
// It is also useful with nothing to add. A fully-configured user gets a pure
// diagnostic — every resolved command run against the canary — which is the
// thing to reach for when summaries mysteriously stop appearing.
func runSetup(args []string) {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	assumeYes := fs.Bool("yes", false, "Write the config without asking for confirmation")
	verifyOnly := fs.Bool("verify", false, "Only check the current configuration; never write")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: sesh setup [options]\n\n")
		fmt.Fprintf(os.Stderr, "Detect an available LLM CLI and configure sesh to use it for\n")
		fmt.Fprintf(os.Stderr, "titles, search, and recaps. Existing settings are never overwritten.\n\n")
		fmt.Fprintf(os.Stderr, "Run it again any time to check that the configured commands still work.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}
	fs.Parse(args)

	// Reaching this command at all means the user knows the feature exists, so
	// stop nudging them about it regardless of how this run ends — including if
	// they decline the write. Asking again after someone said no is worse than
	// never asking.
	suppressSetupHint()

	cfg, path, found := loadConfigWithPath()
	ctx := context.Background()

	if *verifyOnly {
		os.Exit(verifyConfigured(ctx, cfg))
	}

	c, detected := detectCandidate()
	var keys map[string]any
	if detected {
		keys = configKeysFor(c, cfg)
	}

	if len(keys) == 0 {
		// Nothing to add: either everything is already set, or nothing was
		// found to set it to.
		if len(configuredCommands(cfg)) > 0 {
			fmt.Fprintf(os.Stderr, "sesh is already configured (%s).\n\n", path)
			os.Exit(verifyConfigured(ctx, cfg))
		}
		reportNothingDetected()
		os.Exit(1)
	}

	// Propose. Because setup only ever adds absent keys and never modifies an
	// existing one, the list of keys being added is the complete diff.
	verb := "Creating"
	if found {
		verb = "Updating"
	}
	fmt.Fprintf(os.Stderr, "Found %s on your PATH.\n", c.Binary)
	if c.Note != "" {
		fmt.Fprintf(os.Stderr, "  (%s)\n", c.Note)
	}
	fmt.Fprintf(os.Stderr, "\n%s %s with:\n\n%s\n", verb, path, indentJSON(keys))
	if found && len(configuredCommands(cfg)) > 0 {
		fmt.Fprintf(os.Stderr, "Your existing settings are left untouched.\n\n")
	}

	if !*assumeYes && !confirm() {
		fmt.Fprintf(os.Stderr, "Aborted. Nothing was written.\n")
		os.Exit(1)
	}

	if err := writeConfigKeys(path, keys); err != nil {
		fmt.Fprintf(os.Stderr, "sesh: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Wrote %s\n\n", path)

	// Verify against what is now on disk rather than what we intended to
	// write, so a merge that went sideways is caught here.
	updated, _, _ := loadConfigWithPath()
	os.Exit(verifyConfigured(ctx, updated))
}

// verifyConfigured runs the canary against every distinct configured command
// and returns the process exit code.
func verifyConfigured(ctx context.Context, cfg config) int {
	cmds := configuredCommands(cfg)
	if len(cmds) == 0 {
		fmt.Fprintf(os.Stderr, "No LLM command is configured, so there is nothing to check.\n")
		fmt.Fprintf(os.Stderr, "Run 'sesh setup' without --verify to configure one.\n")
		return 1
	}

	failed := 0
	for _, lc := range cmds {
		fmt.Fprintf(os.Stderr, "Checking %s: %s\n", strings.Join(lc.Slots, ", "), displayCommand(lc.Command))
		if err := verifyCommand(ctx, lc.Command, lc.Env); err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "  \033[31mfailed: %s\033[0m\n", summary.CondenseError(err.Error()))
			continue
		}
		fmt.Fprintf(os.Stderr, "  \033[32mok\033[0m\n")
	}

	if failed > 0 {
		fmt.Fprintf(os.Stderr, "\n%d of %d commands failed.\n", failed, len(cmds))
		return 1
	}

	fmt.Fprintf(os.Stderr, "\nAll %d command(s) working.\n", len(cmds))
	// A working setup clears any stale failure hint left by earlier runs.
	health := summary.NewHealth()
	health.Reset()
	if err := health.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "sesh: warning: failed to reset index health: %v\n", err)
	}
	return 0
}

func reportNothingDetected() {
	fmt.Fprintf(os.Stderr, "No supported LLM CLI found on your PATH.\n\n")
	fmt.Fprintf(os.Stderr, "sesh looked for:\n")
	for _, c := range candidates {
		fmt.Fprintf(os.Stderr, "  %s\n", c.Binary)
	}
	fmt.Fprintf(os.Stderr, "\nAny command that reads a transcript on stdin and writes a summary to\n")
	fmt.Fprintf(os.Stderr, "stdout will work. Configure one by hand under \"index\" in:\n  %s\n", configPaths()[0])
	fmt.Fprintf(os.Stderr, "\nSee https://github.com/dru89/sesh#configuring-llm-commands\n")
}

// indentJSON renders the proposed keys the way they will appear in the file.
func indentJSON(keys map[string]any) string {
	data, err := json.MarshalIndent(keys, "  ", "  ")
	if err != nil {
		return fmt.Sprintf("  %v", keys)
	}
	return "  " + string(data)
}

// confirm asks for a yes/no on stderr and reads the answer from stdin.
//
// A closed or piped stdin reads as EOF, which is treated as "no" rather than
// silently proceeding — a config write is not something to do because nobody
// was there to object. Non-interactive callers pass --yes.
func confirm() bool {
	fmt.Fprintf(os.Stderr, "Write this config? [y/N] ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		fmt.Fprintf(os.Stderr, "\n(no answer; re-run with --yes to write without confirming)\n")
		return false
	}

	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}
