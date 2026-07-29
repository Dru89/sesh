// Package paths resolves the on-disk locations sesh reads and writes.
//
// It exists so every caller agrees. When two packages each computed the cache
// directory independently they could disagree, and one of them creating its
// directory changed the answer the other got on the next run — see Cache.
package paths

import (
	"os"
	"path/filepath"
	"runtime"
)

// Cache returns the directory sesh stores its cache files in: the summary
// cache, the index health record, and the update version check.
//
// Resolution order:
//
//  1. $XDG_CACHE_HOME/sesh, honored unconditionally when set. An explicit
//     setting beats a heuristic and must not depend on whether a directory
//     happens to exist.
//  2. ~/.cache/sesh.
//  3. On Windows only, %LOCALAPPDATA%\sesh when ~/.cache does not exist —
//     Unix-style dotfile cache directories are not the norm there.
//
// Step 3 probes the filesystem, which makes it stable only while every caller
// resolves the path the same way. It previously was not: the update checker
// computed its own path without the Windows branch and called MkdirAll on
// ~/.cache/sesh, creating the very directory this function probes for. A
// Windows user's summary cache was written to %LOCALAPPDATA% on the first run
// and then read from ~/.cache on the second, so every summary silently
// disappeared and had to be regenerated. Routing all callers through here keeps
// the answer stable: if the Windows branch wins, nothing creates the directory
// that would flip it.
func Cache() string {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "sesh")
	}

	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".cache", "sesh")

	if runtime.GOOS == "windows" {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
				dir = filepath.Join(localAppData, "sesh")
			}
		}
	}
	return dir
}
