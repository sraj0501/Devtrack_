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

func TestSageBackgroundTimingDefaultsAndOverrides(t *testing.T) {
	t.Setenv("DEVTRACK_SAGE_MODEL_TIMEOUT_SECS", "")
	t.Setenv("DEVTRACK_SAGE_IDLE_POLL_MS", "")
	t.Setenv("DEVTRACK_SAGE_RETRY_DELAY_SECS", "")
	if got := GetSageModelTimeoutSecs(); got != 180 {
		t.Fatalf("model timeout=%d", got)
	}
	if got := GetSageIdlePollMS(); got != 500 {
		t.Fatalf("idle poll=%d", got)
	}
	if got := GetSageRetryDelaySecs(); got != 2 {
		t.Fatalf("retry delay=%d", got)
	}

	t.Setenv("DEVTRACK_SAGE_MODEL_TIMEOUT_SECS", "600")
	t.Setenv("DEVTRACK_SAGE_IDLE_POLL_MS", "100")
	t.Setenv("DEVTRACK_SAGE_RETRY_DELAY_SECS", "7")
	if got := GetSageModelTimeoutSecs(); got != 600 {
		t.Fatalf("model timeout override=%d", got)
	}
	if got := GetSageIdlePollMS(); got != 100 {
		t.Fatalf("idle poll override=%d", got)
	}
	if got := GetSageRetryDelaySecs(); got != 7 {
		t.Fatalf("retry delay override=%d", got)
	}
}

func TestSageBackgroundTimingRejectsUnboundedValues(t *testing.T) {
	t.Setenv("DEVTRACK_SAGE_MODEL_TIMEOUT_SECS", "3601")
	t.Setenv("DEVTRACK_SAGE_IDLE_POLL_MS", "6000")
	t.Setenv("DEVTRACK_SAGE_RETRY_DELAY_SECS", "0")
	if got := GetSageModelTimeoutSecs(); got != 180 {
		t.Fatalf("model timeout fallback=%d", got)
	}
	if got := GetSageIdlePollMS(); got != 500 {
		t.Fatalf("idle poll fallback=%d", got)
	}
	if got := GetSageRetryDelaySecs(); got != 2 {
		t.Fatalf("retry delay fallback=%d", got)
	}
}
