package summary

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dru89/sesh/internal/testhelper"
)

func TestRunLLMSuccess(t *testing.T) {
	cmd := testhelper.WriteMockScript(t,
		"#!/bin/sh\ncat",
		"[Console]::Out.Write([Console]::In.ReadToEnd())",
	)
	result, err := RunLLM(context.Background(), cmd, nil, "hello world", 5*time.Second)
	if err != nil {
		t.Fatalf("RunLLM failed: %v", err)
	}
	if result != "hello world" {
		t.Errorf("got %q, want %q", result, "hello world")
	}
}

func TestRunLLMTruncatesWhitespace(t *testing.T) {
	cmd := testhelper.WriteMockScript(t,
		"#!/bin/sh\necho '  trimmed  '",
		"Write-Output '  trimmed  '",
	)
	result, err := RunLLM(context.Background(), cmd, nil, "", 5*time.Second)
	if err != nil {
		t.Fatalf("RunLLM failed: %v", err)
	}
	if result != "trimmed" {
		t.Errorf("got %q, want %q", result, "trimmed")
	}
}

func TestRunLLMEmptyOutput(t *testing.T) {
	cmd := testhelper.WriteMockScript(t,
		"#!/bin/sh\ntrue",
		"exit 0",
	)
	_, err := RunLLM(context.Background(), cmd, nil, "input", 5*time.Second)
	if err == nil {
		t.Error("expected error for empty output")
	}
}

func TestRunLLMCommandFailure(t *testing.T) {
	cmd := testhelper.WriteMockScript(t,
		"#!/bin/sh\nexit 1",
		"exit 1",
	)
	_, err := RunLLM(context.Background(), cmd, nil, "input", 5*time.Second)
	if err == nil {
		t.Error("expected error for failed command")
	}
}

func TestRunLLMCommandNotFound(t *testing.T) {
	_, err := RunLLM(context.Background(), []string{"/nonexistent/binary"}, nil, "input", 5*time.Second)
	if err == nil {
		t.Error("expected error for missing command")
	}
}

func TestRunLLMTimeout(t *testing.T) {
	cmd := testhelper.WriteMockScript(t,
		"#!/bin/sh\nsleep 10",
		"Start-Sleep 10",
	)
	_, err := RunLLM(context.Background(), cmd, nil, "input", 100*time.Millisecond)
	if err == nil {
		t.Error("expected error for timeout")
	}
}

func TestRunLLMNoCommand(t *testing.T) {
	_, err := RunLLM(context.Background(), nil, nil, "input", 5*time.Second)
	if err == nil {
		t.Error("expected error for nil command")
	}
}

