package summary

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCachePutAndGet(t *testing.T) {
	c := newTestCache(t)

	now := time.Now()
	c.Put("ses_1", "Built auth middleware", now)

	got, ok := c.Get("ses_1", now)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got != "Built auth middleware" {
		t.Errorf("got %q, want %q", got, "Built auth middleware")
	}
}

func TestCacheMiss(t *testing.T) {
	c := newTestCache(t)

	_, ok := c.Get("nonexistent", time.Now())
	if ok {
		t.Error("expected cache miss for nonexistent key")
	}
}

// TestCacheGetIgnoresStaleness verifies that Get returns a cached summary for
// display even when the session has been used since the summary was generated.
// Staleness is a regeneration concern handled by NeedsSummary; showing a stale
// summary beats falling back to the raw (often multi-line) first prompt.
func TestCacheGetIgnoresStaleness(t *testing.T) {
	c := newTestCache(t)

	oldTime := time.Now().Add(-2 * time.Hour)

	// Stale by NeedsSummary's rule: last_used advanced and generation is old.
	c.entries["ses_1"] = Entry{
		Summary:   "Old summary",
		LastUsed:  oldTime,
		Generated: time.Now().Add(-2 * time.Hour),
	}

	// Get still returns the summary for display.
	got, ok := c.Get("ses_1", time.Now())
	if !ok {
		t.Fatal("expected Get to return a stale summary for display")
	}
	if got != "Old summary" {
		t.Errorf("got %q, want %q", got, "Old summary")
	}

	// NeedsSummary still flags it for regeneration.
	need := c.NeedsSummary([]SessionRef{{ID: "ses_1", LastUsed: time.Now()}})
	if len(need) != 1 {
		t.Errorf("expected stale entry to be flagged for regeneration, got %d", len(need))
	}
}

func TestCacheNeedsSummary(t *testing.T) {
	c := newTestCache(t)

	now := time.Now()
	past := now.Add(-2 * time.Hour)

	c.Put("ses_1", "Summary 1", past)
	// Make ses_1's generation old enough to be stale if last_used changes.
	c.entries["ses_1"] = Entry{
		Summary:   "Summary 1",
		LastUsed:  past,
		Generated: now.Add(-2 * time.Hour),
	}

	refs := []SessionRef{
		{ID: "ses_1", LastUsed: now},  // stale: last_used changed + old generation
		{ID: "ses_2", LastUsed: now},  // missing
		{ID: "ses_3", LastUsed: past}, // missing
	}

	need := c.NeedsSummary(refs)
	if len(need) != 3 {
		t.Errorf("expected 3, got %d", len(need))
	}
}

func TestCacheNeedsSummaryAllFresh(t *testing.T) {
	c := newTestCache(t)

	now := time.Now()
	c.Put("ses_1", "Summary 1", now)
	c.Put("ses_2", "Summary 2", now)

	refs := []SessionRef{
		{ID: "ses_1", LastUsed: now},
		{ID: "ses_2", LastUsed: now},
	}

	need := c.NeedsSummary(refs)
	if len(need) != 0 {
		t.Errorf("expected 0, got %d", len(need))
	}
}

func TestCacheSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "summaries.json")

	// Create and populate a cache.
	c := &Cache{
		path:    path,
		entries: make(map[string]Entry),
	}
	now := time.Now().Truncate(time.Millisecond) // JSON loses sub-ms precision
	c.Put("ses_1", "Built auth middleware", now)
	c.Put("ses_2", "Fixed login bug", now)

	if err := c.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("cache file not created")
	}

	// Load into a new cache.
	c2 := &Cache{
		path:    path,
		entries: make(map[string]Entry),
	}
	c2.load()

	if c2.Len() != 2 {
		t.Errorf("loaded %d entries, want 2", c2.Len())
	}

	got, ok := c2.Get("ses_1", now)
	if !ok {
		t.Fatal("expected cache hit after load")
	}
	if got != "Built auth middleware" {
		t.Errorf("got %q, want %q", got, "Built auth middleware")
	}
}

func TestCacheLen(t *testing.T) {
	c := newTestCache(t)
	if c.Len() != 0 {
		t.Errorf("empty cache Len() = %d, want 0", c.Len())
	}

	c.Put("a", "x", time.Now())
	c.Put("b", "y", time.Now())
	if c.Len() != 2 {
		t.Errorf("Len() = %d, want 2", c.Len())
	}
}

func TestCacheClear(t *testing.T) {
	c := newTestCache(t)
	c.Put("a", "x", time.Now())
	c.Put("b", "y", time.Now())
	if c.Len() != 2 {
		t.Fatalf("expected 2, got %d", c.Len())
	}

	c.Clear()
	if c.Len() != 0 {
		t.Errorf("after Clear(), Len() = %d, want 0", c.Len())
	}

	// Should be able to put new entries after clear.
	c.Put("c", "z", time.Now())
	if c.Len() != 1 {
		t.Errorf("after re-add, Len() = %d, want 1", c.Len())
	}
}

func newTestCache(t *testing.T) *Cache {
	t.Helper()
	dir := t.TempDir()
	return &Cache{
		path:    filepath.Join(dir, "summaries.json"),
		entries: make(map[string]Entry),
	}
}

// TestCacheDirHonorsXDGUnconditionally guards a Windows-specific regression:
// the %LOCALAPPDATA% fallback used to run after XDG_CACHE_HOME was applied and
// override it whenever the directory did not exist yet. That made an explicit
// setting depend on whether a directory happened to have been created, and
// silently relocated the cache the first time it was.
func TestCacheDirHonorsXDGUnconditionally(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("LOCALAPPDATA", filepath.Join(dir, "should-not-be-used"))

	// Deliberately do not create the sesh subdirectory.
	want := filepath.Join(dir, "sesh")
	if got := CacheDir(); got != want {
		t.Errorf("CacheDir() = %q, want %q", got, want)
	}
}
