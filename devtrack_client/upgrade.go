package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const githubRepo = "sraj0501/Devtrack_"
const githubAPIBase = "https://api.github.com"
const updateChannelFile = "update-channel"

type updateChannel string

const (
	updateChannelMain updateChannel = "main"
	updateChannelDev  updateChannel = "dev"
)

type githubRelease struct {
	TagName    string        `json:"tag_name"`
	Name       string        `json:"name"`
	Prerelease bool          `json:"prerelease"`
	Assets     []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// parseUpgradeArgs accepts flags in any order. An empty channel means to use
// the persisted selection (main for installations that predate channels).
func parseUpgradeArgs(args []string) (bool, updateChannel, error) {
	var checkOnly bool
	var channel updateChannel
	for _, arg := range args {
		switch arg {
		case "--check":
			checkOnly = true
		case "--dev":
			if channel != "" && channel != updateChannelDev {
				return false, "", fmt.Errorf("--dev and --main cannot be used together")
			}
			channel = updateChannelDev
		case "--main", "--stable":
			if channel != "" && channel != updateChannelMain {
				return false, "", fmt.Errorf("--dev and --main cannot be used together")
			}
			channel = updateChannelMain
		default:
			return false, "", fmt.Errorf("unknown upgrade option %q (use --check, --dev, or --main)", arg)
		}
	}
	return checkOnly, channel, nil
}

// RunUpgrade preserves the original package-level API and follows the saved channel.
func RunUpgrade(checkOnly bool) error {
	return RunUpgradeForChannel(checkOnly, "")
}

// RunUpgradeForChannel implements `devtrack upgrade [--check] [--dev|--main]`.
func RunUpgradeForChannel(checkOnly bool, requested updateChannel) error {
	fmt.Println("Checking for updates...")

	home, homeErr := devtrackDataHome()
	channel := requested
	if channel == "" {
		channel = readUpdateChannel(home)
	}
	fmt.Printf("  Update channel:  %s\n", channel)

	latest, err := fetchRelease(channel)
	if err != nil {
		return fmt.Errorf("could not reach GitHub: %w", err)
	}

	current := GetDevTrackVersion()
	// A source build has no immutable version identity. Keep the historical
	// safeguard unless the user explicitly asks to replace it with a channel build.
	if current == "dev" && requested == "" {
		fmt.Printf("  Current version: dev build (not a release)\n")
		fmt.Printf("  Latest %s build: %s\n", channel, releaseVersion(latest, channel))
		if !checkOnly {
			fmt.Printf("  Tip: use 'devtrack upgrade --%s' to replace this source build.\n", channel)
		}
		return nil
	}

	target := releaseVersion(latest, channel)
	fmt.Printf("  Current version: %s\n", current)
	fmt.Printf("  Latest %s build: %s\n", channel, target)

	if isCurrentVersion(current, target, channel) {
		fmt.Println("  Already up to date.")
		if !checkOnly && requested != "" {
			persistUpdateChannel(home, channel)
		}
		return nil
	}
	if channel == updateChannelMain && !strings.HasPrefix(current, "dev-") && current != "dev" &&
		compareSemver(normaliseTag(current), normaliseTag(target)) > 0 {
		fmt.Println("  Already up to date (current build is newer than latest release).")
		if !checkOnly && requested != "" {
			persistUpdateChannel(home, channel)
		}
		return nil
	}

	fmt.Printf("  Update available: %s → %s\n", current, target)

	if checkOnly {
		flag := ""
		if requested != "" {
			flag = " --" + string(channel)
		}
		fmt.Printf("\nRun 'devtrack upgrade%s' to install the update.\n", flag)
		return nil
	}

	// Locate the asset for this platform
	assetName := platformAssetName()
	var downloadURL string
	for _, asset := range latest.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no binary found for %s/%s in release %s", runtime.GOOS, runtime.GOARCH, latest.TagName)
	}

	fmt.Printf("\nDownloading %s...\n", assetName)
	newBinary, err := downloadBinary(downloadURL, assetName)
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

	fmt.Printf("✓ Updated to %s\n", target)
	if homeErr != nil {
		fmt.Printf("Warning: could not save the %s update channel: %v\n", channel, homeErr)
	} else {
		persistUpdateChannel(home, channel)
	}
	fmt.Println("\nApplying configuration migrations...")
	RunPendingMigrations()

	fmt.Println()
	// Keep the optional server update outside the binary-upgrade critical path.
	// The same durable worker used by setup reports progress through doctor/status.
	if home == "" || homeErr != nil {
		fmt.Printf("Warning: could not determine DevTrack data home: %v\n", homeErr)
	} else if GetServerMode() == ServerModeManaged {
		projectRoot := GetProjectRootOptional()
		if projectRoot == "" {
			projectRoot = filepath.Join(home, "server", "devtrack_server")
		}
		if started, startErr := startServerBootstrap(home, projectRoot, GetLLMProvider(), GetOllamaModel()); startErr != nil {
			fmt.Printf("Warning: background Python server update could not start: %v\n", startErr)
			fmt.Println("The binary was upgraded successfully. Run 'devtrack doctor --repair' to retry.")
		} else if started {
			fmt.Println("Optional Python server update started in the background (check 'devtrack doctor').")
		}
	}

	if isDaemonRunning() {
		fmt.Println("\nRestarting daemon with new binary...")
		restart := exec.Command(execPath, "restart")
		restart.Stdin = os.Stdin
		restart.Stdout = os.Stdout
		restart.Stderr = os.Stderr
		if err := restart.Run(); err != nil {
			fmt.Printf("  Warning: restart failed: %v\n", err)
			fmt.Println("  Run 'devtrack restart' manually.")
		}
	} else {
		fmt.Println("\nDone. Run 'devtrack start' to use the new version.")
	}
	return nil
}

