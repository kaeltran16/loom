package ui

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kael02/loom/internal/git"
)

func loadStatus(ctx context.Context, repo *git.Repo) tea.Cmd {
	return func() tea.Msg {
		files, branch, err := repo.Status(ctx)
		if err != nil {
			return errMsg{err}
		}
		merging, _ := repo.Merging(ctx)
		return statusLoadedMsg{files: files, branch: branch, merging: merging}
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

func loadDiff(ctx context.Context, repo *git.Repo, path string, staged bool, seq int) tea.Cmd {
	return func() tea.Msg {
		text, err := repo.Diff(ctx, path, staged)
		if err != nil {
			return errMsg{err}
		}
		return diffLoadedMsg{text: text, seq: seq}
	}
}

func loadShow(ctx context.Context, repo *git.Repo, hash string, seq int) tea.Cmd {
	return func() tea.Msg {
		text, err := repo.Show(ctx, hash)
		if err != nil {
			return errMsg{err}
		}
		return diffLoadedMsg{text: diffOnlyShow(text), seq: seq}
	}
}

func diffOnlyShow(text string) string {
	const marker = "diff --git "
	if strings.HasPrefix(text, marker) {
		return text
	}
	if i := strings.Index(text, "\n"+marker); i >= 0 {
		return text[i+1:]
	}
	return ""
}

func loadBranchLog(ctx context.Context, repo *git.Repo, name string, seq int) tea.Cmd {
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
		return diffLoadedMsg{text: text, seq: seq}
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
func mergeAbort(ctx context.Context, repo *git.Repo) tea.Cmd {
	return mutation("git merge --abort", func() error { return repo.MergeAbort(ctx) })
}

// openEditor suspends the TUI and opens the conflicted file in the user's editor
// (the one git would use, else VS Code). On exit it yields editorDoneMsg.
func openEditor(ctx context.Context, repo *git.Repo, path string) tea.Cmd {
	editor := repo.EditorCommand(ctx)
	full := filepath.Join(repo.Root(), path)
	return tea.ExecProcess(editorExecCmd(editor, full), func(err error) tea.Msg {
		return editorDoneMsg{err: err}
	})
}

// editorExecCmd builds the command that opens file in editor, run through the
// platform shell so a multi-word editor string ("code --wait") launches the same
// way git launches core.editor.
func editorExecCmd(editor, file string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/c", editor+` "`+file+`"`)
	}
	return exec.Command("sh", "-c", editor+` "$0"`, file)
}
func commit(ctx context.Context, repo *git.Repo, subject, full string) tea.Cmd {
	return func() tea.Msg {
		hash, err := repo.Commit(ctx, full)
		if err != nil {
			return gitDoneMsg{cmd: "git commit", err: err}
		}
		return gitDoneMsg{cmd: "git commit", notice: noticeText(hash, subject)}
	}
}
func loadHeadMessage(ctx context.Context, repo *git.Repo) tea.Cmd {
	return func() tea.Msg {
		full, err := repo.HeadMessage(ctx)
		if err != nil {
			return amendPrefillMsg{err: err}
		}
		subject, body := splitCommitMessage(full)
		return amendPrefillMsg{subject: subject, body: body}
	}
}

func commitAmend(ctx context.Context, repo *git.Repo, subject, full string) tea.Cmd {
	return func() tea.Msg {
		hash, err := repo.CommitAmend(ctx, full)
		if err != nil {
			return gitDoneMsg{cmd: "git commit --amend", err: err}
		}
		return gitDoneMsg{cmd: "git commit --amend", notice: noticeText(hash, subject)}
	}
}

func commitAll(ctx context.Context, repo *git.Repo, subject, full string) tea.Cmd {
	return func() tea.Msg {
		hash, err := repo.CommitAll(ctx, full)
		if err != nil {
			return gitDoneMsg{cmd: "git add -A && git commit", err: err}
		}
		return gitDoneMsg{cmd: "git add -A && git commit", notice: noticeText(hash, subject)}
	}
}

// remoteFunc builds a remote-op command bound to a (cancelable) context.
type remoteFunc func(ctx context.Context, repo *git.Repo) tea.Cmd

func fetch(ctx context.Context, repo *git.Repo) tea.Cmd {
	return remoteOp(ctx, "git fetch", func() (string, error) { return repo.Fetch(ctx) })
}
func pull(ctx context.Context, repo *git.Repo) tea.Cmd {
	return remoteOp(ctx, "git pull", func() (string, error) { return repo.Pull(ctx) })
}
func push(ctx context.Context, repo *git.Repo) tea.Cmd {
	return remoteOp(ctx, "git push", func() (string, error) { return repo.Push(ctx) })
}

func mutation(label string, fn func() error) tea.Cmd {
	return func() tea.Msg { return gitDoneMsg{cmd: label, err: fn()} }
}
func remoteOp(ctx context.Context, label string, fn func() (string, error)) tea.Cmd {
	return func() tea.Msg {
		out, err := fn()
		// a canceled context means the user aborted; report that, not the
		// resulting process error (which is platform-specific, e.g. "signal: killed")
		if ctx.Err() != nil {
			return gitDoneMsg{cmd: label, output: out, canceled: true}
		}
		return gitDoneMsg{cmd: label, output: out, err: err}
	}
}
