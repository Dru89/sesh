package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dru89/sesh/internal/testhelper"
)

// stubLookPath makes exactly the named binaries appear installed.
func stubLookPath(t *testing.T, installed ...string) {
	t.Helper()
	set := make(map[string]bool, len(installed))
	for _, name := range installed {
		set[name] = true
	}
	original := lookPath
	lookPath = func(file string) (string, error) {
		if set[file] {
			return "/usr/bin/" + file, nil
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() { lookPath = original })
}

func TestDetectCandidatePrefersPurposeBuiltTool(t *testing.T) {
	// llm is one API call; the agent CLIs boot a whole harness per summary.
	// When both are present the cheaper one should win.
	stubLookPath(t, "llm", "claude", "codex")

	c, ok := detectCandidate()
	if !ok {
		t.Fatal("expected a candidate")
	}
	if c.Binary != "llm" {
		t.Errorf("detected %q, want llm", c.Binary)
	}
}

func TestDetectCandidateFallsThrough(t *testing.T) {
	stubLookPath(t, "codex")

	c, ok := detectCandidate()
	if !ok {
		t.Fatal("expected a candidate")
	}
	if c.Binary != "codex" {
		t.Errorf("detected %q, want codex", c.Binary)
	}
}

func TestDetectCandidateNoneInstalled(t *testing.T) {
	stubLookPath(t)

	if _, ok := detectCandidate(); ok {
		t.Error("expected no candidate when nothing is installed")
	}
}

// TestClaudeCandidateIsolatesWorkingDirectory pins the flag combination that
// makes `claude -p` usable here. Without --setting-sources it loads project
// settings and CLAUDE.md from the working directory and summarizes those
// instead of the transcript on stdin — measured at 1/5 success from inside a
// project, versus 3/3 with the flag. sesh is essentially always run from a
// project directory, so dropping this silently breaks summarization.
func TestClaudeCandidateIsolatesWorkingDirectory(t *testing.T) {
	var claude candidate
	for _, c := range candidates {
		if c.Binary == "claude" {
			claude = c
		}
	}
	if claude.Binary == "" {
		t.Fatal("no claude candidate in the detection table")
	}

	for _, cmd := range [][]string{claude.Fast, claude.Heavy} {
		joined := strings.Join(cmd, " ")
		if !strings.Contains(joined, "--setting-sources") {
			t.Errorf("command %q is missing --setting-sources", joined)
		}
		if !strings.Contains(joined, "--no-session-persistence") {
			t.Errorf("command %q is missing --no-session-persistence", joined)
		}
	}
}

// TestConfigKeysForFillsOnlyGaps covers the partial-config case: someone who
// already set `index` should keep it and only gain what's missing.
func TestConfigKeysForFillsOnlyGaps(t *testing.T) {
	c := candidate{Binary: "llm", Fast: []string{"llm", "-m", "haiku"}, Heavy: []string{"llm", "-m", "sonnet"}}

	cfg := config{Index: commandConfig{Command: []string{"my-own-thing"}}}
	keys := configKeysFor(c, cfg)

	if _, ok := keys["index"]; ok {
		t.Error("index was already configured; setup must not propose replacing it")
	}
	if _, ok := keys["ask"]; !ok {
		t.Error("ask was absent; setup should propose it")
	}
}

func TestConfigKeysForFullyConfiguredProposesNothing(t *testing.T) {
	c := candidate{Binary: "llm", Fast: []string{"llm"}}
	cfg := config{
		Index: commandConfig{Command: []string{"a"}},
		Ask:   askConfig{Command: []string{"b"}},
	}

	if keys := configKeysFor(c, cfg); len(keys) != 0 {
		t.Errorf("proposed %v for a fully configured setup, want nothing", keys)
	}
}

func TestConfigKeysForReusesFastWhenNoHeavy(t *testing.T) {
	c := candidate{Binary: "codex", Fast: []string{"codex", "exec"}}

	keys := configKeysFor(c, config{})

	ask, ok := keys["ask"].(map[string]any)
	if !ok {
		t.Fatalf("ask key = %#v, want a map", keys["ask"])
	}
	if got := strings.Join(ask["command"].([]string), " "); got != "codex exec" {
		t.Errorf("ask command = %q, want the fast command reused", got)
	}
}

// TestWriteConfigKeysPreservesUnknownFields is the important one. The config
// struct has no catch-all for unrecognized fields, so round-tripping through it
// would silently drop anything this binary doesn't know — a key from a newer
// sesh, or one the user hand-wrote. For a command whose job is editing someone
// else's config file, that is the failure that matters.
func TestWriteConfigKeysPreservesUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	original := `{
  "providers": {"opencode": {"resume_command": "ca opencode -s {{ID}}"}},
  "some_future_key": {"nested": [1, 2, 3]},
  "env": {"AWS_PROFILE": "work"}
}`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	err := writeConfigKeys(path, map[string]any{
		"index": map[string]any{"command": []string{"llm", "-m", "haiku"}},
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	var got map[string]json.RawMessage
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	for _, key := range []string{"providers", "some_future_key", "env"} {
		if _, ok := got[key]; !ok {
			t.Errorf("key %q was dropped", key)
		}
	}
	if _, ok := got["index"]; !ok {
		t.Error("index was not added")
	}

	// Unknown values must survive intact, not just exist.
	var future struct {
		Nested []int `json:"nested"`
	}
	if err := json.Unmarshal(got["some_future_key"], &future); err != nil {
		t.Fatalf("unknown key mangled: %v", err)
	}
	if len(future.Nested) != 3 {
		t.Errorf("unknown key value = %v, want 3 elements", future.Nested)
	}
}

func TestWriteConfigKeysBacksUpExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	original := `{"env": {"A": "1"}}`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	if err := writeConfigKeys(path, map[string]any{"index": map[string]any{}}); err != nil {
		t.Fatalf("write: %v", err)
	}

	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("no backup written: %v", err)
	}
	if string(backup) != original {
		t.Errorf("backup = %q, want the original contents", backup)
	}
}

