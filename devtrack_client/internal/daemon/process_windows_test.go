//go:build windows

package daemon

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestReferenceTasklistAccessFailureDoesNotReportADeadProcess(t *testing.T) {
	if !processMayBeAliveWhenOpenFails(windows.ERROR_ACCESS_DENIED) {
		t.Fatal("restricted process visibility was treated as proof of death")
	}
	if processMayBeAliveWhenOpenFails(windows.ERROR_INVALID_PARAMETER) {
		t.Fatal("a definitely invalid PID was treated as alive")
	}
}
