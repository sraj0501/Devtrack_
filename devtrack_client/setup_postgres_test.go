package main

import (
	"strings"
	"testing"
)

func TestValidatePostgresURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "standard", value: "postgresql://user:pass@localhost:5432/devtrack"},
		{name: "driver scheme", value: "postgresql+psycopg2://user:pass@db/devtrack"},
		{name: "unix socket", value: "postgresql:///devtrack"},
		{name: "missing", value: "", wantErr: true},
		{name: "sqlite", value: "sqlite:///devtrack.db", wantErr: true},
		{name: "missing database", value: "postgresql://user:pass@localhost", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validatePostgresURL(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("validatePostgresURL(%q) error = %v, wantErr %v", test.value, err, test.wantErr)
			}
		})
	}
}

func TestGenerateEnvContentIncludesPostgresURL(t *testing.T) {
	cfg := &SetupConfig{
		ProjectRoot:   "/srv/devtrack",
		WorkspacePath: "/work/project",
		DataDir:       "/data/devtrack",
		Mode:          ModeManaged,
		PostgresURL:   "postgresql://user:pass@db/devtrack",
		LLMProvider:   "ollama",
	}
	content := generateEnvContent(cfg)
	if !strings.Contains(content, "POSTGRES_URL=postgresql://user:pass@db/devtrack\n") {
		t.Fatal("generated environment does not contain POSTGRES_URL")
	}
}
