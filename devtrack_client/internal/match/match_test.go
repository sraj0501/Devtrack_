package match

import "testing"

func docs() []Doc {
	return []Doc{
		{ID: "#7", Title: "Improve dashboard loading performance", Body: "Charts render slowly on the metrics dashboard."},
		{ID: "#42", Title: "Fix login redirect loop", Body: "Users hit an infinite redirect after OAuth login."},
		{ID: "#13", Title: "Add dark mode toggle to settings", Body: "Theme preference in the settings panel."},
	}
}

func topID(sig Signal) string {
	r := Rank(sig, docs())
	if len(r) == 0 {
		return ""
	}
	return docs()[r[0].Index].ID
}

func TestRankByTokens(t *testing.T) {
	sig := Signal{
		Branch:  "feat/login-redirect",
		Subject: "fix oauth login redirect loop",
		Files:   []string{"auth/login.go", "auth/oauth.go"},
	}
	if got := topID(sig); got != "#42" {
		t.Errorf("expected #42 as top match, got %s", got)
	}
}

func TestBranchNumberDominates(t *testing.T) {
	// Subject talks about dashboards, but the branch names ticket 42 explicitly.
	sig := Signal{
		Branch:  "bugfix/issue-42",
		Subject: "tweak dashboard chart spacing",
		Files:   []string{"ui/dashboard.go"},
	}
	r := Rank(sig, docs())
	if docs()[r[0].Index].ID != "#42" {
		t.Fatalf("branch-number match should win, got %s", docs()[r[0].Index].ID)
	}
	if r[0].Score < numberMatchScore {
		t.Errorf("branch-number match score = %.3f, want >= %.3f", r[0].Score, numberMatchScore)
	}
}

func TestBranchNumbers(t *testing.T) {
	got := branchNumbers("feature/PROJ-42-login-v2")
	want := map[int]bool{42: true, 2: true}
	if len(got) != 2 {
		t.Fatalf("branchNumbers = %v, want two values", got)
	}
	for _, n := range got {
		if !want[n] {
			t.Errorf("unexpected branch number %d", n)
		}
	}
}

func TestDocNumber(t *testing.T) {
	cases := map[string]int{"#42": 42, "AB#7": 7, "#0": 0, "none": 0, "": 0}
	for in, want := range cases {
		if got := docNumber(in); got != want {
			t.Errorf("docNumber(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestJaroWinkler(t *testing.T) {
	if jaroWinkler("login", "login") != 1 {
		t.Error("identical strings should score 1")
	}
	if got := jaroWinkler("login", "logni"); got <= 0.8 {
		t.Errorf("near-identical strings scored too low: %.3f", got)
	}
	if got := jaroWinkler("login", "zzzzz"); got > 0.4 {
		t.Errorf("dissimilar strings scored too high: %.3f", got)
	}
}

// stubEmbedder maps each known text to a fixed vector so hybrid blending is
// deterministic and testable without a network.
type stubEmbedder struct{ vecs map[string][]float64 }

func (s stubEmbedder) Embed(text string) ([]float64, error) {
	if v, ok := s.vecs[text]; ok {
		return v, nil
	}
	return []float64{0, 0, 1}, nil
}

func TestRankHybridBlends(t *testing.T) {
	ds := docs()
	sig := Signal{Branch: "feat/perf", Subject: "speed up rendering", Files: []string{"perf.go"}}

	// Semantically align the query with the dashboard-performance ticket (#7).
	emb := stubEmbedder{vecs: map[string][]float64{
		queryText(sig):              {1, 0, 0},
		ds[0].Title + "\n" + ds[0].Body: {1, 0, 0}, // #7 perfect semantic match
	}}

	r := RankHybrid(sig, ds, emb)
	if ds[r[0].Index].ID != "#7" {
		t.Errorf("hybrid should rank #7 first via semantic signal, got %s", ds[r[0].Index].ID)
	}
	if r[0].Semantic <= 0 {
		t.Errorf("expected a positive semantic component, got %.3f", r[0].Semantic)
	}
}
