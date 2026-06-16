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