func releaseVersion(release *githubRelease, channel updateChannel) string {
	if channel == updateChannelDev && strings.TrimSpace(release.Name) != "" {
		return strings.TrimSpace(release.Name)
	}
	return release.TagName
}

func isCurrentVersion(current, target string, channel updateChannel) bool {
	if channel == updateChannelDev {
		return current == target
	}
	return compareSemver(normaliseTag(current), normaliseTag(target)) == 0
}

func readUpdateChannel(home string) updateChannel {
	if home == "" {
		return updateChannelMain
	}
	data, err := os.ReadFile(filepath.Join(home, updateChannelFile))
	if err == nil && strings.TrimSpace(string(data)) == string(updateChannelDev) {
		return updateChannelDev
	}
	return updateChannelMain
}

func persistUpdateChannel(home string, channel updateChannel) {
	if home == "" {
		return
	}
	if err := os.MkdirAll(home, 0755); err != nil {
		fmt.Printf("Warning: could not save update channel: %v\n", err)
		return
	}
	path := filepath.Join(home, updateChannelFile)
	tmp, err := os.CreateTemp(home, ".update-channel-*.tmp")
	if err != nil {
		fmt.Printf("Warning: could not save update channel: %v\n", err)
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err = fmt.Fprintln(tmp, channel); err == nil {
		err = tmp.Close()
	} else {
		_ = tmp.Close()
	}
	if err == nil {
		err = atomicReplaceFile(tmpPath, path)
	}
	if err != nil {
		fmt.Printf("Warning: could not save update channel: %v\n", err)
	}
}

// platformAssetName returns the asset filename for the current OS/arch.
// Windows downloads the .exe directly to avoid zip extraction issues.
func platformAssetName() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	if goos == "windows" {
		return fmt.Sprintf("devtrack_%s_%s.exe", goos, goarch)
	}
	return fmt.Sprintf("devtrack_%s_%s.tar.gz", goos, goarch)
}

// isDaemonRunning returns true if the PID file exists and the process is alive.
// The liveness probe is platform-specific — see upgrade_unix.go / upgrade_windows.go.
func isDaemonRunning() bool {
	data, err := os.ReadFile(GetPIDFilePath())
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return false
	}
	return isProcessAlive(pid)
}

