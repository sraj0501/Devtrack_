// Package command reduces shell command lines to privacy-safe Sage actions.
// Values, paths, and free-form arguments never enter the normalized result.
package command

import (
	"sort"
	"strings"
	"unicode"
)

var prefixes = map[string]bool{
	"sudo": true, "nohup": true, "time": true, "command": true,
	"exec": true, "env": true, "nice": true, "xargs": true,
}

var skippedBinaries = map[string]bool{
	"echo": true, "printf": true, "true": true, "false": true, ":": true,
	"cd": true, "pwd": true, "clear": true, "sleep": true, "export": true,
	"source": true, ".": true, "wait": true, "exit": true, "read": true,
	"test": true, "[": true, "ls": true,
}

var subcommandBinaries = map[string]bool{
	"git": true, "gh": true, "glab": true, "docker": true, "docker-compose": true,
	"podman": true, "kubectl": true, "helm": true, "npm": true, "npx": true,
	"yarn": true, "pnpm": true, "bun": true, "deno": true, "node": true,
	"cargo": true, "go": true, "rustup": true, "pip": true, "pip3": true,
	"pipx": true, "poetry": true, "uv": true, "conda": true, "brew": true,
	"apt": true, "apt-get": true, "az": true, "aws": true, "gcloud": true,
	"terraform": true, "systemctl": true, "make": true, "psql": true,
	"mysql": true, "redis-cli": true, "influx": true, "alembic": true,
	"django-admin": true, "manage.py": true, "pytest": true, "ruff": true,
	"black": true, "mypy": true, "eslint": true, "prettier": true,
	"vitest": true, "jest": true, "tsc": true, "devin": true, "claude": true,
	"codex": true, "copilot": true, "opencode": true, "cursor-agent": true,
	"code": true, "jj": true, "hg": true, "svn": true,
}

var baseDepth = map[string]int{"az": 3, "aws": 3, "gcloud": 3}

var nested = map[string]map[string]bool{
	"git": set("stash", "remote", "submodule", "worktree", "bisect", "reflog", "notes", "config", "sparse-checkout", "lfs", "maintenance", "subtree"),
	"npm": set("run"), "yarn": set("run"), "pnpm": set("run"), "bun": set("run"),
	"docker": set("compose", "image", "container", "volume", "network", "system", "builder", "buildx", "context", "swarm", "node", "secret", "stack"),
	"podman": set("compose", "image", "container", "volume", "network", "system"),
	"gh":     set("pr", "issue", "run", "repo", "workflow", "auth", "api", "release", "secret", "variable", "ruleset", "cache", "codespace", "project"),
	"go":     set("mod", "tool"), "brew": set("services"), "poetry": set("run"),
	"uv": set("run", "pip", "tool"), "terraform": set("state", "workspace"),
	"make":    {},
	"kubectl": set("get", "describe", "delete", "apply", "create", "edit", "patch", "scale", "rollout", "top", "config", "label", "annotate"),
	"helm":    set("repo", "plugin", "dependency"),
}

