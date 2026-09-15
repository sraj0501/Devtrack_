package harness

import (
	"testing"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/hooks"
)

type fakeAdapter struct {
	id        string
	installed bool
}

func (f *fakeAdapter) Descriptor() Descriptor {
	return Descriptor{ID: f.id, DisplayName: f.id, BuiltIn: false}
}
func (f *fakeAdapter) Install() (hooks.Installation, error) {
	f.installed = true
	return hooks.Installation{Installed: true, Mode: "fake"}, nil
}
func (f *fakeAdapter) Uninstall() (hooks.Installation, error) {
	f.installed = false
	return hooks.Installation{Mode: "disabled"}, nil
}
func (f *fakeAdapter) Status() (hooks.Installation, error) {
	return hooks.Installation{Installed: f.installed, Mode: map[bool]string{true: "fake", false: "disabled"}[f.installed]}, nil
}

func TestRegistryInstallsOnlySelectedAdapter(t *testing.T) {
	alpha, beta := &fakeAdapter{id: "alpha"}, &fakeAdapter{id: "beta"}
	registry := New(beta, alpha)
	list, err := registry.List()
	if err != nil || len(list) != 2 || list[0].ID != "alpha" || list[1].ID != "beta" {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	if _, err := registry.Install("alpha"); err != nil {
		t.Fatal(err)
	}
	if !alpha.installed || beta.installed {
		t.Fatalf("selection leaked: alpha=%t beta=%t", alpha.installed, beta.installed)
	}
	if _, err := registry.Uninstall("alpha"); err != nil || alpha.installed {
		t.Fatalf("uninstall: %v", err)
	}
	if _, err := registry.Install("missing"); err == nil {
		t.Fatal("unknown adapter must fail")
	}
}
