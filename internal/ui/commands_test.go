package ui

import (
	"context"
	"testing"

	"github.com/kael02/loom/internal/git"
)

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
