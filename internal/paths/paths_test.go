package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheHonorsXDGUnconditionally(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("LOCALAPPDATA", filepath.Join(dir, "should-not-be-used"))

	// Deliberately do not create the sesh subdirectory: an explicit setting
	// must not depend on whether a directory happens to exist yet.
	want := filepath.Join(dir, "sesh")
	if got := Cache(); got != want {
		t.Errorf("Cache() = %q, want %q", got, want)
	}
}

// TestCacheIsStableAcrossDirectoryCreation is the regression this package
// exists for. The Windows branch probes the filesystem, so the answer must not
// change once the directory it probes for is created — otherwise a cache
// written on one run is looked for somewhere else on the next, and every
// summary silently disappears.
//
// Creating the directory is what the update checker's MkdirAll used to do
// behind the summary cache's back.
func TestCacheIsStableAcrossDirectoryCreation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	before := Cache()
	if err := os.MkdirAll(before, 0755); err != nil {
		t.Fatal(err)
	}
	after := Cache()

	if before != after {
		t.Errorf("Cache() moved from %q to %q once the directory existed", before, after)
	}
}

func TestCacheIsASingleDirectoryForAllCallers(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	// Every cache file sesh writes must resolve to the same parent, so that
	// no caller's MkdirAll can change another caller's answer.
	summaries := filepath.Join(Cache(), "summaries.json")
	health := filepath.Join(Cache(), "index-health.json")
	version := filepath.Join(Cache(), "version-check.json")

	for _, p := range []string{health, version} {
		if filepath.Dir(p) != filepath.Dir(summaries) {
			t.Errorf("%q is not beside %q", p, summaries)
		}
	}
}
