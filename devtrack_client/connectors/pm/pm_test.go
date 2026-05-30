package pm

import "testing"

func TestParseOwnerRepo(t *testing.T) {
	cases := map[string]string{
		"https://github.com/sraj0501/Devtrack_.git":     "sraj0501/Devtrack_",
		"https://github.com/sraj0501/Devtrack_":         "sraj0501/Devtrack_",
		"git@github.com:sraj0501/Devtrack_.git":         "sraj0501/Devtrack_",
		"git@gitlab.com:group/sub/project.git":          "group/sub/project",
		"https://gitlab.example.com/group/sub/proj.git": "group/sub/proj",
		"ssh://git@gitlab.com/group/project.git":        "group/project",
		"":                                              "",
		"not-a-url":                                     "",
	}
	for in, want := range cases {
		if got := parseOwnerRepo(in); got != want {
			t.Errorf("parseOwnerRepo(%q) = %q, want %q", in, got, want)
		}
	}
}
