package azure

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// newTestClient creates a Client whose baseURL points at the given test server.
// It sets a dummy PAT directly on the struct to bypass the env-var check in NewClient.
func newTestClient(t *testing.T, serverURL string) *Client {
	t.Helper()
	parsed, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	return &Client{
		pat:     "test-pat",
		org:     "testorg",
		project: "testproject",
		baseURL: strings.TrimRight(parsed.Scheme+"://"+parsed.Host, "/"),
		http:    &http.Client{},
	}
}

func TestListPRReviewers_URLConstruction(t *testing.T) {
	var capturedPath string

	handler := func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.RequestURI()
		resp := prSearchResponse{
			Count: 1,
			Value: []prSearchItem{
				{
					PullRequestID: 42,
					Reviewers: []PRReviewer{
						{DisplayName: "Alice", Vote: 10},
						{DisplayName: "Bob", Vote: 0},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}

	srv := httptest.NewServer(http.HandlerFunc(handler))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	reviewers, err := client.ListPRReviewers(42)
	if err != nil {
		t.Fatalf("ListPRReviewers: unexpected error: %v", err)
	}

	// Verify the correct URL path was called.
	wantPath := "/testorg/testproject/_apis/git/pullrequests?pullRequestId=42&api-version=7.0"
	if capturedPath != wantPath {
		t.Errorf("URL path: got %q, want %q", capturedPath, wantPath)
	}

	if len(reviewers) != 2 {
		t.Fatalf("reviewers count: got %d, want 2", len(reviewers))
	}
	if reviewers[0].DisplayName != "Alice" || reviewers[0].Vote != 10 {
		t.Errorf("reviewer[0]: got {%q, %d}, want {Alice, 10}", reviewers[0].DisplayName, reviewers[0].Vote)
	}
}

func TestIsPRApproved_Approved(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		resp := prSearchResponse{
			Count: 1,
			Value: []prSearchItem{
				{
					PullRequestID: 7,
					Reviewers: []PRReviewer{
						{DisplayName: "Alice", Vote: 10},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}

	srv := httptest.NewServer(http.HandlerFunc(handler))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	reviewers, err := client.ListPRReviewers(7)
	if err != nil {
		t.Fatalf("ListPRReviewers: %v", err)
	}

	approved := false
	for _, r := range reviewers {
		if r.Vote >= 10 {
			approved = true
		}
	}
	if !approved {
		t.Error("expected IsPRApproved logic to return true when a reviewer has vote=10")
	}
}

func TestIsPRApproved_NotApproved(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		resp := prSearchResponse{
			Count: 1,
			Value: []prSearchItem{
				{
					PullRequestID: 8,
					Reviewers: []PRReviewer{
						{DisplayName: "Bob", Vote: 0},
						{DisplayName: "Carol", Vote: -5},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}

	srv := httptest.NewServer(http.HandlerFunc(handler))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	reviewers, err := client.ListPRReviewers(8)
	if err != nil {
		t.Fatalf("ListPRReviewers: %v", err)
	}

	for _, r := range reviewers {
		if r.Vote >= 10 {
			t.Errorf("expected no approved reviewer but found vote=%d for %s", r.Vote, r.DisplayName)
		}
	}
}

func TestIsPRApproved_PRNotFound(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		resp := prSearchResponse{
			Count: 0,
			Value: []prSearchItem{},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}

	srv := httptest.NewServer(http.HandlerFunc(handler))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	_, err := client.ListPRReviewers(999)
	if err == nil {
		t.Error("expected an error when PR is not found (count=0), got nil")
	}
	if !strings.Contains(err.Error(), "PR not found") {
		t.Errorf("expected 'PR not found' in error, got: %v", err)
	}
}
