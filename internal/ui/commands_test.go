package ui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kael02/loom/internal/git"
)

func TestRemoteOp_MarksCanceledWhenContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // simulate the user having aborted

	cmd := remoteOp(ctx, "git fetch", func() (string, error) {
		return "partial output", context.Canceled
	})
	msg, ok := cmd().(gitDoneMsg)
	if !ok {
		t.Fatalf("expected gitDoneMsg, got %T", cmd())
	}
	if !msg.canceled {
		t.Error("expected canceled=true when the op context was canceled")
	}
	if msg.err != nil {
		t.Errorf("a user-canceled op should not surface as an error, got %v", msg.err)
	}
	if msg.cmd != "git fetch" {
		t.Errorf("cmd label = %q, want git fetch", msg.cmd)
	}
}

func TestRemoteOp_NormalFailureKeepsError(t *testing.T) {
	ctx := context.Background() // not canceled
	cmd := remoteOp(ctx, "git push", func() (string, error) {
		return "rejected", errFake("exit status 1")
	})
	msg := cmd().(gitDoneMsg)
	if msg.canceled {
		t.Error("a normal failure should not be marked canceled")
	}
	if msg.err == nil {
		t.Error("a normal failure should keep its error")
	}
}

func TestLoadStatusCmd_returnsStatusLoadedMsg(t *testing.T) {
	// build a real Repo over a fake runner via the git package test helper.
	repo := git.NewTestRepo(&git.StubRunner{
		Stdout: []byte("# branch.head main\n1 M. N... 100644 100644 100644 1 2 a.go\n"),
	})
	cmd := loadStatus(context.Background(), repo)
	msg := cmd()
	got, ok := msg.(statusLoadedMsg)
	if !ok {
		t.Fatalf("want statusLoadedMsg, got %T", msg)
	}
	if got.branch.Name != "main" || len(got.files) != 1 {
		t.Errorf("payload wrong: %+v", got)
	}
}

func TestCommitCmd_setsNoticeWithHash(t *testing.T) {
	repo := git.NewTestRepo(&git.StubRunner{Stdout: []byte("a1b2c3d\n")})
	cmd := commit(context.Background(), repo, "feat: x", "feat: x")
	msg := cmd().(gitDoneMsg)
	if msg.err != nil {
		t.Fatalf("unexpected error: %v", msg.err)
	}
	if msg.notice != "Committed a1b2c3d feat: x" {
		t.Errorf("notice = %q, want the success line", msg.notice)
	}
	if msg.cmd != "git commit" {
		t.Errorf("cmd = %q, want git commit", msg.cmd)
	}
}

func TestLoadHeadMessageCmd_splitsSubjectBody(t *testing.T) {
	repo := git.NewTestRepo(&git.StubRunner{Stdout: []byte("feat: x\n\nwhy it matters\n")})
	msg := loadHeadMessage(context.Background(), repo)().(amendPrefillMsg)
	if msg.err != nil {
		t.Fatalf("unexpected error: %v", msg.err)
	}
	if msg.subject != "feat: x" || msg.body != "why it matters" {
		t.Errorf("prefill = (%q,%q), want (feat: x, why it matters)", msg.subject, msg.body)
	}
}

func TestLoadShowCmd_suppressesCommitPreamble(t *testing.T) {
	raw := strings.Join([]string{
		"commit 286ba6e741475ad0f4fdb68f1f5cd41935dd77bc",
		"Author: Kael <khang.tran@mozox.com>",
		"Date:   Tue Jun 16 16:52:36 2026 +0700",
		"",
		"    docs(spec): merge-conflict handling via editor delegation",
		"",
		"diff --git a/docs/spec.md b/docs/spec.md",
		"new file mode 100644",
		"index 0000000..2b088cf",
		"--- /dev/null",
		"+++ b/docs/spec.md",
		"@@ -0,0 +1 @@",
		"+# Merge Conflict Editor Delegation Design",
	}, "\n")
	repo := git.NewTestRepo(&git.StubRunner{Stdout: []byte(raw)})

	msg := loadShow(context.Background(), repo, "286ba6e", 7)().(diffLoadedMsg)

	if msg.seq != 7 {
		t.Fatalf("seq = %d, want 7", msg.seq)
	}
	if !strings.HasPrefix(msg.text, "diff --git a/docs/spec.md b/docs/spec.md") {
		t.Fatalf("show text should start at the diff, got:\n%s", msg.text)
	}
	for _, hidden := range []string{"commit 286ba6e", "Author:", "Date:", "docs(spec):"} {
		if strings.Contains(msg.text, hidden) {
			t.Fatalf("show text leaked preamble fragment %q:\n%s", hidden, msg.text)
		}
	}
}

func TestCommitAmendCmd_setsNotice(t *testing.T) {
	repo := git.NewTestRepo(&git.StubRunner{Stdout: []byte("deadbee\n")})
	msg := commitAmend(context.Background(), repo, "feat: x", "feat: x")().(gitDoneMsg)
	if msg.cmd != "git commit --amend" || msg.notice != "Committed deadbee feat: x" {
		t.Errorf("amend msg = %#v", msg)
	}
}

func TestMergeAbortCmd_label(t *testing.T) {
	repo := git.NewTestRepo(&git.StubRunner{})
	msg := mergeAbort(context.Background(), repo)().(gitDoneMsg)
	if msg.err != nil {
		t.Fatalf("unexpected error: %v", msg.err)
	}
	if msg.cmd != "git merge --abort" {
		t.Errorf("cmd = %q, want git merge --abort", msg.cmd)
	}
}

