// Package match ranks tickets by how likely they relate to an in-progress
// commit. It scores a Signal (branch, commit subject, staged files) against
// each candidate ticket using offline fuzzy similarity, and optionally blends
// in semantic similarity when an Embedder is available (hybrid mode).
package match

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// Signal is what we know about the commit being prepared.
type Signal struct {
	Branch  string
	Subject string
	Files   []string
	Refs    []int // explicit ticket numbers parsed from the commit message
}

// Doc is a candidate ticket reduced to its searchable text.
type Doc struct {
	ID     string // display id, e.g. "#42" or "AB#7"
	Title  string
	Body   string
	Labels string
}

// Result is a scored candidate. Score is in [0,1]; higher is more likely.
type Result struct {
	Index    int     // index into the input docs slice
	Score    float64 // final blended score
	Fuzzy    float64 // offline component
	Semantic float64 // embedding component (0 when not used)
}

// embedTopK bounds how many top fuzzy candidates get embedded in hybrid mode,
// keeping commit-time latency predictable regardless of ticket count.
const embedTopK = 8

// numberMatchScore is the floor applied when the branch references a ticket's
// exact number — the strongest signal a developer gives us.
const numberMatchScore = 0.97

// Rank scores docs against sig using offline fuzzy similarity only, returning
// results sorted by descending score.
func Rank(sig Signal, docs []Doc) []Result {
	q := tokenize(queryText(sig))
	// An explicit ref in the message is as strong as a branch-number match.
	bn := append(branchNumbers(sig.Branch), sig.Refs...)
	subj := strings.ToLower(sig.Subject)

	results := make([]Result, len(docs))
	for i, d := range docs {
		f := fuzzyScore(q, subj, bn, d)
		results[i] = Result{Index: i, Score: f, Fuzzy: f}
	}
	sortResults(results)
	return results
}

// RankHybrid blends fuzzy and semantic similarity. When e is nil or embedding
// fails it degrades to pure fuzzy ranking. Only the top fuzzy candidates are
// embedded to bound latency.
func RankHybrid(sig Signal, docs []Doc, e Embedder) []Result {
	base := Rank(sig, docs)
	if e == nil || len(base) == 0 {
		return base
	}

	qv, err := e.Embed(queryText(sig))
	if err != nil || len(qv) == 0 {
		return base
	}

	limit := embedTopK
	if limit > len(base) {
		limit = len(base)
	}
	for i := 0; i < limit; i++ {
		// A branch-number match already dominates; don't dilute it.
		if base[i].Fuzzy >= numberMatchScore {
			continue
		}
		d := docs[base[i].Index]
		dv, err := e.Embed(d.Title + "\n" + d.Body)
		if err != nil || len(dv) == 0 {
			continue
		}
		sem := cosine(qv, dv)
		base[i].Semantic = sem
		base[i].Score = 0.5*base[i].Fuzzy + 0.5*sem
	}
	sortResults(base)
	return base
}

// fuzzyScore combines token-set cosine similarity with a Jaro-Winkler subject↔
// title comparison, and floors the result on an exact branch-number match.
func fuzzyScore(q map[string]int, subject string, branchNums []int, d Doc) float64 {
	// Title is weighted by inclusion twice so it dominates the body.
	dt := tokenize(d.Title + " " + d.Title + " " + d.Body + " " + d.Labels)
	tc := tokenCosine(q, dt)
	jw := jaroWinkler(subject, strings.ToLower(d.Title))
	f := 0.65*tc + 0.35*jw

	if n := docNumber(d.ID); n > 0 {
		for _, bn := range branchNums {
			if bn == n {
				if f < numberMatchScore {
					f = numberMatchScore
				}
				break
			}
		}
	}
	if f > 1 {
		f = 1
	}
	return f
}

func queryText(sig Signal) string {
	return sig.Branch + "\n" + sig.Subject + "\n" + strings.Join(sig.Files, "\n")
}

func sortResults(r []Result) {
	sort.SliceStable(r, func(i, j int) bool { return r[i].Score > r[j].Score })
}

// --- tokenisation ---

var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "this": true,
	"that": true, "from": true, "into": true, "your": true, "you": true,
	"add": true, "fix": true, "update": true, "feat": true, "chore": true,
	"refactor": true, "wip": true, "main": true, "master": true, "dev": true,
}

// tokenize lowercases, splits on non-alphanumeric runes, and drops stopwords
// and 1-char tokens. Returns term frequencies.
func tokenize(s string) map[string]int {
	m := make(map[string]int)
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for _, f := range fields {
		if len(f) < 2 || stopWords[f] {
			continue
		}
		m[f]++
	}
	return m
}

func tokenCosine(a, b map[string]int) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	var dot, na, nb float64
	for t, ca := range a {
		na += float64(ca * ca)
		if cb, ok := b[t]; ok {
			dot += float64(ca * cb)
		}
	}
	for _, cb := range b {
		nb += float64(cb * cb)
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// branchNumbers extracts standalone digit-run tokens from a branch name, e.g.
// "feature/PROJ-42-login" → [42].
func branchNumbers(branch string) []int {
	var nums []int
	for _, f := range strings.FieldsFunc(branch, func(r rune) bool {
		return !unicode.IsDigit(r)
	}) {
		if n, err := strconv.Atoi(f); err == nil && n > 0 {
			nums = append(nums, n)
		}
	}
	return nums
}

// docNumber pulls the trailing integer out of a display id like "#42"/"AB#7".
func docNumber(id string) int {
	var digits strings.Builder
	for _, r := range id {
		if unicode.IsDigit(r) {
			digits.WriteRune(r)
		} else {
			digits.Reset()
		}
	}
	n, _ := strconv.Atoi(digits.String())
	return n
}

// --- semantic helpers ---

func cosine(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// --- Jaro-Winkler ---

func jaroWinkler(a, b string) float64 {
	j := jaro(a, b)
	if j == 0 {
		return 0
	}
	prefix := 0
	for prefix < len(a) && prefix < len(b) && prefix < 4 && a[prefix] == b[prefix] {
		prefix++
	}
	return j + float64(prefix)*0.1*(1-j)
}

func jaro(s1, s2 string) float64 {
	if s1 == s2 {
		if s1 == "" {
			return 0
		}
		return 1
	}
	l1, l2 := len(s1), len(s2)
	if l1 == 0 || l2 == 0 {
		return 0
	}
	maxDist := l1
	if l2 > maxDist {
		maxDist = l2
	}
	maxDist = maxDist/2 - 1
	if maxDist < 0 {
		maxDist = 0
	}

	s1m := make([]bool, l1)
	s2m := make([]bool, l2)
	matches := 0
	for i := 0; i < l1; i++ {
		start := i - maxDist
		if start < 0 {
			start = 0
		}
		end := i + maxDist + 1
		if end > l2 {
			end = l2
		}
		for k := start; k < end; k++ {
			if s2m[k] || s1[i] != s2[k] {
				continue
			}
			s1m[i] = true
			s2m[k] = true
			matches++
			break
		}
	}
	if matches == 0 {
		return 0
	}

	t := 0.0
	k := 0
	for i := 0; i < l1; i++ {
		if !s1m[i] {
			continue
		}
		for !s2m[k] {
			k++
		}
		if s1[i] != s2[k] {
			t++
		}
		k++
	}
	t /= 2

	m := float64(matches)
	return (m/float64(l1) + m/float64(l2) + (m-t)/m) / 3
}
