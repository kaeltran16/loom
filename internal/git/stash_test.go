package git

import "testing"

func TestParseStashes(t *testing.T) {
	in := []byte("stash@{0}\x00WIP on main: abc1234 parser cleanup\x0012 minutes ago\nstash@{1}\x00On feature/login: before auth refactor\x00yesterday\n")

	got := parseStashes(in)

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2: %#v", len(got), got)
	}
	if got[0] != (Stash{
		Ref:     "stash@{0}",
		Index:   0,
		Message: "WIP on main: abc1234 parser cleanup",
		Branch:  "main",
		Age:     "12 minutes ago",
	}) {
		t.Fatalf("stash[0] = %#v", got[0])
	}
	if got[1] != (Stash{
		Ref:     "stash@{1}",
		Index:   1,
		Message: "On feature/login: before auth refactor",
		Branch:  "feature/login",
		Age:     "yesterday",
	}) {
		t.Fatalf("stash[1] = %#v", got[1])
	}
}

func TestParseStashesSkipsMalformedRows(t *testing.T) {
	in := []byte("stash@{0}\x00On main: good\x001 hour ago\nbad row without separators\nstash@{abc}\x00On main: bad index\x00now\n")

	got := parseStashes(in)

	if len(got) != 1 {
		t.Fatalf("len = %d, want 1: %#v", len(got), got)
	}
	if got[0].Ref != "stash@{0}" || got[0].Index != 0 || got[0].Branch != "main" {
		t.Fatalf("parsed stash = %#v", got[0])
	}
}

func TestStashBranchFromSubjectIsBestEffort(t *testing.T) {
	cases := map[string]string{
		"WIP on main: abc1234 work":        "main",
		"On feature/ui: save before pull":  "feature/ui",
		"custom hand-written stash title":  "",
		"WIP on release/v1.2: deadbee fix": "release/v1.2",
	}

	for subject, want := range cases {
		if got := stashBranchFromSubject(subject); got != want {
			t.Errorf("stashBranchFromSubject(%q) = %q, want %q", subject, got, want)
		}
	}
}
