package git

import "testing"

func TestParseBranches(t *testing.T) {
	// fields are NUL-separated; "*" in the HEAD column marks the current branch.
	in := []byte("main\x00origin/main\x00*\nfeat/login\x00\x00\n")
	got := parseBranches(in)
	if len(got) != 2 {
		t.Fatalf("want 2 branches, got %d", len(got))
	}
	if got[0].Name != "main" || got[0].Upstream != "origin/main" || !got[0].Current {
		t.Errorf("branch[0] wrong: %+v", got[0])
	}
	if got[1].Name != "feat/login" || got[1].Current {
		t.Errorf("branch[1] wrong: %+v", got[1])
	}
}
