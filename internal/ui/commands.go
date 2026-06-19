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

func loadCommitAuthors(ctx context.Context, repo *git.Repo, ref string) tea.Cmd {
	return func() tea.Msg {
		authors, err := repo.CommitAuthors(ctx, ref, defaultCommitAuthorsLimit)
		return commitAuthorsLoadedMsg{branch: ref, authors: authors, err: err}
	}
}

func searchCommits(ctx context.Context, repo *git.Repo, q git.CommitSearch) tea.Cmd {
	return func() tea.Msg {
		commits, err := repo.SearchCommits(ctx, q)
		return commitSearchLoadedMsg{commits: commits, summary: commitSearchSummary(q), err: err}
	}
}

func commitSearchSummary(q git.CommitSearch) string {
	var parts []string
	if strings.TrimSpace(q.Query) != "" {
		parts = append(parts, fmt.Sprintf("%q", strings.TrimSpace(q.Query)))
	}
	if strings.TrimSpace(q.Ref) != "" {
		parts = append(parts, "branch "+q.Ref)
	}
	if strings.TrimSpace(q.Author) != "" && q.Author != authorAny {
		parts = append(parts, "author "+q.Author)
	}
	return "Search: " + strings.Join(parts, " | ")
}

func loadStashes(ctx context.Context, repo *git.Repo) tea.Cmd {
	return func() tea.Msg {
		ss, err := repo.Stashes(ctx)
		if err != nil {
			return errMsg{err}
		}
		return stashesLoadedMsg{stashes: ss}
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

func loadStashShow(ctx context.Context, repo *git.Repo, ref string, seq int) tea.Cmd {
	return func() tea.Msg {
		text, err := repo.StashShow(ctx, ref)
		if err != nil {
			return errMsg{err}
		}
		return stashShowLoadedMsg{text: text, seq: seq}
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

func cherryPick(ctx context.Context, repo *git.Repo, hashes []string) tea.Cmd {
	return func() tea.Msg {
		out, err := repo.CherryPick(ctx, hashes)
		if err != nil {
			return gitDoneMsg{cmd: "git cherry-pick", output: out, err: err}
		}
		return gitDoneMsg{
			cmd:    "git cherry-pick",
			output: out,
			notice: fmt.Sprintf("Cherry-picked %d commits", len(hashes)),
		}
	}
}

func stashPush(ctx context.Context, repo *git.Repo, message string) tea.Cmd {
	return func() tea.Msg {
		out, err := repo.StashPush(ctx, message)
		if err != nil {
			return gitDoneMsg{cmd: "git stash push", output: out, err: err}
		}
		return gitDoneMsg{cmd: "git stash push", output: out, notice: "Stashed " + strings.TrimSpace(message)}
	}
}

func stashApply(ctx context.Context, repo *git.Repo, ref string) tea.Cmd {
	return stashMutation("git stash apply "+ref, func() (string, error) { return repo.StashApply(ctx, ref) })
}

func stashPop(ctx context.Context, repo *git.Repo, ref string) tea.Cmd {
	return stashMutation("git stash pop "+ref, func() (string, error) { return repo.StashPop(ctx, ref) })
}

func stashDrop(ctx context.Context, repo *git.Repo, ref string) tea.Cmd {
	return stashMutation("git stash drop "+ref, func() (string, error) { return repo.StashDrop(ctx, ref) })
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

func stashMutation(label string, fn func() (string, error)) tea.Cmd {
	return func() tea.Msg {
		out, err := fn()
		return gitDoneMsg{cmd: label, output: out, err: err}
	}
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
