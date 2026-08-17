package main

import (
	"fmt"
	"io"
)

func printFirstRunGuidance(out io.Writer) {
	result, _ := ReadFirstRunResult()
	writeFirstRunGuidance(out, result, GetServerMode() != ServerModeExternal)
}

func writeFirstRunGuidance(out io.Writer, result *FirstRunResult, automaticVoice bool) {
	fmt.Fprintln(out, "First-run quick win:")
	fmt.Fprintln(out, "  Run: devtrack mcp setup")
	fmt.Fprintln(out, `  Then ask Claude Code: "What am I working on?"`)
	if result == nil && automaticVoice {
		fmt.Fprintln(out, "  Voice profile: builds automatically when the optional AI server is ready")
	} else if result == nil {
		fmt.Fprintln(out, "  Voice profile: automatic local mining is available in managed mode")
	} else {
		fmt.Fprintf(out, "  Voice profile: built from %d commits\n", result.CommitCount)
		fmt.Fprintln(out, "  Try: devtrack work report")
	}
	fmt.Fprintln(out)
}
