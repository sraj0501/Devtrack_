package daemon

import (
	"errors"
	"reflect"
	"testing"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
)

type fakeFirstRunVoiceClient struct {
	statusResponses []*trigger.VoiceStatusResponse
	statusCalls     int
	seedRequests    []trigger.VoiceSeedRequest
	seedError       error
	profilePaths    []string
}

func (f *fakeFirstRunVoiceClient) Ping() bool { return true }

func (f *fakeFirstRunVoiceClient) VoiceSeed(req trigger.VoiceSeedRequest) (*trigger.VoiceSeedResponse, error) {
	f.seedRequests = append(f.seedRequests, req)
	if f.seedError != nil {
		return nil, f.seedError
	}
	return &trigger.VoiceSeedResponse{Embedded: 3, RepoPath: req.RepoPath}, nil
}

func (f *fakeFirstRunVoiceClient) VoiceProfileGenerate(paths []string) (string, int, error) {
	f.profilePaths = append([]string(nil), paths...)
	return "/data/learning/profile.md", 245, nil
}

func (f *fakeFirstRunVoiceClient) VoiceStatus() (*trigger.VoiceStatusResponse, error) {
	index := f.statusCalls
	f.statusCalls++
	if index >= len(f.statusResponses) {
		return nil, errors.New("status unavailable")
	}
	return f.statusResponses[index], nil
}

func TestBuildFirstRunProfileSeedsEveryRepoAndUsesCorpusCount(t *testing.T) {
	client := &fakeFirstRunVoiceClient{statusResponses: []*trigger.VoiceStatusResponse{
		{ProfileExists: false},
		{BySource: map[string]int{"git_history": 27}},
	}}
	paths := []string{"/repo/one", "/repo/two"}

	result, err := buildFirstRunProfile(client, paths, 6)
	if err != nil {
		t.Fatal(err)
	}
	if result.CommitCount != 27 || result.WordCount != 245 || result.ProfilePath != "/data/learning/profile.md" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(client.seedRequests) != 2 {
		t.Fatalf("seed requests = %d, want 2", len(client.seedRequests))
	}
	for i, request := range client.seedRequests {
		if request.RepoPath != paths[i] || request.SinceMonths != 6 || request.Force {
			t.Fatalf("seed request %d = %+v", i, request)
		}
	}
	if !reflect.DeepEqual(client.profilePaths, paths) {
		t.Fatalf("profile paths = %#v, want %#v", client.profilePaths, paths)
	}
}

func TestBuildFirstRunProfileReusesExistingProfile(t *testing.T) {
	client := &fakeFirstRunVoiceClient{statusResponses: []*trigger.VoiceStatusResponse{{
		ProfileExists: true, ProfileWordCount: 310,
		BySource: map[string]int{"git_history": 81},
	}}}

	result, err := buildFirstRunProfile(client, []string{"/repo"}, 6)
	if err != nil {
		t.Fatal(err)
	}
	if result.CommitCount != 81 || result.WordCount != 310 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(client.seedRequests) != 0 || len(client.profilePaths) != 0 {
		t.Fatal("existing profile triggered unnecessary seed or generation")
	}
}

func TestBuildFirstRunProfileFailureIsReturnedWithoutProfileGeneration(t *testing.T) {
	client := &fakeFirstRunVoiceClient{
		statusResponses: []*trigger.VoiceStatusResponse{{ProfileExists: false}},
		seedError:       errors.New("server dependency unavailable"),
	}

	if _, err := buildFirstRunProfile(client, []string{"/repo"}, 6); err == nil {
		t.Fatal("expected seed failure")
	}
	if len(client.profilePaths) != 0 {
		t.Fatal("profile generation ran after seed failure")
	}
}

func TestBuildFirstRunProfileRequiresLocalWorkspace(t *testing.T) {
	client := &fakeFirstRunVoiceClient{statusResponses: []*trigger.VoiceStatusResponse{{ProfileExists: false}}}
	if _, err := buildFirstRunProfile(client, nil, 6); err == nil {
		t.Fatal("expected missing-workspace error")
	}
}
