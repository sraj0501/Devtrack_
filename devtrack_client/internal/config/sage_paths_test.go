package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestReferenceRuntimeSettingIsRelativeToConfig(t *testing.T) {
	base := filepath.Join(t.TempDir(), "config", "sage")
	t.Setenv("DEVTRACK_SAGE_RUNTIME_DIR", ".runtime")

	got := GetSageRuntimeDir(base)
	want := filepath.Join(base, ".runtime")
	if got != want {
		t.Fatalf("GetSageRuntimeDir() = %q, want %q", got, want)
	}
}

func TestReferenceExplicitKbOverrideKeepsRuntimeIsolated(t *testing.T) {
	base := filepath.Join(t.TempDir(), "config", "sage")
	knowledge := filepath.Join(t.TempDir(), "isolated knowledge")
	t.Setenv("DEVTRACK_SAGE_RUNTIME_DIR", "")
	t.Setenv("DEVTRACK_SAGE_KNOWLEDGE_DIR", knowledge)

	runtimeRoot := GetSageRuntimeDir(base)
	knowledgeRoot := GetSageKnowledgeDir(runtimeRoot)
	if runtimeRoot != filepath.Clean(base) {
		t.Fatalf("runtime root = %q, want %q", runtimeRoot, base)
	}
	if knowledgeRoot != filepath.Clean(knowledge) {
		t.Fatalf("knowledge root = %q, want %q", knowledgeRoot, knowledge)
	}
	if runtimeRoot == knowledgeRoot || runtimeRoot == filepath.Join(knowledgeRoot, ".runtime") {
		t.Fatalf("runtime root %q must remain isolated from knowledge root %q", runtimeRoot, knowledgeRoot)
	}
}

func TestRelativeKnowledgeOverrideIsNotWorkingDirectoryRelative(t *testing.T) {
	base := filepath.Join(t.TempDir(), "config", "sage")
	t.Setenv("DEVTRACK_SAGE_KNOWLEDGE_DIR", "personal knowledge")

	got := GetSageKnowledgeDir(base)
	want := filepath.Join(base, "personal knowledge")
	if got != want {
		t.Fatalf("GetSageKnowledgeDir() = %q, want %q", got, want)
	}
}

func TestRuntimeOverrideDoesNotMoveDefaultKnowledgeRoot(t *testing.T) {
	base := filepath.Join(t.TempDir(), "config", "sage")
	t.Setenv("DEVTRACK_SAGE_RUNTIME_DIR", ".runtime")
	t.Setenv("DEVTRACK_SAGE_KNOWLEDGE_DIR", "")

	runtimeRoot := GetSageRuntimeDir(base)
	knowledgeRoot := GetSageKnowledgeDir(base)
	if runtimeRoot != filepath.Join(base, ".runtime") {
		t.Fatalf("runtime root = %q", runtimeRoot)
	}
	if knowledgeRoot != filepath.Join(base, "knowledge") {
		t.Fatalf("knowledge root = %q", knowledgeRoot)
	}
	if strings.HasPrefix(knowledgeRoot, runtimeRoot+string(filepath.Separator)) {
		t.Fatalf("knowledge root %q must not be nested under runtime root %q", knowledgeRoot, runtimeRoot)
	}
}
