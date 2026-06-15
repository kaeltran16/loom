package git

import (
	"context"
	"io"
)

// StubRunner is a Runner with canned output, for use by other packages' tests.
type StubRunner struct {
	Stdout []byte
	Stderr []byte
	Err    error
}

func (s *StubRunner) Run(_ context.Context, _ io.Reader, _ ...string) ([]byte, []byte, error) {
	return s.Stdout, s.Stderr, s.Err
}

// NewTestRepo builds a Repo over an arbitrary Runner. Test-only seam.
func NewTestRepo(r Runner) *Repo { return &Repo{runner: r} }