func TestEditorExecCmd_embedsEditorAndFile(t *testing.T) {
	c := editorExecCmd("code --wait", "/repo/a.go")
	joined := strings.Join(c.Args, " ")
	if !strings.Contains(joined, "code --wait") || !strings.Contains(joined, "a.go") {
		t.Errorf("args = %v, want editor and file embedded", c.Args)
	}
}

func TestLoadStashesCmd_returnsStashesLoadedMsg(t *testing.T) {
	repo := git.NewTestRepo(&git.StubRunner{Stdout: []byte("stash@{0}\x00On main: save\x001 minute ago\n")})

	msg := loadStashes(context.Background(), repo)()

	got, ok := msg.(stashesLoadedMsg)
	if !ok {
		t.Fatalf("want stashesLoadedMsg, got %T", msg)
	}
	if len(got.stashes) != 1 || got.stashes[0].Ref != "stash@{0}" {
		t.Fatalf("stashes = %#v", got.stashes)
	}
}

func TestLoadStashShowCmd_returnsDiffLoadedMsg(t *testing.T) {
	repo := git.NewTestRepo(&git.StubRunner{Stdout: []byte("diff --git a/a.go b/a.go\n+new\n")})

	msg := loadStashShow(context.Background(), repo, "stash@{0}", 9)().(stashShowLoadedMsg)

	if msg.seq != 9 {
		t.Fatalf("seq = %d, want 9", msg.seq)
	}
	if !strings.Contains(msg.text, "diff --git") {
		t.Fatalf("stash show text = %q", msg.text)
	}
}

func TestStashPushCmd_keepsOutputAndNotice(t *testing.T) {
	repo := git.NewTestRepo(&git.StubRunner{Stdout: []byte("Saved working directory and index state On main: save\n")})

	msg := stashPush(context.Background(), repo, "save")().(gitDoneMsg)

	if msg.cmd != "git stash push" {
		t.Fatalf("cmd = %q", msg.cmd)
	}
	if msg.output == "" {
		t.Fatal("expected stash output in command log")
	}
	if msg.notice != "Stashed save" {
		t.Fatalf("notice = %q", msg.notice)
	}
	if msg.err != nil {
		t.Fatalf("unexpected error: %v", msg.err)
	}
}

func TestStashActionCommandsKeepOutput(t *testing.T) {
	cases := []struct {
		name string
		cmd  tea.Cmd
		want string
	}{
		{"apply", stashApply(context.Background(), git.NewTestRepo(&git.StubRunner{Stdout: []byte("applied\n")}), "stash@{0}"), "git stash apply stash@{0}"},
		{"pop", stashPop(context.Background(), git.NewTestRepo(&git.StubRunner{Stdout: []byte("popped\n")}), "stash@{0}"), "git stash pop stash@{0}"},
		{"drop", stashDrop(context.Background(), git.NewTestRepo(&git.StubRunner{Stdout: []byte("dropped\n")}), "stash@{0}"), "git stash drop stash@{0}"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			msg := c.cmd().(gitDoneMsg)
			if msg.cmd != c.want {
				t.Fatalf("cmd = %q, want %q", msg.cmd, c.want)
			}
			if msg.output == "" {
				t.Fatal("expected output preserved")
			}
			if msg.err != nil {
				t.Fatalf("unexpected error: %v", msg.err)
			}
		})
	}
}

func TestLoadCommitAuthorsCmd_returnsCommitAuthorsLoadedMsg(t *testing.T) {
	repo := git.NewTestRepo(&git.StubRunner{Stdout: []byte("Kael\nAlex\n")})

	msg := loadCommitAuthors(context.Background(), repo, "main")()

	got, ok := msg.(commitAuthorsLoadedMsg)
	if !ok {
		t.Fatalf("want commitAuthorsLoadedMsg, got %T", msg)
	}
	if got.branch != "main" {
		t.Fatalf("branch = %q, want main", got.branch)
	}
	if strings.Join(got.authors, ",") != "Alex,Kael" {
		t.Fatalf("authors = %#v, want sorted authors", got.authors)
	}
}

func TestSearchCommitsCmd_returnsCommitSearchLoadedMsg(t *testing.T) {
	repo := git.NewTestRepo(&git.StubRunner{Stdout: []byte("abc123\x00fix auth\x00Kael\x002 hours ago\n")})
	q := git.CommitSearch{Query: "fix", Ref: "main", Author: "Kael", Limit: 50}

	msg := searchCommits(context.Background(), repo, q)()

	got, ok := msg.(commitSearchLoadedMsg)
	if !ok {
		t.Fatalf("want commitSearchLoadedMsg, got %T", msg)
	}
	if len(got.commits) != 1 || got.commits[0].Hash != "abc123" {
		t.Fatalf("commits = %#v", got.commits)
	}
	if got.summary != `Search: "fix" | branch main | author Kael` {
		t.Fatalf("summary = %q", got.summary)
	}
}

func TestCommitSearchSummaryOmitsAnyAuthorAndEmptyQuery(t *testing.T) {
	q := git.CommitSearch{Ref: "main", Author: "Any"}
	if got := commitSearchSummary(q); got != "Search: branch main" {
		t.Fatalf("summary = %q, want Search: branch main", got)
	}
}
