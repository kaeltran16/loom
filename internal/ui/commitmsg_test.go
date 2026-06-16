package ui

import "testing"

func TestSubjectLevelThresholds(t *testing.T) {
	cases := []struct {
		n    int
		want commitLevel
	}{
		{0, levelOK}, {50, levelOK}, {51, levelWarn}, {72, levelWarn}, {73, levelOver},
	}
	for _, c := range cases {
		if got := subjectLevel(c.n); got != c.want {
			t.Errorf("subjectLevel(%d) = %d, want %d", c.n, got, c.want)
		}
	}
}

func TestBuildCommitMessage(t *testing.T) {
	cases := []struct {
		name, subject, body, want string
	}{
		{"subject only", "feat: x", "", "feat: x"},
		{"subject and body", "feat: x", "why it matters", "feat: x\n\nwhy it matters"},
		{"trims", "  feat: x  ", "  body  ", "feat: x\n\nbody"},
		{"blank body omitted", "feat: x", "   ", "feat: x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := buildCommitMessage(c.subject, c.body); got != c.want {
				t.Errorf("buildCommitMessage = %q, want %q", got, c.want)
			}
		})
	}
}

func TestSplitCommitMessage(t *testing.T) {
	cases := []struct {
		name, full, wantSub, wantBody string
	}{
		{"subject only", "feat: x", "feat: x", ""},
		{"subject blank body", "feat: x\n\nwhy", "feat: x", "why"},
		{"single newline", "feat: x\nwhy", "feat: x", "why"},
		{"crlf", "feat: x\r\n\r\nwhy", "feat: x", "why"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sub, body := splitCommitMessage(c.full)
			if sub != c.wantSub || body != c.wantBody {
				t.Errorf("splitCommitMessage(%q) = (%q,%q), want (%q,%q)", c.full, sub, body, c.wantSub, c.wantBody)
			}
		})
	}
}

func TestNoticeText(t *testing.T) {
	if got := noticeText("a1b2c3d", "feat: x"); got != "Committed a1b2c3d feat: x" {
		t.Errorf("noticeText = %q", got)
	}
	if got := noticeText("", "feat: x"); got != "Committed feat: x" {
		t.Errorf("noticeText empty hash = %q", got)
	}
}