// TestWriteConfigKeysRefusesUnparseable guards against destroying a config that
// merely has a syntax error — the user's content is still in there and a
// clobbering write would lose it.
func TestWriteConfigKeysRefusesUnparseable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	broken := `{"index": {"command": ["llm"` // truncated
	if err := os.WriteFile(path, []byte(broken), 0644); err != nil {
		t.Fatal(err)
	}

	if err := writeConfigKeys(path, map[string]any{"ask": map[string]any{}}); err == nil {
		t.Fatal("expected an error for an unparseable config")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != broken {
		t.Errorf("file was modified despite the error: %q", after)
	}
}

func TestWriteConfigKeysCreatesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "config.json")

	if err := writeConfigKeys(path, map[string]any{"index": map[string]any{"command": []string{"llm"}}}); err != nil {
		t.Fatalf("write: %v", err)
	}

	var got map[string]json.RawMessage
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["$schema"]; !ok {
		t.Error("new config should carry a $schema reference for editor completion")
	}
	if _, err := os.Stat(path + ".bak"); err == nil {
		t.Error("no backup should be written when there was no original")
	}
}

// TestSummarizesCanaryToleratesParaphrase is the lesson from testing against a
// real model: asked for fifteen words, it returned "Renaming a helper function
// in cache.go and updating its call sites" — dropping the proper noun entirely.
// Requiring one exact token would have rejected a working configuration.
func TestSummarizesCanaryToleratesParaphrase(t *testing.T) {
	valid := []string{
		"Rename quokka helper in cache.go and update call sites",
		"Renaming a helper function in cache.go and updating its call sites",
		"Refactor: rename helper, update the two call sites",
	}
	for _, out := range valid {
		if !summarizesCanary(out) {
			t.Errorf("rejected a valid summary: %q", out)
		}
	}
}

// TestSummarizesCanaryRejectsWrongSubject covers what verification exists for:
// output that is fluent, non-empty, and about something else entirely.
func TestSummarizesCanaryRejectsWrongSubject(t *testing.T) {
	invalid := []string{
		// The working-directory context leak.
		"Implement setup subcommand with LLM CLI detection and interactive configuration",
		"Senior engineer at Disney. Peer communication. Worktrunk, Sift, Tuesday standup.",
		// A refusal or request for clarification.
		"I don't see a session transcript in your message. Please provide the transcript.",
		"No transcript provided. Please share the session transcript to generate a label.",
		"",
	}
	for _, out := range invalid {
		if summarizesCanary(out) {
			t.Errorf("accepted output about the wrong subject: %q", out)
		}
	}
}

func TestVerifyCommandAcceptsWorkingCommand(t *testing.T) {
	cmd := testhelper.WriteMockScript(t,
		"#!/bin/sh\ncat >/dev/null\necho 'Rename quokka helper in cache.go'\n",
		"$null = [Console]::In.ReadToEnd()\nWrite-Output 'Rename quokka helper in cache.go'\n",
	)

	if err := verifyCommand(context.Background(), cmd, nil); err != nil {
		t.Errorf("verify: %v", err)
	}
}

func TestVerifyCommandRejectsWrongSubject(t *testing.T) {
	cmd := testhelper.WriteMockScript(t,
		"#!/bin/sh\ncat >/dev/null\necho 'I do not see a transcript in your message.'\n",
		"$null = [Console]::In.ReadToEnd()\nWrite-Output 'I do not see a transcript in your message.'\n",
	)

	err := verifyCommand(context.Background(), cmd, nil)
	if err == nil {
		t.Fatal("expected verification to reject output about the wrong subject")
	}
	if !strings.Contains(err.Error(), "did not describe the test transcript") {
		t.Errorf("error = %v, want it to explain the mismatch", err)
	}
}

