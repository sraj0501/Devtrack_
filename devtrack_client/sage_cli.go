package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
)

// routeSage is intentionally pure so legacy and explicit Git aliases can be
// checked without starting the LLM, daemon, or a live Git repository.
func routeSage(args []string) (sub string, rest []string, legacy bool) {
	if len(args) == 0 {
		return "interactive", nil, true
	}
	if args[0] == "git" {
		if len(args) == 1 {
			return "interactive", nil, false
		}
		return args[1], args[2:], false
	}
	switch args[0] {
	case "status", "pause", "resume", "doctor", "install-hooks", "search", "topics":
		return args[0], args[1:], false
	default:
		return args[0], args[1:], true
	}
}

func runSageState(sub string, args []string) error {
	return writeSageState(sub, args, os.Stdout)
}

func writeSageState(sub string, args []string, output io.Writer) error {
	if len(args) != 0 {
		return fmt.Errorf("sage %s takes no arguments", sub)
	}
	dataHome, err := config.DevtrackDataHome()
	if err != nil {
		return fmt.Errorf("sage data directory unavailable: %w", err)
	}
	root := filepath.Join(dataHome, "sage")
	var state sage.State
	switch sub {
	case "pause":
		state, err = sage.SetPaused(root, true)
	case "resume":
		state, err = sage.SetPaused(root, false)
	default:
		state, err = sage.ReadState(root)
	}
	if err != nil {
		return fmt.Errorf("sage state unavailable: %w", err)
	}
	return json.NewEncoder(output).Encode(state)
}
