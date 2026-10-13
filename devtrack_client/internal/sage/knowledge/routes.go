package knowledge

import (
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
)

const routesFile = ".routes.json"

var routeBinary = regexp.MustCompile(`^[a-z0-9][a-z0-9._+-]{0,127}$`)

// Route explains the effective destination. Existing entries take precedence
// over remembered decisions, including after a user moves a topic file.
type Route struct {
	Binary   string `json:"binary"`
	Filename string `json:"filename"`
	Title    string `json:"title"`
	Source   string `json:"source"`
}

func ValidRouteBinary(binary string) bool { return routeBinary.MatchString(binary) }

// RememberRoute records future routing without moving existing documentation.
// Callers in different processes must hold the same Sage database writer lock.
func (w Writer) RememberRoute(binary, topic string) error {
	binary = strings.ToLower(strings.TrimSpace(binary))
	if !ValidRouteBinary(binary) {
		return errors.New("invalid Sage route binary")
	}
	publicationMu.Lock()
	defer publicationMu.Unlock()
	if err := w.prepare(); err != nil {
		return err
	}
	if _, err := w.recoverMerge(); err != nil {
		return err
	}
	if err := w.ensureGitignore(); err != nil {
		return err
	}
	routes, err := w.loadRoutes()
	if err != nil {
		return err
	}
	name := SafeFilename(topic)
	if routes[binary] == name {
		return nil
	}
	routes[binary] = name
	raw, err := json.MarshalIndent(routes, "", "  ")
	if err != nil {
		return err
	}
	return w.write(routesFile, append(raw, '\n'))
}

func (w Writer) loadRoutes() (map[string]string, error) {
	raw, err := w.read(routesFile)
	if err != nil {
		return nil, err
	}
	routes := make(map[string]string)
	if len(raw) == 0 {
		return routes, nil
	}
	if json.Unmarshal(raw, &routes) != nil || routes == nil {
		return nil, errors.New("invalid Sage routes file")
	}
	for binary, name := range routes {
		if !ValidRouteBinary(binary) || SafeFilename(name) != name {
			return nil, errors.New("invalid Sage route")
		}
	}
	return routes, nil
}

// Routes returns sorted, effective routes with provenance. No command is run.
func (w Writer) Routes() ([]Route, error) {
	publicationMu.Lock()
	defer publicationMu.Unlock()
	if err := w.prepare(); err != nil {
		return nil, err
	}
	return w.routes()
}

// Route returns the effective destination for one binary. Existing entries
// take precedence over remembered classifier or user choices.
func (w Writer) Route(binary string) (Route, bool, error) {
	binary = strings.ToLower(strings.TrimSpace(binary))
	if !ValidRouteBinary(binary) {
		return Route{}, false, errors.New("invalid Sage route binary")
	}
	routes, err := w.Routes()
	if err != nil {
		return Route{}, false, err
	}
	for _, route := range routes {
		if route.Binary == binary {
			return route, true, nil
		}
	}
	return Route{}, false, nil
}

// Sections returns the stable section headings in one safe topic file. It is
// used only as bounded context for distillation; entry content is not exposed.
func (w Writer) Sections(filename string) ([]string, error) {
	if SafeFilename(filename) != filename {
		return nil, errors.New("invalid Sage topic filename")
	}
	publicationMu.Lock()
	defer publicationMu.Unlock()
	if err := w.prepare(); err != nil {
		return nil, err
	}
	raw, err := w.read(filename)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var sections []string
	for _, entry := range ParseMarkdown(string(raw)) {
		if entry.Section != "" && !seen[entry.Section] {
			seen[entry.Section] = true
			sections = append(sections, entry.Section)
		}
	}
	return sections, nil
}

func (w Writer) routes() ([]Route, error) {
	if pending, err := w.read(mergeFile); err != nil {
		return nil, err
	} else if len(pending) != 0 {
		return nil, errors.New("Sage merge recovery pending; retry the merge")
	}
	saved, err := w.loadRoutes()
	if err != nil {
		return nil, err
	}
	index := make(map[string]Route)
	for binary, name := range saved {
		title, err := w.title(name)
		if err != nil {
			return nil, err
		}
		index[binary] = Route{binary, name, title, "remembered"}
	}
	names, err := w.topics()
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		// Routing only emits portable names; user files remain untouched.
		if SafeFilename(name) != name {
			continue
		}
		raw, err := w.read(name)
		if err != nil {
			return nil, err
		}
		for _, entry := range ParseMarkdown(string(raw)) {
			binary := entryBinary(entry)
			if binary == "" || index[binary].Source == "entries" {
				continue
			}
			index[binary] = Route{binary, name, topicTitle(name, string(raw)), "entries"}
		}
	}
	out := make([]Route, 0, len(index))
	for _, route := range index {
		out = append(out, route)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Binary < out[j].Binary })
	return out, nil
}

// The capture adapter currently accepts simple commands. Shell wrappers and
// compound-command action parsing are a separate parity boundary.
func entryBinary(entry MarkdownEntry) string {
	action := entryAction(entry)
	if action != "" {
		return strings.Fields(action)[0]
	}
	return ""
}

func (w Writer) title(name string) (string, error) {
	raw, err := w.read(name)
	return topicTitle(name, string(raw)), err
}

func topicTitle(name, text string) string {
	fence := ""
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if fence != "" {
			if closesFence(trim, fence) {
				fence = ""
			}
			continue
		}
		if f := openingFence(trim); f != "" {
			fence = f
			continue
		}
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(line[2:])
		}
	}
	return strings.ReplaceAll(strings.TrimSuffix(name, ".md"), "-", " ")
}
