package ui

import "github.com/kael02/loom/internal/git"

type statusLoadedMsg struct {
	files   []git.FileStatus
	branch  git.BranchInfo
	merging bool
}
type branchesLoadedMsg struct{ branches []git.Branch }
type commitsLoadedMsg struct{ commits []git.Commit }
type stashesLoadedMsg struct{ stashes []git.Stash }
type diffLoadedMsg struct {
	text string
	seq  int // request token; the handler drops responses whose seq is stale
}
type stashShowLoadedMsg struct {
	text string
	seq  int
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

// editorDoneMsg reports that the external editor launched for a conflicted file
// has exited.
type editorDoneMsg struct{ err error }

type commitAuthorsLoadedMsg struct {
	branch  string
	authors []string
	err     error
}

type commitSearchLoadedMsg struct {
	commits []git.Commit
	summary string
	err     error
}
