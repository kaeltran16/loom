package ui

import "github.com/kael02/loom/internal/git"

type statusLoadedMsg struct {
	files  []git.FileStatus
	branch git.BranchInfo
}
type branchesLoadedMsg struct{ branches []git.Branch }
type commitsLoadedMsg struct{ commits []git.Commit }
type diffLoadedMsg struct {
	diff git.Diff
	seq  int // request token; the handler drops responses whose seq is stale
}
type logLoadedMsg struct {
	text string
	seq  int // request token; the handler drops responses whose seq is stale
}
type gitDoneMsg struct {
	cmd      string
	output   string
	notice   string // success line to flash (set by commit commands)
	err      error
	canceled bool // true when the user aborted the op via esc
}
type errMsg struct{ err error }

// amendPrefillMsg carries HEAD's message into the editor to start an amend.
type amendPrefillMsg struct {
	subject string
	body    string
	err     error
}
