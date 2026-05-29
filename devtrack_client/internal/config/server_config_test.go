package config

import (
	"os"
	"testing"
)

// ---------------------------------------------------------------------------
// GetServerMode / GetServerURL (config helpers)
// ---------------------------------------------------------------------------

func TestGetServerMode_Defaults_Managed(t *testing.T) {
	os.Unsetenv("DEVTRACK_SERVER_MODE")
	// Ensure no cloud.json can be found (we're in test; cloud.go will return false)
	mode := GetServerMode()
	if mode != ServerModeManaged {
		t.Errorf("unexpected default server mode: %s", mode)
	}
}

func TestGetServerMode_External(t *testing.T) {
	os.Setenv("DEVTRACK_SERVER_MODE", "external")
	defer os.Unsetenv("DEVTRACK_SERVER_MODE")
	mode := GetServerMode()
	if mode != ServerModeExternal {
		t.Errorf("expected external, got %s", mode)
	}
}

func TestGetServerURL_UsesEnvVar(t *testing.T) {
	os.Setenv("DEVTRACK_SERVER_URL", "http://example.com:9000")
	os.Setenv("DEVTRACK_TLS", "false")
	defer func() {
		os.Unsetenv("DEVTRACK_SERVER_URL")
		os.Unsetenv("DEVTRACK_TLS")
	}()
	url := GetServerURL()
	if url != "http://example.com:9000" {
		t.Errorf("expected http://example.com:9000, got %s", url)
	}
}

func TestGetServerURL_DefaultsToLocalhost(t *testing.T) {
	os.Unsetenv("DEVTRACK_SERVER_URL")
	os.Setenv("DEVTRACK_TLS", "false")
	os.Setenv("WEBHOOK_PORT", "8089")
	defer func() {
		os.Unsetenv("DEVTRACK_TLS")
		os.Unsetenv("WEBHOOK_PORT")
	}()
	url := GetServerURL()
	if url != "http://127.0.0.1:8089" {
		t.Errorf("expected http://127.0.0.1:8089, got %s", url)
	}
}

func TestIsExternalServer_False_WhenManaged(t *testing.T) {
	os.Unsetenv("DEVTRACK_SERVER_MODE")
	// Without cloud.json this should be managed (not external)
	if IsExternalServer() {
		// Only fail if we're also not in cloud mode
		if GetServerMode() == ServerModeManaged {
			t.Error("expected IsExternalServer=false in managed mode")
		}
	}
}

func TestIsExternalServer_True_WhenExternal(t *testing.T) {
	os.Setenv("DEVTRACK_SERVER_MODE", "external")
	defer os.Unsetenv("DEVTRACK_SERVER_MODE")
	if !IsExternalServer() {
		t.Error("expected IsExternalServer=true in external mode")
	}
}