// fetchRelease queries the stable release endpoint or the rolling dev prerelease.
func fetchRelease(channel updateChannel) (*githubRelease, error) {
	path := "/repos/" + githubRepo + "/releases/latest"
	if channel == updateChannelDev {
		path = "/repos/" + githubRepo + "/releases/tags/dev"
	}
	return fetchReleaseAt(githubAPIBase+path, channel, &http.Client{Timeout: 15 * time.Second})
}

func fetchReleaseAt(url string, channel updateChannel, client *http.Client) (*githubRelease, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "devtrack-upgrade/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if release.TagName == "" {
		return nil, fmt.Errorf("no releases found")
	}
	if channel == updateChannelDev && (!release.Prerelease || release.TagName != "dev") {
		return nil, fmt.Errorf("development release is not a dev prerelease")
	}
	return &release, nil
}

// compareSemver compares two semver strings (without leading 'v').
// Returns -1 if a < b, 0 if equal, +1 if a > b.
func compareSemver(a, b string) int {
	pa := parseSemver(a)
	pb := parseSemver(b)
	for i := range 3 {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

func parseSemver(v string) [3]int {
	// Strip any pre-release suffix (e.g. "1.2.3-beta" → "1.2.3")
	v = strings.SplitN(v, "-", 2)[0]
	parts := strings.SplitN(v, ".", 3)
	var out [3]int
	for i, p := range parts {
		if i >= 3 {
			break
		}
		out[i], _ = strconv.Atoi(p)
	}
	return out
}

// downloadBinary fetches the archive at url, extracts the devtrack binary,
// writes it to a temp file, and returns the temp path.
func downloadBinary(url, assetName string) (string, error) {
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	if strings.HasSuffix(assetName, ".exe") {
		return writeTempBinary(bytes.NewReader(body))
	}
	if strings.HasSuffix(assetName, ".zip") {
		return extractFromZip(body)
	}
	return extractFromTarGz(bytes.NewReader(body))
}

// extractFromTarGz pulls the devtrack binary out of a .tar.gz archive.
func extractFromTarGz(r io.Reader) (string, error) {
	gz, err := gzip.NewReader(r)
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
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		base := filepath.Base(hdr.Name)
		if base != "devtrack" && base != "devtrack.exe" {
			continue
		}
		return writeTempBinary(tr)
	}
	return "", fmt.Errorf("devtrack binary not found in archive")
}

// extractFromZip pulls the devtrack binary out of a .zip archive.
func extractFromZip(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("zip: %w", err)
	}
	for _, f := range zr.File {
		base := filepath.Base(f.Name)
		if base != "devtrack" && base != "devtrack.exe" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("zip open: %w", err)
		}
		defer rc.Close()
		return writeTempBinary(rc)
	}
	return "", fmt.Errorf("devtrack binary not found in zip archive")
}

// writeTempBinary copies r into a temp file with executable permissions.
func writeTempBinary(r io.Reader) (string, error) {
	tmp, err := os.CreateTemp("", "devtrack-upgrade-*")
	if err != nil {
		return "", fmt.Errorf("temp file: %w", err)
	}
	if _, err := io.Copy(tmp, r); err != nil {
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

// replaceBinary replaces dst with src.
// It first tries a direct copy+rename (works when running as root or the
// binary is in a user-writable location).  If that fails with a permission
// error it retries via "sudo cp", which prompts the user for their password
// exactly as "sudo" normally would.
func replaceBinary(dst, src string) error {
	tmpDst := dst + ".upgrade"

	err := copyFile(src, tmpDst)
	if err == nil {
		_ = os.Chmod(tmpDst, 0755)
		if renameErr := os.Rename(tmpDst, dst); renameErr == nil {
			return nil
		}
		os.Remove(tmpDst)
	}

	if err != nil && !errors.Is(err, os.ErrPermission) {
		return fmt.Errorf("stage new binary: %w", err)
	}

	// Permission denied — delegate to the platform-specific elevated replace
	// (sudo on Unix, guidance message on Windows).
	return elevatedReplace(dst, src)
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
