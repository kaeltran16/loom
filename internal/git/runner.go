// Package git is loom's access layer over the real git binary. It runs git
// through the Runner seam and parses output into typed structs. It knows
// nothing about the UI.
package git

import (
	"bytes"
	"context"
	"io"
	"os/exec"
)

// Runner is the single seam to the outside world. Production uses execRunner;
// tests inject a fake. stdin is nil for most commands; it carries the commit
// message for `git commit -F -`.
type Runner interface {
	Run(ctx context.Context, stdin io.Reader, args ...string) (stdout, stderr []byte, err error)
}

type execRunner struct {
	dir string // working directory; "" means the current process dir
}

// NewExecRunner returns a Runner that shells out to the git binary in dir.
func NewExecRunner(dir string) Runner {
	return &execRunner{dir: dir}
}

func (e *execRunner) Run(ctx context.Context, stdin io.Reader, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = e.dir
	cmd.Stdin = stdin
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	return out.Bytes(), errb.Bytes(), err
}
