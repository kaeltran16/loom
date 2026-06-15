package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kael02/loom/internal/git"
)

func loadStatus(ctx context.Context, repo *git.Repo) tea.Cmd {
	return func() tea.Msg {
		files, branch, err := repo.Status(ctx)
		if err != nil {
			return errMsg{err}
		}
		return statusLoadedMsg{files: files, branch: branch}
	}
}

func loadBranches(ctx context.Context, repo *git.Repo) tea.Cmd {
	return func() tea.Msg {
		bs, err := repo.Branches(ctx)
		if err != nil {
			return errMsg{err}
		}
		return branchesLoadedMsg{branches: bs}
	}
}

func loadCommits(ctx context.Context, repo *git.Repo) tea.Cmd {
	return func() tea.Msg {
		cs, err := repo.Log(ctx, "", 50)
		if err != nil {
			return errMsg{err}
		}
		return commitsLoadedMsg{commits: cs}
	}
}

func loadDiff(ctx context.Context, repo *git.Repo, path string, staged bool) tea.Cmd {
	return func() tea.Msg {
		text, err := repo.Diff(ctx, path, staged)
		if err != nil {
			return errMsg{err}
		}
		return diffLoadedMsg{text: text}
	}
}

func loadShow(ctx context.Context, repo *git.Repo, hash string) tea.Cmd {
	return func() tea.Msg {
		text, err := repo.Show(ctx, hash)
		if err != nil {
			return errMsg{err}
		}
		return diffLoadedMsg{text: text}
	}
}

func loadBranchLog(ctx context.Context, repo *git.Repo, name string) tea.Cmd {
	return func() tea.Msg {
		cs, err := repo.Log(ctx, name, 50)
		if err != nil {
			return errMsg{err}
		}
		// reuse diffLoadedMsg to render text in the main pane
		text := ""
		for _, c := range cs {
			text += c.Hash[:7] + "  " + c.Subject + "\n"
		}
		return diffLoadedMsg{text: text}
	}
}

func stageFile(ctx context.Context, repo *git.Repo, path string) tea.Cmd {
	return mutation(fmt.Sprintf("git add %s", path), func() error { return repo.Stage(ctx, path) })
}
func unstageFile(ctx context.Context, repo *git.Repo, path string) tea.Cmd {
	return mutation(fmt.Sprintf("git restore --staged %s", path), func() error { return repo.Unstage(ctx, path) })
}
func discardFile(ctx context.Context, repo *git.Repo, path string) tea.Cmd {
	return mutation(fmt.Sprintf("git restore %s", path), func() error { return repo.Discard(ctx, path) })
}
func switchBranch(ctx context.Context, repo *git.Repo, name string) tea.Cmd {
	return mutation(fmt.Sprintf("git switch %s", name), func() error { return repo.Switch(ctx, name) })
}
func commit(ctx context.Context, repo *git.Repo, msg string) tea.Cmd {
	return mutation("git commit", func() error { return repo.Commit(ctx, msg) })
}
func commitAll(ctx context.Context, repo *git.Repo, msg string) tea.Cmd {
	return mutation("git add -A && git commit", func() error { return repo.CommitAll(ctx, msg) })
}

func fetch(ctx context.Context, repo *git.Repo) tea.Cmd {
	return remoteOp("git fetch", func() (string, error) { return repo.Fetch(ctx) })
}
func pull(ctx context.Context, repo *git.Repo) tea.Cmd {
	return remoteOp("git pull", func() (string, error) { return repo.Pull(ctx) })
}
func push(ctx context.Context, repo *git.Repo) tea.Cmd {
	return remoteOp("git push", func() (string, error) { return repo.Push(ctx) })
}

func mutation(label string, fn func() error) tea.Cmd {
	return func() tea.Msg { return gitDoneMsg{cmd: label, err: fn()} }
}
func remoteOp(label string, fn func() (string, error)) tea.Cmd {
	return func() tea.Msg {
		out, err := fn()
		return gitDoneMsg{cmd: label, output: out, err: err}
	}
}
