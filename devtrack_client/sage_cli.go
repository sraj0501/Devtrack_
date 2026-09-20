package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
	sageharness "github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/harness"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/hooks"
	sageknowledge "github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/knowledge"
)

// routeSage is intentionally pure so invalid commands fail before starting a
// model, daemon, network request, or Git operation.
func routeSage(args []string) (sub string, rest []string, err error) {
	if len(args) == 0 {
		return "", nil, fmt.Errorf("usage: devtrack sage status|pause|resume|search|topics|doctor|harness")
	}
	switch args[0] {
	case "status", "pause", "resume", "doctor", "harness", "install-hooks", "uninstall-hooks", "hook", "search", "topics":
		return args[0], args[1:], nil
	default:
		return "", nil, fmt.Errorf("unknown Sage command %q", args[0])
	}
}

func sageRoot() (string, error) {
	dataHome, err := config.DevtrackDataHome()
	if err != nil {
		return "", fmt.Errorf("sage data directory unavailable: %w", err)
	}
	return filepath.Join(dataHome, "sage"), nil
}

func runSageState(sub string, args []string) error {
	return writeSageState(sub, args, os.Stdout)
}

func writeSageState(sub string, args []string, output io.Writer) error {
	if len(args) != 0 {
		return fmt.Errorf("sage %s takes no arguments", sub)
	}
	root, err := sageRoot()
	if err != nil {
		return err
	}
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

func runSageHookInstall(install bool, args []string) error {
	if len(args) != 0 && !(len(args) == 2 && args[0] == "--harness" && args[1] == "codex") {
		command := "uninstall-hooks"
		if install {
			command = "install-hooks"
		}
		return fmt.Errorf("usage: devtrack sage %s [--harness codex]", command)
	}
	root, err := sageRoot()
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("devtrack executable unavailable: %w", err)
	}
	registry := sageharness.New(sageharness.Codex{Installer: hooks.Installer{Root: root, Executable: executable, GOOS: runtime.GOOS}})
	var result hooks.Installation
	if install {
		result, err = registry.Install("codex")
	} else {
		result, err = registry.Uninstall("codex")
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func runSageHarness(args []string, output io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: devtrack sage harness list|install|uninstall [name]")
	}
	root, err := sageRoot()
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("devtrack executable unavailable: %w", err)
	}
	registry := sageharness.New(sageharness.Codex{Installer: hooks.Installer{Root: root, Executable: executable, GOOS: runtime.GOOS}})
	encoder := json.NewEncoder(output)
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return fmt.Errorf("usage: devtrack sage harness list")
		}
		result, err := registry.List()
		if err != nil {
			return err
		}
		return encoder.Encode(result)
	case "install", "uninstall":
		if len(args) != 2 {
			return fmt.Errorf("usage: devtrack sage harness %s <name>", args[0])
		}
		var result hooks.Installation
		if args[0] == "install" {
			result, err = registry.Install(args[1])
		} else {
			result, err = registry.Uninstall(args[1])
		}
		if err != nil {
			return err
		}
		return encoder.Encode(result)
	default:
		return fmt.Errorf("unknown Sage harness action")
	}
}

// runSageHook is deliberately silent and fail-open: malformed input or local
// storage failure must never interrupt the host agent session.
func runSageHook(args []string, input io.Reader) error {
	if len(args) != 2 || args[0] != "codex" || args[1] != hooks.OwnerMarker {
		return nil
	}
	raw, err := io.ReadAll(io.LimitReader(input, hooks.MaxCodexHookBytes+1))
	if err != nil {
		return nil
	}
	event, ok := hooks.NormalizeCodexPostToolUse(raw, time.Now().UTC())
	if !ok {
		return nil
	}
	root, err := sageRoot()
	if err != nil {
		return nil
	}
	_ = sage.WriteEvent(root, event)
	return nil
}

type sageKnowledgeStore interface {
	SearchSageKnowledge(query, topic string, limit int) ([]sageknowledge.Entry, error)
	ListSageTopics() ([]sageknowledge.Topic, error)
}

func runSageSearch(args []string, output io.Writer) error {
	database, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("sage knowledge unavailable: %w", err)
	}
	defer database.Close()
	return writeSageSearch(database, args, output)
}

func writeSageSearch(store sageKnowledgeStore, args []string, output io.Writer) error {
	var queryParts []string
	topic := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--topic" {
			if topic != "" || i+1 >= len(args) {
				return fmt.Errorf("usage: devtrack sage search <query> [--topic <topic>]")
			}
			topic = args[i+1]
			i++
			continue
		}
		queryParts = append(queryParts, args[i])
	}
	if len(queryParts) == 0 {
		return fmt.Errorf("usage: devtrack sage search <query> [--topic <topic>]")
	}
	entries, err := store.SearchSageKnowledge(strings.Join(queryParts, " "), topic, 20)
	if err != nil {
		return err
	}
	_, err = io.WriteString(output, sageknowledge.RenderEntries(entries))
	return err
}

func runSageTopics(args []string, output io.Writer) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: devtrack sage topics")
	}
	database, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("sage knowledge unavailable: %w", err)
	}
	defer database.Close()
	topics, err := database.ListSageTopics()
	if err != nil {
		return err
	}
	_, err = io.WriteString(output, sageknowledge.RenderTopics(topics))
	return err
}
