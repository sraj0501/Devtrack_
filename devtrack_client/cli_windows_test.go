//go:build windows

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func consoleWindow() uintptr {
	handle, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleWindow").Call()
	return handle
}

func TestReferenceWindowsHelpersRequestNoConsole(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/d", "/c", "exit", "0")
	setSetsid(cmd)
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&createNoWindowFlag == 0 {
		t.Fatal("background command did not request CREATE_NO_WINDOW")
	}
}

func TestReferenceRealHelperHasNoConsoleAndPreservesPipedInput(t *testing.T) {
	if os.Getenv("DEVTRACK_TEST_NO_CONSOLE_HELPER") == "1" {
		payload, err := io.ReadAll(os.Stdin)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("%d\n%s\n", consoleWindow(), payload)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestReferenceRealHelperHasNoConsoleAndPreservesPipedInput$")
	cmd.Env = append(os.Environ(), "DEVTRACK_TEST_NO_CONSOLE_HELPER=1")
	cmd.Stdin = strings.NewReader("hook payload")
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	setSetsid(cmd)
	if err := cmd.Run(); err != nil {
		t.Fatalf("helper failed: %v: %s", err, output.String())
	}
	if !strings.Contains(output.String(), "0\nhook payload\n") {
		t.Fatalf("helper console/input output = %q", output.String())
	}
}

func TestReferenceDetachedProcessHasNoConsole(t *testing.T) {
	if output := os.Getenv("DEVTRACK_TEST_DETACHED_CONSOLE_FILE"); output != "" {
		if err := os.WriteFile(output, []byte(fmt.Sprint(consoleWindow())), 0600); err != nil {
			t.Fatal(err)
		}
		return
	}

	output := filepath.Join(t.TempDir(), "console.txt")
	cmd := exec.Command(os.Args[0], "-test.run=^TestReferenceDetachedProcessHasNoConsole$")
	cmd.Env = append(os.Environ(), "DEVTRACK_TEST_DETACHED_CONSOLE_FILE="+output)
	setSetsid(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Process.Release(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(output)
		if err == nil {
			if string(data) != "0" {
				t.Fatalf("detached helper console handle = %q", data)
			}
			return
		}
		if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("detached helper did not report its console state")
}
