package git

import "testing"

func TestParseLog(t *testing.T) {
	in := []byte("a1b2c3\x00fix nav\x00Kael\x002 hours ago\nd4e5f6\x00init\x00Kael\x003 days ago\n")
	got := parseLog(in)
	if len(got) != 2 {
		t.Fatalf("want 2 commits, got %d", len(got))
	}
	if got[0].Hash != "a1b2c3" || got[0].Subject != "fix nav" || got[0].Author != "Kael" || got[0].RelTime != "2 hours ago" {
		t.Errorf("commit[0] wrong: %+v", got[0])
	}
}
