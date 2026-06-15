package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Update is the pure reducer.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.layout()
		return m, nil

	case statusLoadedMsg:
		m.files, m.err = msg.files, nil
		m.branch = msg.branch
		return m, m.loadMainForSelection()
	case branchesLoadedMsg:
		m.branches = msg.branches
		return m, nil
	case commitsLoadedMsg:
		m.commits = msg.commits
		return m, nil
	case diffLoadedMsg:
		m.viewport.SetContent(msg.text)
		return m, nil
	case errMsg:
		m.err = msg.err
		m.busy = false
		return m, nil

	case gitDoneMsg:
		m.busy = false
		m.cmdLog = append(m.cmdLog, cmdEntry{at: time.Now(), text: msg.cmd})
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		// refresh the data that a mutation could have changed
		return m, tea.Batch(
			loadStatus(m.ctx, m.repo),
			loadBranches(m.ctx, m.repo),
			loadCommits(m.ctx, m.repo),
		)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mode == ModeConfirming {
		switch msg.String() {
		case keyConfirm:
			action := m.confirm.action
			m.mode = ModeNormal
			m.busy = true
			return m, action
		case keyCancel, "n":
			m.mode = ModeNormal
			return m, nil
		}
		return m, nil
	}

	if m.mode == ModeCommitting {
		switch msg.Type {
		case tea.KeyCtrlD:
			text := m.input.Value()
			m.input.Reset()
			m.mode = ModeNormal
			if text == "" {
				return m, nil
			}
			m.busy = true
			// stage everything first when nothing is staged, so commit needs no
			// prior per-file staging; otherwise commit just the staged index.
			if m.anyStaged() {
				return m, commit(m.ctx, m.repo, text)
			}
			return m, commitAll(m.ctx, m.repo, text)
		case tea.KeyEsc:
			m.input.Reset()
			m.mode = ModeNormal
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case keyQuit, keyQuitAlt:
		return m, tea.Quit
	case keyTab:
		m.focus = (m.focus + 1) % panelCount
		m.mainFocused = false
		return m, m.loadMainForSelection()
	case "1":
		m.focus = PanelFiles
		m.mainFocused = false
		return m, m.loadMainForSelection()
	case "2":
		m.focus = PanelBranches
		m.mainFocused = false
		return m, m.loadMainForSelection()
	case "3":
		m.focus = PanelCommits
		m.mainFocused = false
		return m, m.loadMainForSelection()
	case keyMainFocus, keyMainFocusAlt:
		m.mainFocused = true
		return m, nil
	case keyListFocus, keyListFocusAlt:
		m.mainFocused = false
		return m, nil
	case keyDown, keyDownAlt:
		if m.mainFocused {
			m.viewport.LineDown(1)
			return m, nil
		}
		m.moveCursor(1)
		return m, m.loadMainForSelection()
	case keyUp, keyUpAlt:
		if m.mainFocused {
			m.viewport.LineUp(1)
			return m, nil
		}
		m.moveCursor(-1)
		return m, m.loadMainForSelection()
	case keyTop:
		if m.mainFocused {
			m.viewport.GotoTop()
		}
		return m, nil
	case keyBottom:
		if m.mainFocused {
			m.viewport.GotoBottom()
		}
		return m, nil
	case keyStage:
		return m.stageSelected()
	case keyDiscard:
		return m.discardSelected()
	case keyCommit:
		m.mode = ModeCommitting
		m.input.Focus()
		return m, nil
	case keyFetch:
		m.busy = true
		return m, fetch(m.ctx, m.repo)
	case keyPull:
		m.busy = true
		return m, pull(m.ctx, m.repo)
	case keyPush:
		m.busy = true
		return m, push(m.ctx, m.repo)
	case keyLog:
		m.showLog = !m.showLog
		m.showHelp = false
		return m, nil
	case keyHelp:
		m.showHelp = !m.showHelp
		m.showLog = false
		return m, nil
	case keyEnter:
		if m.focus == PanelBranches {
			i := m.cursor[PanelBranches]
			if i < len(m.branches) {
				m.busy = true
				return m, switchBranch(m.ctx, m.repo, m.branches[i].Name)
			}
		}
		return m, nil
	}
	return m, nil
}

func (m Model) stageSelected() (tea.Model, tea.Cmd) {
	if m.focus != PanelFiles {
		return m, nil
	}
	i := m.cursor[PanelFiles]
	if i >= len(m.files) {
		return m, nil
	}
	f := m.files[i]
	m.busy = true
	if f.IsStaged() {
		return m, unstageFile(m.ctx, m.repo, f.Path)
	}
	return m, stageFile(m.ctx, m.repo, f.Path)
}

func (m Model) discardSelected() (tea.Model, tea.Cmd) {
	if m.focus != PanelFiles {
		return m, nil
	}
	i := m.cursor[PanelFiles]
	if i >= len(m.files) {
		return m, nil
	}
	path := m.files[i].Path
	m.mode = ModeConfirming
	m.confirm = confirmReq{
		prompt: "Discard changes to " + path + "? [y/n]",
		action: discardFile(m.ctx, m.repo, path),
	}
	return m, nil
}

func (m *Model) moveCursor(delta int) {
	n := m.focusLen()
	if n == 0 {
		return
	}
	c := m.cursor[m.focus] + delta
	if c < 0 {
		c = 0
	}
	if c > n-1 {
		c = n - 1
	}
	m.cursor[m.focus] = c

	off := m.scroll[m.focus]
	if c < off {
		off = c
	}
	if m.listHeight > 0 && c >= off+m.listHeight {
		off = c - m.listHeight + 1
	}
	m.scroll[m.focus] = off
}

// stagedCount reports how many changed files are currently staged.
func (m Model) stagedCount() int {
	n := 0
	for _, f := range m.files {
		if f.IsStaged() {
			n++
		}
	}
	return n
}

func (m Model) anyStaged() bool { return m.stagedCount() > 0 }

func (m Model) focusLen() int {
	switch m.focus {
	case PanelFiles:
		return len(m.files)
	case PanelBranches:
		return len(m.branches)
	case PanelCommits:
		return len(m.commits)
	}
	return 0
}

// loadMainForSelection returns a Cmd to refresh the main pane for the current selection.
func (m Model) loadMainForSelection() tea.Cmd {
	switch m.focus {
	case PanelFiles:
		if i := m.cursor[PanelFiles]; i < len(m.files) {
			f := m.files[i]
			return loadDiff(m.ctx, m.repo, f.Path, f.IsStaged())
		}
	case PanelCommits:
		if i := m.cursor[PanelCommits]; i < len(m.commits) {
			return loadShow(m.ctx, m.repo, m.commits[i].Hash)
		}
	case PanelBranches:
		if i := m.cursor[PanelBranches]; i < len(m.branches) {
			return loadBranchLog(m.ctx, m.repo, m.branches[i].Name)
		}
	}
	return nil
}
