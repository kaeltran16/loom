package ui

import (
	"context"
	"testing"

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
