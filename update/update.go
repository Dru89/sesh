package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	repoOwner = "dru89"
	repoName  = "sesh"
	apiURL    = "https://api.github.com/repos/" + repoOwner + "/" + repoName + "/releases/latest"
)

// Release represents a GitHub release.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
	HTMLURL string  `json:"html_url"`
}

// Asset is a downloadable file in a release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// VersionCheck holds the cached result of a version check.
type VersionCheck struct {
	Latest    string    `json:"latest"`
	CheckedAt time.Time `json:"checked_at"`
}

// CheckLatest queries GitHub for the latest release version.
func CheckLatest() (*Release, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("check latest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("github API returned %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parse release: %w", err)
	}

	return &release, nil
}

// CheckCached returns the cached version check, or nil if stale/missing.
// Cache is valid for 24 hours.
func CheckCached() *VersionCheck {
	path := cachePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var vc VersionCheck
	if err := json.Unmarshal(data, &vc); err != nil {
		return nil
	}
	if time.Since(vc.CheckedAt) > 24*time.Hour {
		return nil
	}
	return &vc
}

// SaveCache stores the latest version check result.
func SaveCache(latest string) {
	vc := VersionCheck{
		Latest:    latest,
		CheckedAt: time.Now(),
	}
	data, err := json.Marshal(vc)
	if err != nil {
		return
	}
	dir := filepath.Dir(cachePath())
	os.MkdirAll(dir, 0755)
	os.WriteFile(cachePath(), data, 0644)
}

// IsNewer returns true if latest is a newer version than current.
// Both should be semver strings like "0.5.0" (no "v" prefix).
func IsNewer(current, latest string) bool {
	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")
	if current == "" || current == "dev" {
		return false // dev build, don't nag
	}
	return latest != current && compareSemver(latest, current) > 0
}

// AssetName returns the expected archive filename for the current platform.
func AssetName(version string) string {
	version = strings.TrimPrefix(version, "v")
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("sesh_%s_%s_%s.%s", version, runtime.GOOS, runtime.GOARCH, ext)
}

// FindAsset finds the download URL for the current platform in a release.
func FindAsset(release *Release) (string, error) {
	want := AssetName(release.TagName)
	for _, a := range release.Assets {
		if a.Name == want {
			return a.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("no release asset found for %s/%s (looking for %s)", runtime.GOOS, runtime.GOARCH, want)
}

// DownloadAndReplace downloads the release archive, extracts the binary,
// and replaces the currently running binary.
func DownloadAndReplace(url string) error {
	// Download the archive.
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	// Determine current binary path.
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	// Extract the sesh binary from the archive.
	var binaryData []byte
	if strings.HasSuffix(url, ".zip") {
		binaryData, err = extractFromZip(resp.Body)
	} else {
		binaryData, err = extractFromTarGz(resp.Body)
	}
	if err != nil {
		return err
	}

	// Write to a temp file next to the current binary, then rename.
	dir := filepath.Dir(execPath)
	tmp, err := os.CreateTemp(dir, "sesh-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(binaryData); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	tmp.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod: %w", err)
	}

	// Atomic rename.
	if err := os.Rename(tmpPath, execPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replace binary: %w", err)
	}

	return nil
}

func extractFromTarGz(r io.Reader) ([]byte, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar: %w", err)
		}
		name := filepath.Base(hdr.Name)
		if name == "sesh" || name == "sesh.exe" {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("read binary: %w", err)
			}
			return data, nil
		}
	}
	return nil, fmt.Errorf("sesh binary not found in archive")
}

func extractFromZip(r io.Reader) ([]byte, error) {
	// zip needs random access, so buffer to a temp file.
	tmp, err := os.CreateTemp("", "sesh-download-*.zip")
	if err != nil {
		return nil, fmt.Errorf("create temp: %w", err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if _, err := io.Copy(tmp, r); err != nil {
		return nil, fmt.Errorf("download to temp: %w", err)
	}

	stat, err := tmp.Stat()
	if err != nil {
		return nil, err
	}

	zr, err := zip.NewReader(tmp, stat.Size())
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	for _, f := range zr.File {
		name := filepath.Base(f.Name)
		if name == "sesh" || name == "sesh.exe" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("sesh binary not found in zip")
}

func cachePath() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".cache", "sesh")
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		dir = filepath.Join(xdg, "sesh")
	}
	return filepath.Join(dir, "version-check.json")
}

// compareSemver compares two semver strings (without v prefix).
// Returns >0 if a > b, <0 if a < b, 0 if equal.
func compareSemver(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		var av, bv int
		if i < len(aParts) {
			fmt.Sscanf(aParts[i], "%d", &av)
		}
		if i < len(bParts) {
			fmt.Sscanf(bParts[i], "%d", &bv)
		}
		if av != bv {
			return av - bv
		}
	}
	return 0
}
