package ticket

import "testing"

func TestResolverCanonicalKinds(t *testing.T) {
	resolver, err := NewResolver(ResolverConfig{TicketKey: "PROJ"})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range DefaultKinds {
		branch := kind + "/PROJ-123-valid-slug"
		got := resolver.Resolve(ResolveInput{Branch: branch})
		if got.TicketID != "PROJ-123" || got.Source != SourceBranch || got.State != StateLinked {
			t.Errorf("Resolve(%q) = %#v", branch, got)
		}
	}
}

func TestResolverRejectsNonCanonicalBranches(t *testing.T) {
	resolver, err := NewResolver(ResolverConfig{TicketKey: "PROJ"})
	if err != nil {
		t.Fatal(err)
	}
	cases := []string{
		"main",
		"feature/no-ticket",
		"feature/proj-123-slug",
		"feature/OTHER-123-slug",
		"prefix/feature/PROJ-123-slug",
		"feature/PROJ-0-slug",
		"feature/PROJ-123",
		"feature/PROJ-123-Bad-Slug",
		"feature/PROJ-123-good_slug",
		"feature/PROJ-123-slug/extra",
	}
	for _, branch := range cases {
		if got := resolver.Resolve(ResolveInput{Branch: branch}); got.State != StateUnlinked || got.TicketID != "" {
			t.Errorf("Resolve(%q) = %#v, want unlinked", branch, got)
		}
	}
}

func TestResolverPrecedenceAndConflicts(t *testing.T) {
	resolver, err := NewResolver(ResolverConfig{TicketKey: "PROJ"})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		input      ResolveInput
		wantID     string
		wantSource Source
		wantState  State
		conflict   bool
	}{
		{
			name:   "branch wins",
			input:  ResolveInput{Branch: "feature/PROJ-1-slug", CommitMessage: "PROJ-2: other", ActiveTicket: "PROJ-3"},
			wantID: "PROJ-1", wantSource: SourceBranch, wantState: StateConflict, conflict: true,
		},
		{
			name:   "prefix fallback",
			input:  ResolveInput{Branch: "main", CommitMessage: "PROJ-2: fix it"},
			wantID: "PROJ-2", wantSource: SourceCommitPrefix, wantState: StateLinked,
		},
		{
			name:   "trailer fallback",
			input:  ResolveInput{Branch: "main", CommitMessage: "fix it\n\nRefs: PROJ-3"},
			wantID: "PROJ-3", wantSource: SourceCommitTrailer, wantState: StateLinked,
		},
		{
			name:   "active fallback",
			input:  ResolveInput{Branch: "main", CommitMessage: "fix it", ActiveTicket: "PROJ-4"},
			wantID: "PROJ-4", wantSource: SourceActiveTicket, wantState: StateLinked,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolver.Resolve(tc.input)
			if got.TicketID != tc.wantID || got.Source != tc.wantSource || got.State != tc.wantState || got.Conflict != tc.conflict {
				t.Fatalf("Resolve() = %#v", got)
			}
		})
	}
}

func TestResolverIgnoresIncidentalProse(t *testing.T) {
	resolver, err := NewResolver(ResolverConfig{TicketKey: "PROJ"})
	if err != nil {
		t.Fatal(err)
	}
	got := resolver.Resolve(ResolveInput{Branch: "main", CommitMessage: "fix bug mentioned in PROJ-99 during review"})
	if got.State != StateUnlinked || got.TicketID != "" {
		t.Fatalf("Resolve() = %#v, want unlinked", got)
	}
}

func TestResolverAmbiguousExplicitMessageIsUnlinked(t *testing.T) {
	resolver, err := NewResolver(ResolverConfig{TicketKey: "PROJ"})
	if err != nil {
		t.Fatal(err)
	}
	got := resolver.Resolve(ResolveInput{
		Branch:        "main",
		CommitMessage: "PROJ-1: fix it\n\nRefs: PROJ-2",
	})
	if got.State != StateUnlinked || got.TicketID != "" || !got.Conflict {
		t.Fatalf("Resolve() = %#v, want ambiguous unlinked result", got)
	}
}

func TestResolverMergeSourceBranch(t *testing.T) {
	resolver, err := NewResolver(ResolverConfig{TicketKey: "PROJ"})
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range []string{
		"Merge branch 'feature/PROJ-42-merge-me'",
		"Merge pull request #12 from owner/feature/PROJ-42-merge-me",
	} {
		got := resolver.Resolve(ResolveInput{Branch: "main", CommitMessage: message, IsMerge: true})
		if got.TicketID != "PROJ-42" || got.Source != SourceMergeBranch {
			t.Errorf("Resolve(%q) = %#v", message, got)
		}
	}
}

func TestResolverAmbiguousMergeSourcesAreUnlinked(t *testing.T) {
	resolver, err := NewResolver(ResolverConfig{TicketKey: "PROJ"})
	if err != nil {
		t.Fatal(err)
	}
	got := resolver.Resolve(ResolveInput{
		Branch:        "main",
		CommitMessage: "Merge branch 'feature/PROJ-41-first'\nMerge pull request #12 from owner/fix/PROJ-42-second",
		IsMerge:       true,
	})
	if got.TicketID != "" || got.State != StateUnlinked || !got.Conflict {
		t.Fatalf("Resolve() = %#v, want ambiguous unlinked merge", got)
	}
}

func TestResolverCustomPatternValidation(t *testing.T) {
	invalid := []string{
		`(?P<ticket>DT-\d+)`,
		`^(DT-\d+)$`,
		`^(?P<other>DT-\d+)$`,
		`[invalid`,
	}
	for _, pattern := range invalid {
		if _, err := NewResolver(ResolverConfig{BranchPattern: pattern}); err == nil {
			t.Errorf("NewResolver(%q) succeeded, want error", pattern)
		}
	}

	resolver, err := NewResolver(ResolverConfig{
		TicketKey:     "DT",
		BranchPattern: `^work/(?P<ticket>DT-[1-9][0-9]*)/[a-z]+$`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := resolver.Resolve(ResolveInput{Branch: "work/DT-9/test"}); got.TicketID != "DT-9" {
		t.Fatalf("custom Resolve() = %#v", got)
	}
}

func TestExternalID(t *testing.T) {
	for _, tc := range []struct{ ref, platform, want string }{
		{"GH-42", "github", "42"},
		{"GL-18", "gitlab", "18"},
		{"ADO-456", "azure", "456"},
		{"PROJ-123", "jira", "PROJ-123"},
	} {
		if got := ExternalID(tc.ref, tc.platform); got != tc.want {
			t.Errorf("ExternalID(%q, %q) = %q, want %q", tc.ref, tc.platform, got, tc.want)
		}
	}
}
