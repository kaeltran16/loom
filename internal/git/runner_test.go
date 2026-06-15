package git

import (
	"context"
	"io"
	"strings"
	"testing"
)

// execRunner is exercised against the real git binary as a smoke test:
// if this fails, git is not on PATH and nothing else in loom can work.
func TestExecRunner_GitVersion(t *testing.T) {
	r := NewExecRunner("")
	stdout, _, err := r.Run(context.Background(), nil, "--version")
	if err != nil {
		t.Fatalf("running `git --version`: %v", err)
	}
	if !strings.HasPrefix(string(stdout), "git version") {
		t.Fatalf("unexpected output: %q", stdout)
	}
}

// fakeRunner is the seam every git-layer test below relies on. It records the
// args (and stdin) it was called with and returns canned output.
type fakeRunner struct {
	stdout  []byte
	stderr  []byte
	err     error
	gotArgs []string
	gotIn   []byte
	calls   [][]string // every call's args, in order — for methods that run more than one command
}

func (f *fakeRunner) Run(_ context.Context, stdin io.Reader, args ...string) ([]byte, []byte, error) {
	f.gotArgs = args
	f.calls = append(f.calls, args)
	if stdin != nil {
		f.gotIn, _ = io.ReadAll(stdin)
	}
	return f.stdout, f.stderr, f.err
}