func TestRunLLMStderrInError(t *testing.T) {
	cmd := testhelper.WriteMockScript(t,
		"#!/bin/sh\necho 'bad model' >&2\nexit 1",
		"[Console]::Error.WriteLine('bad model'); exit 1",
	)
	_, err := RunLLM(context.Background(), cmd, nil, "input", 5*time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got == "" {
		t.Error("error message should include stderr")
	}
}

func TestRunLLMWithEnv(t *testing.T) {
	cmd := testhelper.WriteMockScript(t,
		"#!/bin/sh\necho $TEST_LLM_ENV",
		"Write-Output $env:TEST_LLM_ENV",
	)
	env := append(os.Environ(), "TEST_LLM_ENV=hello_from_env")
	result, err := RunLLM(context.Background(), cmd, env, "input", 5*time.Second)
	if err != nil {
		t.Fatalf("RunLLM failed: %v", err)
	}
	if result != "hello_from_env" {
		t.Errorf("got %q, want %q", result, "hello_from_env")
	}
}

func TestGenerateSuccess(t *testing.T) {
	cmd := testhelper.WriteMockScript(t,
		"#!/bin/sh\necho 'Built JWT auth middleware'",
		"Write-Output 'Built JWT auth middleware'",
	)
	gen := NewGenerator(Config{Command: cmd})
	result, err := gen.Generate(context.Background(), "session text here")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if result != "Built JWT auth middleware" {
		t.Errorf("got %q, want %q", result, "Built JWT auth middleware")
	}
}

func TestGenerateTruncatesLongOutput(t *testing.T) {
	long := strings.Repeat("x", 300)
	cmd := testhelper.WriteMockScript(t,
		"#!/bin/sh\nprintf '"+long+"'",
		"Write-Output '"+long+"'",
	)
	gen := NewGenerator(Config{Command: cmd})
	result, err := gen.Generate(context.Background(), "input")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if len(result) != 200 {
		t.Errorf("expected truncated to 200, got %d", len(result))
	}
}

func TestGenerateNotConfigured(t *testing.T) {
	gen := NewGenerator(Config{})
	_, err := gen.Generate(context.Background(), "input")
	if err == nil {
		t.Error("expected error when not configured")
	}
}

func TestGenerateBatch(t *testing.T) {
	cmd := testhelper.WriteMockScript(t,
		"#!/bin/sh\necho 'summary'",
		"Write-Output 'summary'",
	)
	gen := NewGenerator(Config{Command: cmd})
	cache := newTestCache(t)

	items := []BatchItem{
		{ID: "ses_1", LastUsed: time.Now(), Text: "text 1"},
		{ID: "ses_2", LastUsed: time.Now(), Text: "text 2"},
	}

	var progress []int
	succeeded := gen.GenerateBatch(context.Background(), items, cache, func(i, total int, id string, err error) {
		progress = append(progress, i)
	})

	if succeeded != 2 {
		t.Errorf("expected 2 succeeded, got %d", succeeded)
	}
	if len(progress) != 2 {
		t.Errorf("expected 2 progress calls, got %d", len(progress))
	}
	if cache.Len() != 2 {
		t.Errorf("expected 2 cached entries, got %d", cache.Len())
	}
}

func TestGenerateBatchPartialFailure(t *testing.T) {
	cmd := testhelper.WriteMockScript(t,
		"#!/bin/sh\ninput=$(cat)\ncase \"$input\" in *fail*) exit 1;; esac\necho 'ok'",
		"$content = [Console]::In.ReadToEnd()\nif ($content -match 'fail') { exit 1 }\nWrite-Output 'ok'",
	)
	gen := NewGenerator(Config{Command: cmd})
	cache := newTestCache(t)

	items := []BatchItem{
		{ID: "ses_1", LastUsed: time.Now(), Text: "good"},
		{ID: "ses_2", LastUsed: time.Now(), Text: "fail"},
		{ID: "ses_3", LastUsed: time.Now(), Text: "also good"},
	}

	succeeded := gen.GenerateBatch(context.Background(), items, cache, nil)
	if succeeded != 2 {
		t.Errorf("expected 2 succeeded, got %d", succeeded)
	}
	if cache.Len() != 2 {
		t.Errorf("expected 2 cached, got %d", cache.Len())
	}
}

func TestBuildPrompt(t *testing.T) {
	t.Run("all defaults", func(t *testing.T) {
		got := BuildPrompt("", "", "default system", "default task", "the transcript")
		want := "default system\n\n---\n\nthe transcript\n\n---\n\ndefault task"
		if got != want {
			t.Errorf("got:\n%s\n\nwant:\n%s", got, want)
		}
	})

	t.Run("custom system prompt replaces default", func(t *testing.T) {
		got := BuildPrompt("custom system", "", "default system", "default task", "transcript")
		if !strings.HasPrefix(got, "custom system\n") {
			t.Errorf("expected custom system prefix, got:\n%s", got)
		}
		if strings.Contains(got, "default system") {
			t.Error("default system should not appear when custom is set")
		}
	})

	t.Run("custom prompt replaces default", func(t *testing.T) {
		got := BuildPrompt("", "custom task", "default system", "default task", "transcript")
		if !strings.Contains(got, "custom task") {
			t.Error("expected custom task in output")
		}
		if strings.Contains(got, "default task") {
			t.Error("default task should not appear when custom is set")
		}
	})

	t.Run("both custom", func(t *testing.T) {
		got := BuildPrompt("my system", "my task", "default system", "default task", "transcript")
		want := "my system\n\n---\n\ntranscript\n\n---\n\nmy task"
		if got != want {
			t.Errorf("got:\n%s\n\nwant:\n%s", got, want)
		}
	})

	t.Run("template variable in prompt", func(t *testing.T) {
		got := BuildPrompt("", "Here is the data: {{TRANSCRIPT}} Now label it.", "default system", "default task", "session data")
		want := "default system\n\nHere is the data: session data Now label it."
		if got != want {
			t.Errorf("got:\n%s\n\nwant:\n%s", got, want)
		}
	})

	t.Run("template variable with custom system", func(t *testing.T) {
		got := BuildPrompt("custom system", "Label: {{TRANSCRIPT}}", "default system", "default task", "text")
		if !strings.HasPrefix(got, "custom system\n") {
			t.Errorf("expected custom system prefix, got:\n%s", got)
		}
		if !strings.Contains(got, "Label: text") {
			t.Errorf("expected expanded template, got:\n%s", got)
		}
		if strings.Contains(got, "{{TRANSCRIPT}}") {
			t.Error("template variable should be expanded")
		}
	})

	t.Run("no separator when using template variable", func(t *testing.T) {
		got := BuildPrompt("", "Data: {{TRANSCRIPT}}", "sys", "task", "content")
		if strings.Contains(got, "---") {
			t.Errorf("should not contain --- separators in template mode, got:\n%s", got)
		}
	})
}

func TestGenerateWithSystemPrompt(t *testing.T) {
	cmd := testhelper.WriteMockScript(t,
		"#!/bin/sh\ncat",
		"[Console]::Out.Write([Console]::In.ReadToEnd())",
	)
	gen := NewGenerator(Config{
		Command:      cmd,
		SystemPrompt: "You are a test assistant.",
	})

	result, err := gen.Generate(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if !strings.HasPrefix(result, "You are a test assistant.") {
		t.Errorf("expected custom system prompt at start, got:\n%s", result)
	}
	if !strings.Contains(result, "hello") {
		t.Error("expected transcript in output")
	}
}

func TestStripMarkdown(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text", "hello world", "hello world"},
		{"bold", "this is **bold** text", "this is bold text"},
		{"inline code", "use `sesh` to search", "use sesh to search"},
		{"heading", "## Session Summary\nthe content", "Session Summary the content"},
		{"hrule", "---\ncontent after", "content after"},
		{"multiline collapses", "line one\nline two\nline three", "line one line two line three"},
		{"crlf collapses", "line one\r\nline two", "line one line two"},
		{"double spaces cleaned", "too  many   spaces", "too many spaces"},
		{"leading list marker dash", "- Built auth middleware", "Built auth middleware"},
		{"leading list marker bullet", "• Fixed login bug", "Fixed login bug"},
		{"leading list marker star", "* Refactored API", "Refactored API"},
		{"combined", "## Summary\n**Built** `auth` middleware\n- for the API", "Summary Built auth middleware - for the API"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("StripMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestGenerateBatchProcessesEveryItem verifies the worker pool doesn't drop or
// duplicate work — every item is summarized exactly once regardless of how the
// jobs are distributed.
func TestGenerateBatchProcessesEveryItem(t *testing.T) {
	cmd := testhelper.WriteMockScript(t,
		"#!/bin/sh\ncat >/dev/null\necho summary\n",
		"$null = [Console]::In.ReadToEnd()\nWrite-Output 'summary'\n",
	)
	gen := NewGenerator(Config{Command: cmd, Concurrency: 4})
	cache := newTestCache(t)

	items := make([]BatchItem, 12)
	for i := range items {
		items[i] = BatchItem{ID: fmt.Sprintf("s%d", i), Text: "some session text", LastUsed: time.Now()}
	}

	var (
		mu   sync.Mutex
		seen = map[string]int{}
	)
	succeeded := gen.GenerateBatch(context.Background(), items, cache, func(done, total int, id string, err error) {
		mu.Lock()
		defer mu.Unlock()
		seen[id]++
	})

	if succeeded != len(items) {
		t.Errorf("succeeded = %d, want %d", succeeded, len(items))
	}
	if len(seen) != len(items) {
		t.Errorf("reported on %d distinct items, want %d", len(seen), len(items))
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("item %s reported %d times, want 1", id, n)
		}
	}
	if cache.Len() != len(items) {
		t.Errorf("cached %d summaries, want %d", cache.Len(), len(items))
	}
}

// TestGenerateBatchSerializesProgressCallback pins the contract existing callers
// rely on: runIndex mutates an error-count map from the callback and
// RunRecorder mutates two flags, neither with a lock. Run under -race, an
// unsynchronized callback fails here.
func TestGenerateBatchSerializesProgressCallback(t *testing.T) {
	cmd := testhelper.WriteMockScript(t,
		"#!/bin/sh\ncat >/dev/null\necho summary\n",
		"$null = [Console]::In.ReadToEnd()\nWrite-Output 'summary'\n",
	)
	gen := NewGenerator(Config{Command: cmd, Concurrency: 4})
	cache := newTestCache(t)

	items := make([]BatchItem, 12)
	for i := range items {
		items[i] = BatchItem{ID: fmt.Sprintf("s%d", i), Text: "text", LastUsed: time.Now()}
	}

	// Deliberately unsynchronized, exactly like the real callers.
	counts := map[string]int{}
	calls := 0
	gen.GenerateBatch(context.Background(), items, cache, func(done, total int, id string, err error) {
		counts["seen"]++
		calls++
		if done != calls {
			t.Errorf("done = %d on call %d, want a monotonic completion count", done, calls)
		}
		if total != len(items) {
			t.Errorf("total = %d, want %d", total, len(items))
		}
	})

	if counts["seen"] != len(items) {
		t.Errorf("callback ran %d times, want %d", counts["seen"], len(items))
	}
}

// TestGenerateBatchRunsInParallel is the point of the change. Serial execution
// of 6 items sleeping 0.4s each takes >=2.4s; three workers should finish in
// roughly a third of that. The threshold is loose so process-spawn overhead on
// a slow runner can't make it flaky.
func TestGenerateBatchRunsInParallel(t *testing.T) {
	cmd := testhelper.WriteMockScript(t,
		"#!/bin/sh\ncat >/dev/null\nsleep 0.4\necho summary\n",
		"$null = [Console]::In.ReadToEnd()\nStart-Sleep -Milliseconds 400\nWrite-Output 'summary'\n",
	)
	gen := NewGenerator(Config{Command: cmd, Concurrency: 3})
	cache := newTestCache(t)

	items := make([]BatchItem, 6)
	for i := range items {
		items[i] = BatchItem{ID: fmt.Sprintf("s%d", i), Text: "text", LastUsed: time.Now()}
	}

	start := time.Now()
	succeeded := gen.GenerateBatch(context.Background(), items, cache, nil)
	elapsed := time.Since(start)

	if succeeded != len(items) {
		t.Fatalf("succeeded = %d, want %d", succeeded, len(items))
	}
	const serial = 6 * 400 * time.Millisecond
	if elapsed >= serial {
		t.Errorf("took %v, which is no better than serial (%v)", elapsed, serial)
	}
}

func TestGenerateBatchEmpty(t *testing.T) {
	gen := NewGenerator(Config{Command: []string{"false"}})
	if got := gen.GenerateBatch(context.Background(), nil, newTestCache(t), nil); got != 0 {
		t.Errorf("succeeded = %d, want 0 for an empty batch", got)
	}
}

// TestGenerateBatchStopsOnCancelledContext guards the dispatch loop: workers
// stop draining once the context is done, so a naive send would block forever.
func TestGenerateBatchStopsOnCancelledContext(t *testing.T) {
	cmd := testhelper.WriteMockScript(t,
		"#!/bin/sh\ncat >/dev/null\necho summary\n",
		"$null = [Console]::In.ReadToEnd()\nWrite-Output 'summary'\n",
	)
	gen := NewGenerator(Config{Command: cmd, Concurrency: 2})

	items := make([]BatchItem, 50)
	for i := range items {
		items[i] = BatchItem{ID: fmt.Sprintf("s%d", i), Text: "text", LastUsed: time.Now()}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan int, 1)
	go func() { done <- gen.GenerateBatch(ctx, items, newTestCache(t), nil) }()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("GenerateBatch did not return on a cancelled context")
	}
}

func TestGenerateBatchDefaultsConcurrency(t *testing.T) {
	// A zero or negative value must fall back rather than deadlock on zero
	// workers with a blocking dispatch.
	cmd := testhelper.WriteMockScript(t,
		"#!/bin/sh\ncat >/dev/null\necho summary\n",
		"$null = [Console]::In.ReadToEnd()\nWrite-Output 'summary'\n",
	)
	for _, n := range []int{0, -1} {
		gen := NewGenerator(Config{Command: cmd, Concurrency: n})
		items := []BatchItem{{ID: "a", Text: "t", LastUsed: time.Now()}}

		done := make(chan int, 1)
		go func() { done <- gen.GenerateBatch(context.Background(), items, newTestCache(t), nil) }()

		select {
		case got := <-done:
			if got != 1 {
				t.Errorf("concurrency %d: succeeded = %d, want 1", n, got)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("concurrency %d: deadlocked", n)
		}
	}
}
