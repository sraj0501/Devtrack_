package main

import (
	"archive/tar"
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

const githubRepo = "sraj0501/automation_tools"

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Name    string        `json:"name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// RunUpgrade implements `devtrack upgrade [--check]`.
func RunUpgrade(checkOnly bool) error {
	fmt.Println("Checking for updates...")

	latest, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("could not reach GitHub: %w", err)
	}

	current := GetDevTrackVersion()
	if current == "dev" {
		fmt.Printf("  Current version: dev build (not a release)\n")
		fmt.Printf("  Latest release:  %s\n", latest.TagName)
		if !checkOnly {
			fmt.Println("  Tip: upgrade only applies to installed release binaries.")
		}
		return nil
	}

	fmt.Printf("  Current version: %s\n", current)
	fmt.Printf("  Latest release:  %s\n", latest.TagName)

	if normaliseTag(current) == normaliseTag(latest.TagName) {
		fmt.Println("  Already up to date.")
		return nil
	}

	fmt.Printf("  Update available: %s → %s\n", current, latest.TagName)

	if checkOnly {
		fmt.Println("\nRun 'devtrack upgrade' to install the update.")
		return nil
	}

	// Find the asset for this platform
	assetName := fmt.Sprintf("devtrack_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	var downloadURL string
	for _, a := range latest.Assets {
		if a.Name == assetName {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no binary found for %s/%s in release %s", runtime.GOOS, runtime.GOARCH, latest.TagName)
	}

	fmt.Printf("\nDownloading %s...\n", assetName)
	newBinary, err := downloadBinaryFromAsset(downloadURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer os.Remove(newBinary)

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate current binary: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("could not resolve binary path: %w", err)
	}

	fmt.Printf("Installing to %s...\n", execPath)
	if err := replaceBinary(execPath, newBinary); err != nil {
		return fmt.Errorf("install failed: %w", err)
	}

	fmt.Printf("✓ Updated to %s\n", latest.TagName)
	fmt.Println("\nApplying configuration migrations...")
	RunPendingMigrations()
	fmt.Println("Done. Run 'devtrack start' to use the new version.")
	return nil
}

// fetchLatestRelease queries the GitHub releases API.
func fetchLatestRelease() (*githubRelease, error) {
	url := "https://api.github.com/repos/" + githubRepo + "/releases/latest"
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "devtrack-upgrade/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &rel, nil
}

// downloadBinaryFromAsset downloads a .tar.gz asset, extracts the `devtrack`
// binary from it, writes it to a temp file, and returns the temp path.
func downloadBinaryFromAsset(url string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "devtrack-upgrade/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download returned %d", resp.StatusCode)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("tar: %w", err)
		}
		// The binary inside the archive is named "devtrack"
		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != "devtrack" {
			continue
		}

		tmp, err := os.CreateTemp("", "devtrack-upgrade-*")
		if err != nil {
			return "", fmt.Errorf("temp file: %w", err)
		}
		if _, err := io.Copy(tmp, tr); err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return "", fmt.Errorf("extract: %w", err)
		}
		tmp.Close()
		if err := os.Chmod(tmp.Name(), 0755); err != nil {
			os.Remove(tmp.Name())
			return "", err
		}
		return tmp.Name(), nil
	}
	return "", fmt.Errorf("devtrack binary not found in archive")
}

// replaceBinary atomically replaces dst with src using rename.
// On the same filesystem this is atomic.  Falls back to copy+rename
// when src and dst are on different filesystems.
func replaceBinary(dst, src string) error {
	// Try atomic rename first (same filesystem)
	tmpDst := dst + ".upgrade"
	if err := copyFile(src, tmpDst); err != nil {
		return fmt.Errorf("stage new binary: %w", err)
	}
	if err := os.Chmod(tmpDst, 0755); err != nil {
		os.Remove(tmpDst)
		return err
	}
	if err := os.Rename(tmpDst, dst); err != nil {
		os.Remove(tmpDst)
		return fmt.Errorf("replace binary: %w", err)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// normaliseTag strips a leading 'v' for comparison.
func normaliseTag(tag string) string {
	return strings.TrimPrefix(tag, "v")
}
