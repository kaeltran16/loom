package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kael02/loom/internal/git"
)

const (
	defaultCommitSearchLimit  = 50
	defaultCommitAuthorsLimit = 200
	authorAny                 = "Any"
)

func (m Model) branchChoices() []string {
	if len(m.branches) > 0 {
		choices := make([]string, 0, len(m.branches))
		for _, b := range m.branches {
			choices = append(choices, b.Name)
		}
		return choices
	}
	if strings.TrimSpace(m.branch.Name) != "" {
		return []string{m.branch.Name}
	}
	return []string{"HEAD"}
}

func (m Model) authorChoices() []string {
	choices := make([]string, 0, len(m.authors)+1)
	choices = append(choices, authorAny)
	choices = append(choices, m.authors...)
	return choices
}

func (m *Model) syncCommitSearchSelection() {
	branches := m.branchChoices()
	if m.commitSearch.Branch == "" {
		m.commitSearch.Branch = branches[0]
	}
	m.commitSearch.BranchCursor = indexOrZero(branches, m.commitSearch.Branch)
	m.commitSearch.Branch = branches[m.commitSearch.BranchCursor]

	authors := m.authorChoices()
	if m.commitSearch.Author == "" {
		m.commitSearch.Author = authorAny
	}
	m.commitSearch.AuthorCursor = indexOrZero(authors, m.commitSearch.Author)
	m.commitSearch.Author = authors[m.commitSearch.AuthorCursor]
}

func indexOrZero(values []string, target string) int {
	for i, v := range values {
		if v == target {
			return i
		}
	}
	return 0
}

func (m Model) openCommitSearch() (tea.Model, tea.Cmd) {
	if m.focus != PanelCommits {
		return m, nil
	}
	m.mode = ModeCommitSearch
	m.mainFocused = false
	m.commitSearch.Field = searchFieldQuery
	m.commitQuery.Focus()
	m.syncCommitSearchSelection()
	return m, loadCommitAuthors(m.ctx, m.repo, m.commitSearch.Branch)
}

func (m *Model) resetCommitSearchEditor() {
	m.commitQuery.Blur()
	m.commitSearch.Field = searchFieldQuery
	m.mode = ModeNormal
}

func (m Model) clearCommitSearch() (tea.Model, tea.Cmd) {
	if m.focus != PanelCommits || !m.commitSearch.Active {
		return m, nil
	}
	m.commitSearch.Active = false
	m.commitSearch.Summary = ""
	m.commitQuery.Reset()
	m.commitQuery.Blur()
	// reset to the top so the reloaded normal list and its preview stay in sync
	m.cursor[PanelCommits] = 0
	m.scroll[PanelCommits] = 0
	m.busy = true
	return m, loadCommits(m.ctx, m.repo)
}

func (m Model) commitSearchQuery() git.CommitSearch {
	author := m.commitSearch.Author
	if author == authorAny {
		author = ""
	}
	return git.CommitSearch{
		Query:  strings.TrimSpace(m.commitQuery.Value()),
		Ref:    m.commitSearch.Branch,
		Author: author,
		Limit:  defaultCommitSearchLimit,
	}
}

func (m Model) applyCommitSearch() (tea.Model, tea.Cmd) {
	m.syncCommitSearchSelection()
	q := m.commitSearchQuery()
	m.resetCommitSearchEditor()
	m.busy = true
	return m, searchCommits(m.ctx, m.repo, q)
}

func (m *Model) nextCommitSearchField(delta int) {
	n := int(searchFieldAuthor) + 1
	next := (int(m.commitSearch.Field) + delta + n) % n
	m.commitSearch.Field = commitSearchField(next)
	if m.commitSearch.Field == searchFieldQuery {
		m.commitQuery.Focus()
		return
	}
	m.commitQuery.Blur()
}

func (m *Model) moveCommitSearchChoice(delta int) (branchChanged bool) {
	switch m.commitSearch.Field {
	case searchFieldBranch:
		choices := m.branchChoices()
		m.commitSearch.BranchCursor = clampIndex(m.commitSearch.BranchCursor+delta, len(choices))
		old := m.commitSearch.Branch
		m.commitSearch.Branch = choices[m.commitSearch.BranchCursor]
		if old != m.commitSearch.Branch {
			m.commitSearch.Author = authorAny
			m.commitSearch.AuthorCursor = 0
			return true
		}
	case searchFieldAuthor:
		choices := m.authorChoices()
		m.commitSearch.AuthorCursor = clampIndex(m.commitSearch.AuthorCursor+delta, len(choices))
		m.commitSearch.Author = choices[m.commitSearch.AuthorCursor]
	}
	return false
}

func clampIndex(i, n int) int {
	if n <= 0 {
		return 0
	}
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}
