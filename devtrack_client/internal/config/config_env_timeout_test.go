package config

import "testing"

func TestHTTPAndSQLiteTimeoutDefaults(t *testing.T) {
	t.Setenv("HTTP_TIMEOUT", "")
	t.Setenv("HTTP_TIMEOUT_LONG", "")
	t.Setenv("SQLITE_BUSY_TIMEOUT_MS", "")

	if got := GetHTTPTimeout(); got != 30 {
		t.Fatalf("GetHTTPTimeout() = %d, want 30", got)
	}
	if got := GetHTTPTimeoutLong(); got != 60 {
		t.Fatalf("GetHTTPTimeoutLong() = %d, want 60", got)
	}
	if got := GetSQLiteBusyTimeoutMS(); got != 5000 {
		t.Fatalf("GetSQLiteBusyTimeoutMS() = %d, want 5000", got)
	}
}

func TestHTTPAndSQLiteTimeoutOverrides(t *testing.T) {
	t.Setenv("HTTP_TIMEOUT", "41")
	t.Setenv("HTTP_TIMEOUT_LONG", "91")
	t.Setenv("SQLITE_BUSY_TIMEOUT_MS", "7500")

	if got := GetHTTPTimeout(); got != 41 {
		t.Fatalf("GetHTTPTimeout() = %d, want 41", got)
	}
	if got := GetHTTPTimeoutLong(); got != 91 {
		t.Fatalf("GetHTTPTimeoutLong() = %d, want 91", got)
	}
	if got := GetSQLiteBusyTimeoutMS(); got != 7500 {
		t.Fatalf("GetSQLiteBusyTimeoutMS() = %d, want 7500", got)
	}
}
