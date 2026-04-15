package update

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{"newer patch", "0.5.0", "0.5.1", true},
		{"newer minor", "0.5.0", "0.6.0", true},
		{"newer major", "0.5.0", "1.0.0", true},
		{"same", "0.5.0", "0.5.0", false},
		{"older", "0.6.0", "0.5.0", false},
		{"with v prefix", "v0.5.0", "v0.6.0", true},
		{"mixed prefix", "0.5.0", "v0.6.0", true},
		{"dev build", "dev", "0.6.0", false},
		{"empty current", "", "0.6.0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNewer(tt.current, tt.latest)
			if got != tt.want {
				t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		a, b string
		want int // >0, <0, or 0
	}{
		{"1.0.0", "0.9.0", 1},
		{"0.5.0", "0.5.0", 0},
		{"0.5.0", "0.5.1", -1},
		{"1.0.0", "0.99.99", 1},
		{"0.10.0", "0.9.0", 1},
	}
	for _, tt := range tests {
		t.Run(tt.a+" vs "+tt.b, func(t *testing.T) {
			got := compareSemver(tt.a, tt.b)
			if (tt.want > 0 && got <= 0) || (tt.want < 0 && got >= 0) || (tt.want == 0 && got != 0) {
				t.Errorf("compareSemver(%q, %q) = %d, want sign %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestAssetName(t *testing.T) {
	name := AssetName("v0.5.0")
	// Just verify it contains the version and has an extension.
	if name == "" {
		t.Error("AssetName returned empty")
	}
	if !contains(name, "0.5.0") {
		t.Errorf("AssetName should contain version, got %q", name)
	}
	if !contains(name, ".tar.gz") && !contains(name, ".zip") {
		t.Errorf("AssetName should have archive extension, got %q", name)
	}
}

func TestFindAsset(t *testing.T) {
	release := &Release{
		TagName: "v0.5.0",
		Assets: []Asset{
			{Name: "sesh_0.5.0_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux"},
			{Name: "sesh_0.5.0_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/darwin"},
		},
	}

	url, err := FindAsset(release)
	// This will match based on runtime.GOOS/GOARCH — just verify it doesn't error
	// on the test platform.
	if err != nil {
		// Might not find an asset if test platform isn't in the list.
		t.Logf("FindAsset: %v (expected on some platforms)", err)
		return
	}
	if url == "" {
		t.Error("FindAsset returned empty URL")
	}
}

func TestFindAssetMissing(t *testing.T) {
	release := &Release{
		TagName: "v0.5.0",
		Assets:  []Asset{},
	}
	_, err := FindAsset(release)
	if err == nil {
		t.Error("expected error for empty assets")
	}
}

func TestCacheRoundtrip(t *testing.T) {
	// Override cache path via XDG.
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	// No cache initially.
	if vc := CheckCached(); vc != nil {
		t.Error("expected nil for empty cache")
	}

	// Save and read back.
	SaveCache("0.6.0")
	vc := CheckCached()
	if vc == nil {
		t.Fatal("expected cached value")
	}
	if vc.Latest != "0.6.0" {
		t.Errorf("cached latest = %q, want 0.6.0", vc.Latest)
	}
}

func TestCacheExpiry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	// Write a cache entry that's 25 hours old.
	vc := VersionCheck{
		Latest:    "0.6.0",
		CheckedAt: time.Now().Add(-25 * time.Hour),
	}
	data, _ := json.Marshal(vc)
	cacheDir := filepath.Join(dir, "sesh")
	os.MkdirAll(cacheDir, 0755)
	os.WriteFile(filepath.Join(cacheDir, "version-check.json"), data, 0644)

	if got := CheckCached(); got != nil {
		t.Error("expected nil for expired cache")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- verifyChecksum tests ---

func TestVerifyChecksumValid(t *testing.T) {
	// Create a fake archive file.
	content := []byte("fake archive content for testing")
	archivePath := filepath.Join(t.TempDir(), "sesh_1.0.0_darwin_arm64.tar.gz")
	if err := os.WriteFile(archivePath, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Compute its SHA256.
	h := sha256.Sum256(content)
	hash := hex.EncodeToString(h[:])

	// Serve a checksums.txt with the correct hash.
	checksums := fmt.Sprintf("%s  sesh_1.0.0_darwin_arm64.tar.gz\n%s  sesh_1.0.0_linux_amd64.tar.gz\n",
		hash, "aaaa")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(checksums))
	}))
	defer srv.Close()

	release := &Release{
		Assets: []Asset{
			{Name: "checksums.txt", BrowserDownloadURL: srv.URL + "/checksums.txt"},
		},
	}

	err := verifyChecksum(release, archivePath, "sesh_1.0.0_darwin_arm64.tar.gz")
	if err != nil {
		t.Errorf("expected valid checksum, got error: %v", err)
	}
}

func TestVerifyChecksumMismatch(t *testing.T) {
	content := []byte("fake archive content")
	archivePath := filepath.Join(t.TempDir(), "sesh_1.0.0_darwin_arm64.tar.gz")
	if err := os.WriteFile(archivePath, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Serve a checksums.txt with a wrong hash.
	checksums := "deadbeef00000000000000000000000000000000000000000000000000000000  sesh_1.0.0_darwin_arm64.tar.gz\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(checksums))
	}))
	defer srv.Close()

	release := &Release{
		Assets: []Asset{
			{Name: "checksums.txt", BrowserDownloadURL: srv.URL + "/checksums.txt"},
		},
	}

	err := verifyChecksum(release, archivePath, "sesh_1.0.0_darwin_arm64.tar.gz")
	if err == nil {
		t.Error("expected checksum mismatch error, got nil")
	}
	if err != nil && !containsStr(err.Error(), "checksum mismatch") {
		t.Errorf("expected 'checksum mismatch' in error, got: %v", err)
	}
}

func TestVerifyChecksumNoChecksumsAsset(t *testing.T) {
	// Release has no checksums.txt — verification should be skipped.
	release := &Release{
		Assets: []Asset{
			{Name: "sesh_1.0.0_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/archive"},
		},
	}

	err := verifyChecksum(release, "/nonexistent", "sesh_1.0.0_darwin_arm64.tar.gz")
	if err != nil {
		t.Errorf("expected nil (skip verification), got: %v", err)
	}
}

func TestVerifyChecksumArchiveNotInChecksums(t *testing.T) {
	// checksums.txt exists but doesn't contain our archive name.
	checksums := "abcdef1234567890  sesh_1.0.0_linux_amd64.tar.gz\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(checksums))
	}))
	defer srv.Close()

	release := &Release{
		Assets: []Asset{
			{Name: "checksums.txt", BrowserDownloadURL: srv.URL + "/checksums.txt"},
		},
	}

	err := verifyChecksum(release, "/nonexistent", "sesh_1.0.0_darwin_arm64.tar.gz")
	if err == nil {
		t.Error("expected error for missing archive in checksums, got nil")
	}
	if err != nil && !containsStr(err.Error(), "not found in checksums.txt") {
		t.Errorf("expected 'not found in checksums.txt' in error, got: %v", err)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}())
}
