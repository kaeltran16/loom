package git

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Repo is the high-level git API the UI calls. It never exposes command strings.
type Repo struct {
	runner Runner
	root   string
}

// Open finds the repository root for dir and returns a Repo bound to it.
// Returns an error if dir is not inside a git working tree.
func Open(ctx context.Context, dir string) (*Repo, error) {
	r := NewExecRunner(dir)
	out, errb, err := r.Run(ctx, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %s", strings.TrimSpace(string(errb)))
	}
	root := strings.TrimSpace(string(out))
	return &Repo{runner: NewExecRunner(root), root: root}, nil
}

// Root returns the absolute path of the repository root.
func (r *Repo) Root() string { return r.root }

func (r *Repo) Status(ctx context.Context) ([]FileStatus, BranchInfo, error) {
	out, errb, err := r.runner.Run(ctx, nil, "status", "--porcelain=v2", "--branch")
	if err != nil {
		return nil, BranchInfo{}, fmt.Errorf("git status: %w: %s", err, errb)
	}
	files, br, perr := parseStatus(out)
	return files, br, perr
}

func (r *Repo) Branches(ctx context.Context) ([]Branch, error) {
	out, errb, err := r.runner.Run(ctx, nil,
		"for-each-ref", "--format=%(refname:short)%00%(upstream:short)%00%(HEAD)", "refs/heads")
	if err != nil {
		return nil, fmt.Errorf("git for-each-ref: %w: %s", err, errb)
	}
	return parseBranches(out), nil
}

func (r *Repo) Log(ctx context.Context, ref string, n int) ([]Commit, error) {
	args := []string{"log", "--format=%H%x00%s%x00%an%x00%ar", "-n", strconv.Itoa(n)}
	if ref != "" {
		args = append(args, ref)
	}
	out, errb, err := r.runner.Run(ctx, nil, args...)
	if err != nil {
		return nil, fmt.Errorf("git log: %w: %s", err, errb)
	}
	return parseLog(out), nil
}

func (r *Repo) Diff(ctx context.Context, path string, staged bool) (string, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--cached")
	}
	args = append(args, "--", path)
	out, errb, err := r.runner.Run(ctx, nil, args...)
	if err != nil {
		return "", fmt.Errorf("git diff: %w: %s", err, errb)
	}
	return string(out), nil
}

func (r *Repo) Show(ctx context.Context, hash string) (string, error) {
	out, errb, err := r.runner.Run(ctx, nil, "show", hash)
	if err != nil {
		return "", fmt.Errorf("git show: %w: %s", err, errb)
	}
	return string(out), nil
}

func (r *Repo) Stage(ctx context.Context, path string) error {
	return r.mutate(ctx, nil, "add", "--", path)
}

func (r *Repo) Unstage(ctx context.Context, path string) error {
	return r.mutate(ctx, nil, "restore", "--staged", "--", path)
}

func (r *Repo) Discard(ctx context.Context, path string) error {
	return r.mutate(ctx, nil, "restore", "--", path)
}

func (r *Repo) Switch(ctx context.Context, branch string) error {
	return r.mutate(ctx, nil, "switch", branch)
}

func (r *Repo) Commit(ctx context.Context, msg string) error {
	return r.mutate(ctx, strings.NewReader(msg), "commit", "-F", "-")
}

// CommitAll stages every change in the working tree (modified, deleted, and new
// files) and then commits, so a commit needs no prior per-file staging.
func (r *Repo) CommitAll(ctx context.Context, msg string) error {
	if err := r.mutate(ctx, nil, "add", "-A"); err != nil {
		return err
	}
	return r.Commit(ctx, msg)
}

func (r *Repo) Fetch(ctx context.Context) (string, error) { return r.remote(ctx, "fetch") }
func (r *Repo) Pull(ctx context.Context) (string, error)  { return r.remote(ctx, "pull") }
func (r *Repo) Push(ctx context.Context) (string, error)  { return r.remote(ctx, "push") }

// mutate runs a command whose only result is success/failure.
func (r *Repo) mutate(ctx context.Context, stdin io.Reader, args ...string) error {
	_, errb, err := r.runner.Run(ctx, stdin, args...)
	if err != nil {
		return fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(string(errb)))
	}
	return nil
}

// remote runs a long-running remote command and returns its combined output.
func (r *Repo) remote(ctx context.Context, sub string) (string, error) {
	out, errb, err := r.runner.Run(ctx, nil, sub)
	combined := strings.TrimSpace(string(out) + "\n" + string(errb))
	if err != nil {
		return combined, fmt.Errorf("git %s: %w", sub, err)
	}
	return combined, nil
}
