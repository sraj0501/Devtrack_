# pm-config.md
# Global agent configuration for the DevTrack project.
# Agents read this file at startup (Step 0) to understand project context.
# Schema reference: ~/.claude/AGENT_AUTHORING.md

```yaml
project:
  name: DevTrack
  description: "Go daemon + Python AI pipeline — monitors git/timers, AI enrichment, routes to PM systems"
  languages: [Go, Python]
  type: daemon

paths:
  board: Data/agent_logs/project_board.md
  engineer_log: Data/agent_logs/engineer_log.md
  feature_tracker: Data/agent_logs/feature_tracker.md
  posts_dir: Data/agent_logs/posts
  docs: docs/
  src: [devtrack_client/, devtrack_server/]
  memory_project: .claude/memory/

git:
  default_branch: main
  merge_target: dev
  branch_prefixes: [features/, fix/, docs/, hotfix/]
  pr_base: dev
  push_prefix: "GIT_NO_DEVTRACK=1"

commit:
  tool: devtrack
  command: devtrack git commit
  fallback: git commit

test:
  commands:
    - cd devtrack_client && go test ./...
    - cd devtrack_server && uv run pytest backend/tests/ -q

scan:
  paths: [devtrack_client/, devtrack_server/backend/]
  file_types: ["*.go", "*.py"]
  exclude: ["*_test.go", "test_*.py", "conftest.py", "config.py", "config_env.go"]

vision:
  rules:
    - "offline-first: every feature must work with Ollama and no internet. Red flags: hard cloud URL dependency; failure when MongoDB/Redis absent"
    - "CLI-only: devtrack binary is terminal-only, no browser launching from the Go binary. Red flags: subprocess opening a browser; HTML served from Go"
    - "wedge-first: public-facing copy leads with standup-generation, not platform. Red flags: README leading with Swiss Army knife metaphor; first-run path getting longer"
  pm_system: github

posts:
  author: Shashank Raj
  platform: dev.to
```
