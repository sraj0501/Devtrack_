package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestParseUpgradeArgs(t *testing.T) {
	check, channel, err := parseUpgradeArgs([]string{"--dev", "--check"})
	if err != nil || !check || channel != updateChannelDev {
		t.Fatalf("parseUpgradeArgs() = %v, %q, %v", check, channel, err)
	}

	_, channel, err = parseUpgradeArgs([]string{"--stable"})
	if err != nil || channel != updateChannelMain {
		t.Fatalf("--stable channel = %q, %v", channel, err)
	}

	if _, _, err := parseUpgradeArgs([]string{"--dev", "--main"}); err == nil {
		t.Fatal("expected conflicting channels to fail")
	}
	if _, _, err := parseUpgradeArgs([]string{"--unknown"}); err == nil {
		t.Fatal("expected unknown option to fail")
	}
}

func TestUpdateChannelDefaultsAndPersists(t *testing.T) {
	home := t.TempDir()
	if got := readUpdateChannel(home); got != updateChannelMain {
		t.Fatalf("default channel = %q, want main", got)
	}

	persistUpdateChannel(home, updateChannelDev)
	if got := readUpdateChannel(home); got != updateChannelDev {
		t.Fatalf("persisted channel = %q, want dev", got)
	}
	if got := readUpdateChannel(filepath.Join(home, "missing")); got != updateChannelMain {
		t.Fatalf("missing channel = %q, want main", got)
	}
}

func TestFetchReleaseAtValidatesDevPrerelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"dev","name":"dev-abc1234","prerelease":true,"assets":[]}`))
	}))
	defer server.Close()

	release, err := fetchReleaseAt(server.URL, updateChannelDev, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if got := releaseVersion(release, updateChannelDev); got != "dev-abc1234" {
		t.Fatalf("release version = %q", got)
	}
}

func TestCurrentVersionComparisonIsChannelAware(t *testing.T) {
	if !isCurrentVersion("dev-abc1234", "dev-abc1234", updateChannelDev) {
		t.Fatal("matching dev commits should be current")
	}
	if isCurrentVersion("v3.1.1", "dev-abc1234", updateChannelDev) {
		t.Fatal("stable build should not satisfy dev channel")
	}
	if !isCurrentVersion("3.1.1", "v3.1.1", updateChannelMain) {
		t.Fatal("equivalent stable versions should be current")
	}
}
