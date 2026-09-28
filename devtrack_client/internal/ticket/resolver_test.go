package ticket

import (
	"strings"
	"testing"
)

func resolver(t *testing.T, provider string) *Resolver {
	t.Helper()
	r, err := NewResolver(Contract{Key: "PROJ", Provider: provider})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestCanonicalBranchGrammar(t *testing.T) {
	r := resolver(t, "jira")
	for _, kind := range strings.Split(DefaultKinds, "|") {
		if got := r.BranchReference(kind + "/PROJ-12-login-v2"); got != "PROJ-12" {
			t.Errorf("kind %s: %q", kind, got)
		}
	}
	for _, branch := range []string{"main", "PROJ-12", "feature/PROJ-12", "feature/PROJ-0-login", "feature/PROJ-01-login", "feature/proj-12-login", "feature/OTHER-12-login", "prefix/feature/PROJ-12-login", "feature/PROJ-12-Login", "feature/PROJ-12-a--b", "feature/PROJ-12-a/../b", "feature/PROJ-12-a\n", "release/PROJ-12-login", "feature/PROJ-12-a_thing", "feature/PROJ-12--a"} {
		if got := r.BranchReference(branch); got != "" {
			t.Errorf("invalid branch %q mapped to %q", branch, got)
		}
	}
}

func TestTicketContractValidation(t *testing.T) {
	for _, pattern := range []string{`(?P<ticket>PROJ-\d+)`, `^(PROJ-\d+)$`, `^(?P<ticket>PROJ-\d+)|x$`, `(?m)^(?P<ticket>PROJ-\d+)$`, `^(?P<ticket>PROJ-\d+)(?P<ticket>x)$`, `[broken`} {
		if _, err := NewResolver(Contract{Key: "PROJ", Pattern: pattern}); err == nil {
			t.Errorf("accepted invalid pattern %q", pattern)
		}
	}
	for _, key := range []string{"", "proj", "A-B", "2PROJ", "PROJ|OTHER"} {
		if _, err := NewResolver(Contract{Key: key}); err == nil {
			t.Errorf("accepted invalid key %q", key)
		}
	}
	r, err := NewResolver(Contract{Key: "PROJ", Pattern: `^work/(?P<ticket>PROJ-[1-9][0-9]*)/[a-z]+$`})
	if err != nil {
		t.Fatal(err)
	}
	if r.BranchReference("work/PROJ-42/login") != "PROJ-42" || r.BranchReference("feat/PROJ-42-login") != "" {
		t.Fatal("custom grammar was not exclusive")
	}
}

func TestDeterministicPrecedenceAndConflict(t *testing.T) {
	r := resolver(t, "jira")
	tests := []struct {
		name              string
		evidence          Evidence
		id, source, state string
		conflict          bool
	}{
		{"branch", Evidence{Branch: "feat/PROJ-1-login"}, "PROJ-1", "branch", "linked", false},
		{"branch beats prefix", Evidence{Branch: "feat/PROJ-1-login", Message: "PROJ-2: login", ActiveTicket: "PROJ-3"}, "PROJ-1", "branch", "conflict", true},
		{"incidental prose ignored", Evidence{Branch: "feat/no-ticket", Message: "fix bug PROJ-2"}, "", "none", "unlinked", false},
		{"incidental branch ignored", Evidence{Branch: "other/PROJ-2-extra"}, "", "none", "unlinked", false},
		{"prefix", Evidence{Message: "PROJ-2: login"}, "PROJ-2", "commit_reference", "linked", false},
		{"trailer", Evidence{Message: "login\n\nRefs: PROJ-2"}, "PROJ-2", "commit_reference", "linked", false},
		{"duplicate evidence", Evidence{Message: "PROJ-2: login\n\nRefs: PROJ-2", ActiveTicket: "PROJ-2"}, "PROJ-2", "commit_reference", "linked", false},
		{"ambiguous prefix trailer", Evidence{Message: "PROJ-2: login\n\nRefs: PROJ-3", ActiveTicket: "PROJ-4"}, "", "commit_reference", "unlinked", true},
		{"ambiguous trailers", Evidence{Message: "login\n\nRefs: PROJ-2, PROJ-3"}, "", "commit_reference", "unlinked", true},
		{"active", Evidence{Message: "incidental PROJ-2 prose", ActiveTicket: "PROJ-4"}, "PROJ-4", "active_override", "linked", false},
		{"prefix beats active", Evidence{Message: "PROJ-2: login", ActiveTicket: "PROJ-4"}, "PROJ-2", "commit_reference", "conflict", true},
		{"body refs ignored", Evidence{Message: "login\n\nRefs: PROJ-2\n\nThis is prose."}, "", "none", "unlinked", false},
		{"merge branch", Evidence{Branch: "main", MergeToDefault: true, Message: "Merge branch 'fix/PROJ-2-login'"}, "PROJ-2", "merge_branch", "linked", false},
		{"merge github", Evidence{Branch: "main", MergeToDefault: true, Message: "Merge pull request #3 from owner/fix/PROJ-2-login"}, "PROJ-2", "merge_branch", "linked", false},
		{"merge other branch ignored", Evidence{Branch: "feature/no-ticket", Message: "Merge branch 'fix/PROJ-2-login'"}, "", "none", "unlinked", false},
		{"merge ambiguous", Evidence{Branch: "main", MergeToDefault: true, Message: "Merge branch 'fix/PROJ-2-login' and 'fix/PROJ-3-other'"}, "", "merge_branch", "unlinked", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.Resolve(tt.evidence)
			if got.TicketID != tt.id || got.Source != tt.source || got.State != tt.state || got.Conflict != tt.conflict {
				t.Fatalf("resolution=%+v", got)
			}
			if got.TicketID == "" && got.Confidence != 0 {
				t.Fatal("unlinked mapping has confidence")
			}
		})
	}
}

func TestProviderExternalIdentity(t *testing.T) {
	for _, provider := range []string{"github", "gitlab", "azure", "jira", "none", ""} {
		r := resolver(t, provider)
		want := "42"
		if provider == "jira" || provider == "none" || provider == "" {
			want = "PROJ-42"
		}
		if got := r.Resolve(Evidence{Branch: "fix/PROJ-42-login"}); got.TicketID != "PROJ-42" || got.ExternalID != want {
			t.Errorf("provider %s: %+v", provider, got)
		}
		if r.ExternalID("OTHER-42") != "" {
			t.Fatal("foreign namespace accepted")
		}
	}
}

func FuzzResolverNeverInventsIdentity(f *testing.F) {
	f.Add("feat/PROJ-1-login", "PROJ-2: contradictory", "PROJ-3")
	f.Add("main", "incidental PROJ-9", "")
	r, _ := NewResolver(Contract{Key: "PROJ", Provider: "github"})
	f.Fuzz(func(t *testing.T, branch, message, active string) {
		got := r.Resolve(Evidence{Branch: branch, Message: message, ActiveTicket: active})
		if got.TicketID != "" && (!r.ValidReference(got.TicketID) || got.ExternalID != strings.TrimPrefix(got.TicketID, "PROJ-")) {
			t.Fatalf("invalid identity %+v", got)
		}
		if ref := r.BranchReference(branch); ref != "" && ref != got.TicketID {
			t.Fatal("branch precedence violated")
		}
	})
}