func set(values ...string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

// Action is one shell segment reduced to reusable, non-sensitive identity.
type Action struct {
	Binary      string
	Subcommands []string
	Flags       []string
	Command     string
	Signature   string
}

// Parse returns one normalized action per shell segment. Operators inside
// quotes remain data. Malformed quoting is rejected instead of guessed.
func Parse(raw string) []Action {
	segments, ok := tokenize(raw)
	if !ok {
		return nil
	}
	out := make([]Action, 0, len(segments))
	for _, tokens := range segments {
		if action, ok := normalize(tokens); ok {
			out = append(out, action)
		}
	}
	return out
}

// Identity returns the binary plus true subcommands, deliberately ignoring
// flags and argument values. It is used to attach command variants to an
// existing documented entry without another model call.
func Identity(raw string) string {
	actions := Parse(raw)
	if len(actions) == 0 {
		return ""
	}
	return strings.Join(append([]string{actions[0].Binary}, actions[0].Subcommands...), " ")
}

func normalize(tokens []string) (Action, bool) {
	index := 0
	for index < len(tokens) && (prefixes[strings.ToLower(tokens[index])] || assignment(tokens[index])) {
		index++
	}
	if index >= len(tokens) {
		return Action{}, false
	}
	binary := strings.ToLower(baseName(tokens[index]))
	if binary == "" || skippedBinaries[binary] || strings.HasPrefix(binary, "-") || !identifier(binary) {
		return Action{}, false
	}

	rest := tokens[index+1:]
	subcommands := make([]string, 0, 3)
	flags := make([]string, 0, 4)
	skipNext := false
	for i, token := range rest {
		if skipNext {
			skipNext = false
			continue
		}
		if strings.HasPrefix(token, "-") && token != "-" {
			name := strings.SplitN(token, "=", 2)[0]
			if validFlag(name) {
				flags = append(flags, strings.ToLower(name))
			}
			nextIsValue := i+1 < len(rest) && !strings.HasPrefix(rest[i+1], "-")
			if (binary == "python" || binary == "python3") && name == "-m" && nextIsValue && identifier(rest[i+1]) {
				subcommands = append(subcommands, strings.ToLower(rest[i+1]))
				skipNext = true
			} else if !strings.Contains(token, "=") && len(name) <= 2 && nextIsValue {
				skipNext = true
			}
			continue
		}
		if !subcommandBinaries[binary] || len(subcommands) >= depth(binary, subcommands) || !identifier(token) {
			continue
		}
		subcommands = append(subcommands, strings.ToLower(token))
	}
	flags = uniqueSorted(flags)
	parts := append([]string{binary}, subcommands...)
	parts = append(parts, flags...)
	canonical := strings.Join(parts, " ")
	if len(canonical) > 127 {
		return Action{}, false
	}
	return Action{Binary: binary, Subcommands: subcommands, Flags: flags, Command: canonical, Signature: canonical}, true
}

func depth(binary string, subcommands []string) int {
	value := baseDepth[binary]
	if value == 0 {
		value = 1
	}
	if len(subcommands) > 0 && nested[binary][subcommands[0]] {
		value++
	}
	return value
}

func assignment(token string) bool {
	index := strings.IndexByte(token, '=')
	return index > 0 && !strings.HasPrefix(token, "-") && identifier(token[:index])
}

func baseName(value string) string {
	value = strings.TrimRight(value, "/\\")
	if index := strings.LastIndexAny(value, "/\\"); index >= 0 {
		return value[index+1:]
	}
	return value
}

func identifier(value string) bool {
	if value == "" || !unicode.IsLetter(rune(value[0])) {
		return false
	}
	for _, r := range value[1:] {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("_:+-.", r) {
			return false
		}
	}
	return true
}

func validFlag(value string) bool {
	if len(value) < 2 || value[0] != '-' {
		return false
	}
	for _, r := range value[1:] {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func tokenize(raw string) ([][]string, bool) {
	var segments [][]string
	segment := make([]string, 0, 8)
	var word strings.Builder
	var quote rune
	escaped := false
	flushWord := func() {
		if word.Len() > 0 {
			segment = append(segment, word.String())
			word.Reset()
		}
	}
	flushSegment := func() {
		flushWord()
		if len(segment) > 0 {
			segments = append(segments, segment)
			segment = make([]string, 0, 8)
		}
	}
	runes := []rune(raw)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if escaped {
			word.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if unicode.IsSpace(r) {
			flushWord()
			continue
		}
		if r == ';' || r == '|' || r == '&' {
			flushSegment()
			if i+1 < len(runes) && ((r == '&' && runes[i+1] == '&') || (r == '|' && (runes[i+1] == '|' || runes[i+1] == '&'))) {
				i++
			}
			continue
		}
		word.WriteRune(r)
	}
	if escaped || quote != 0 {
		return nil, false
	}
	flushSegment()
	return segments, true
}
