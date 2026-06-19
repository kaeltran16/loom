package ui

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	cherryPickNoSelection = "No commits selected"
	cherryPickDirtyTree   = "Commit or stash current changes before cherry-pick"
	cherryPickStopped     = "Cherry-pick stopped; resolve conflicts outside Loom or inspect Files"
)

func (m Model) selectedCommitHashesInListOrder() []string {
	hashes := make([]string, 0, len(m.selectedCommits))
	for _, c := range m.commits {
		if m.selectedCommits[c.Hash] {
			hashes = append(hashes, c.Hash)
		}
	}
	return hashes
}

func (m Model) selectedCommitCount() int {
	return len(m.selectedCommitHashesInListOrder())
}

func (m Model) commitSelected(hash string) bool {
	return m.selectedCommits != nil && m.selectedCommits[hash]
}

func (m *Model) clearCommitSelection() {
	m.selectedCommits = map[string]bool{}
}

func (m *Model) toggleCommitSelection() {
	if m.focus != PanelCommits {
		return
	}
	i := m.cursor[PanelCommits]
	if i < 0 || i >= len(m.commits) {
		return
	}
	if m.selectedCommits == nil {
		m.selectedCommits = map[string]bool{}
	}
	hash := m.commits[i].Hash
	if m.selectedCommits[hash] {
		delete(m.selectedCommits, hash)
		return
	}
	m.selectedCommits[hash] = true
}

func (m Model) dirtyForCherryPick() bool {
	return len(m.files) > 0
}

func (m Model) startCherryPick() (tea.Model, tea.Cmd) {
	if m.focus != PanelCommits {
		return m, nil
	}
	hashes := m.selectedCommitHashesInListOrder()
	if len(hashes) == 0 {
		m.err = errors.New(cherryPickNoSelection)
		return m, nil
	}
	if m.dirtyForCherryPick() {
		m.err = errors.New(cherryPickDirtyTree)
		return m, nil
	}
	m.mode = ModeConfirming
	m.confirm = confirmReq{
		prompt: fmt.Sprintf("Cherry-pick %d commits in list order? [y/n]", len(hashes)),
		action: cherryPick(m.ctx, m.repo, hashes),
	}
	return m, nil
}
