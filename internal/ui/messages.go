package ui

import "github.com/kael02/loom/internal/git"

type statusLoadedMsg struct {
	files  []git.FileStatus
	branch git.BranchInfo
}
type branchesLoadedMsg struct{ branches []git.Branch }
type commitsLoadedMsg struct{ commits []git.Commit }
type diffLoadedMsg struct{ text string }
type gitDoneMsg struct {
	cmd    string
	output string
	err    error
}
type errMsg struct{ err error }
