// Package harness owns the pluggable Sage capture-adapter registry.
package harness

import (
	"errors"
	"sort"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/hooks"
)

type Descriptor struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	BuiltIn     bool   `json:"built_in"`
	Installed   bool   `json:"installed"`
	Mode        string `json:"mode"`
}

type Adapter interface {
	Descriptor() Descriptor
	Install() (hooks.Installation, error)
	Uninstall() (hooks.Installation, error)
	Status() (hooks.Installation, error)
}

type Registry struct{ adapters map[string]Adapter }

func New(adapters ...Adapter) *Registry {
	registry := &Registry{adapters: map[string]Adapter{}}
	for _, adapter := range adapters {
		registry.Register(adapter)
	}
	return registry
}

func (r *Registry) Register(adapter Adapter) {
	if adapter == nil {
		return
	}
	descriptor := adapter.Descriptor()
	if descriptor.ID == "" {
		return
	}
	r.adapters[descriptor.ID] = adapter
}

func (r *Registry) List() ([]Descriptor, error) {
	result := make([]Descriptor, 0, len(r.adapters))
	for _, adapter := range r.adapters {
		descriptor := adapter.Descriptor()
		status, err := adapter.Status()
		if err != nil {
			return nil, err
		}
		descriptor.Installed, descriptor.Mode = status.Installed, status.Mode
		result = append(result, descriptor)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (r *Registry) Install(id string) (hooks.Installation, error) {
	adapter, ok := r.adapters[id]
	if !ok {
		return hooks.Installation{}, errors.New("unknown Sage harness")
	}
	return adapter.Install()
}

func (r *Registry) Uninstall(id string) (hooks.Installation, error) {
	adapter, ok := r.adapters[id]
	if !ok {
		return hooks.Installation{}, errors.New("unknown Sage harness")
	}
	return adapter.Uninstall()
}

type Codex struct{ Installer hooks.Installer }

func (c Codex) Descriptor() Descriptor {
	return Descriptor{ID: "codex", DisplayName: "Codex", BuiltIn: true}
}
func (c Codex) Install() (hooks.Installation, error)   { return c.Installer.Install() }
func (c Codex) Uninstall() (hooks.Installation, error) { return c.Installer.Uninstall() }
func (c Codex) Status() (hooks.Installation, error)    { return c.Installer.Status() }