func TestVerifyCommandReportsFailure(t *testing.T) {
	cmd := testhelper.WriteMockScript(t,
		"#!/bin/sh\ncat >/dev/null\necho 'model not found' >&2\nexit 1\n",
		"$null = [Console]::In.ReadToEnd()\n[Console]::Error.WriteLine('model not found')\nexit 1\n",
	)

	err := verifyCommand(context.Background(), cmd, nil)
	if err == nil {
		t.Fatal("expected an error for a failing command")
	}
	if !strings.Contains(err.Error(), "model not found") {
		t.Errorf("error = %v, want it to surface the command's stderr", err)
	}
}

// TestConfiguredCommandsCollapsesDuplicates: the fallback chains mean a config
// setting only `index` resolves all four slots to the same command. Running the
// canary against it four times would just be slow.
func TestConfiguredCommandsCollapsesDuplicates(t *testing.T) {
	cfg := config{Index: commandConfig{Command: []string{"llm", "-m", "haiku"}}}

	got := configuredCommands(cfg)

	if len(got) != 1 {
		t.Fatalf("got %d distinct commands, want 1", len(got))
	}
	if len(got[0].Slots) != 4 {
		t.Errorf("slots = %v, want all four attributed to the one command", got[0].Slots)
	}
}

func TestConfiguredCommandsSeparatesDistinct(t *testing.T) {
	cfg := config{
		Index: commandConfig{Command: []string{"llm", "-m", "haiku"}},
		Ask:   askConfig{Command: []string{"llm", "-m", "sonnet"}},
	}

	got := configuredCommands(cfg)

	if len(got) != 2 {
		t.Fatalf("got %d distinct commands, want 2", len(got))
	}
}

// TestDisplayCommandQuotesEmptyArgs: `--setting-sources ""` printed bare looks
// like a flag with no argument, and copying that line reintroduces the
// working-directory leak the flag prevents.
func TestDisplayCommandQuotesEmptyArgs(t *testing.T) {
	got := displayCommand([]string{"claude", "-p", "--setting-sources", ""})

	if !strings.HasSuffix(got, `--setting-sources ""`) {
		t.Errorf("displayCommand() = %q, want the empty argument visible", got)
	}
}

func TestDisplayCommandLeavesOrdinaryArgsAlone(t *testing.T) {
	got := displayCommand([]string{"llm", "-m", "haiku"})

	if got != "llm -m haiku" {
		t.Errorf("displayCommand() = %q, want it unquoted", got)
	}
}

func TestConfiguredCommandsEmptyWhenUnconfigured(t *testing.T) {
	if got := configuredCommands(config{}); len(got) != 0 {
		t.Errorf("got %v, want nothing for an empty config", got)
	}
}

// TestSetupHintStopsAfterLimit: one sighting is easy to miss, but someone who
// ignored it three times has decided.
func TestSetupHintStopsAfterLimit(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	stubLookPath(t, "llm")

	for i := 0; i < setupHintLimit; i++ {
		maybeShowSetupHint(setupHintMinSessions)
	}
	if got := loadSetupHintState().Shown; got != setupHintLimit {
		t.Fatalf("shown = %d, want %d", got, setupHintLimit)
	}

	maybeShowSetupHint(setupHintMinSessions)
	if got := loadSetupHintState().Shown; got != setupHintLimit {
		t.Errorf("shown = %d after exceeding the limit, want it capped at %d", got, setupHintLimit)
	}
}

func TestSetupHintSilentBelowSessionThreshold(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	stubLookPath(t, "llm")

	maybeShowSetupHint(setupHintMinSessions - 1)

	if got := loadSetupHintState().Shown; got != 0 {
		t.Errorf("shown = %d, want 0 below the session threshold", got)
	}
}

// TestSetupHintSilentWithNothingDetected: suggesting setup to someone with no
// supported CLI installed is pure noise — there is nothing for them to do.
func TestSetupHintSilentWithNothingDetected(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	stubLookPath(t)

	maybeShowSetupHint(setupHintMinSessions * 10)

	if got := loadSetupHintState().Shown; got != 0 {
		t.Errorf("shown = %d, want 0 when nothing was detected", got)
	}
}

// TestSetupHintSuppressed: running `sesh setup` stops the nudge for good,
// including when the user declines the write. Asking again after someone said
// no is worse than never asking.
func TestSetupHintSuppressed(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	stubLookPath(t, "llm")

	suppressSetupHint()
	maybeShowSetupHint(setupHintMinSessions * 10)

	state := loadSetupHintState()
	if !state.Suppressed {
		t.Error("expected suppression to persist")
	}
	if state.Shown != 0 {
		t.Errorf("shown = %d, want 0 once suppressed", state.Shown)
	}
}
