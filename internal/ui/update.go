package ui

import (
	"context"
	"fmt"
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
		m.refreshViewport()
		return m, nil

	case statusLoadedMsg:
		m.files, m.err = msg.files, nil
		m.branch = msg.branch
		return m.reloadMain()
	case branchesLoadedMsg:
		m.branches = msg.branches
		return m, nil
	case commitsLoadedMsg:
		m.commits = msg.commits
		return m, nil
	case diffLoadedMsg:
		// drop responses for a selection we've already navigated away from
		if msg.seq == m.reqSeq {
			m.mainDiff = &msg.diff
			m.mainText = ""
			m.refreshViewport()
		}
		return m, nil
	case logLoadedMsg:
		if msg.seq == m.reqSeq {
			m.mainDiff = nil
			m.mainText = msg.text
			m.refreshViewport()
		}
		return m, nil
	case errMsg:
		m.err = msg.err
		m.busy = false
		return m, nil

	case gitDoneMsg:
		m.busy = false
		m.cancelOp = nil
		text := msg.cmd
		if msg.canceled {
			text += " (canceled)"
		}
		m.cmdLog = append(m.cmdLog, cmdEntry{at: time.Now(), text: text, output: msg.output})
		switch {
		case msg.canceled:
			m.err = nil
			return m, nil
		case msg.err != nil && msg.output != "":
			// remote op: keep the footer/rail one line; git's real reason lives in the command log
			m.err = fmt.Errorf("%s failed — press %s for details", msg.cmd, keyLog)
			return m, nil
		case msg.err != nil:
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
	case keyCancel:
		// abort an in-flight remote op; harmless no-op otherwise
		if m.busy && m.cancelOp != nil {
			m.cancelOp()
		}
		return m, nil
	case keyTab:
		m.focus = (m.focus + 1) % panelCount
		m.mainFocused = false
		return m.reloadMain()
	case "1":
		m.focus = PanelFiles
		m.mainFocused = false
		return m.reloadMain()
	case "2":
		m.focus = PanelBranches
		m.mainFocused = false
		return m.reloadMain()
	case "3":
		m.focus = PanelCommits
		m.mainFocused = false
		return m.reloadMain()
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
		return m.reloadMain()
	case keyUp, keyUpAlt:
		if m.mainFocused {
			m.viewport.LineUp(1)
			return m, nil
		}
		m.moveCursor(-1)
		return m.reloadMain()
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
		return m.startRemote(fetch)
	case keyPull:
		return m.startRemote(pull)
	case keyPush:
		return m.startRemote(push)
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
	case keyHunkNext:
		if m.mainDiff != nil {
			m.viewport.SetYOffset(nextOffset(m.hunkRows, m.viewport.YOffset))
		}
		return m, nil
	case keyHunkPrev:
		if m.mainDiff != nil {
			m.viewport.SetYOffset(prevOffset(m.hunkRows, m.viewport.YOffset))
		}
		return m, nil
	}
	return m, nil
}

// nextOffset returns the first hunk row strictly below cur, clamped to the last.
func nextOffset(rows []int, cur int) int {
	for _, r := range rows {
		if r > cur {
			return r
		}
	}
	if len(rows) > 0 {
		return rows[len(rows)-1]
	}
	return cur
}

// prevOffset returns the last hunk row strictly above cur, clamped to the first.
func prevOffset(rows []int, cur int) int {
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i] < cur {
			return rows[i]
		}
	}
	if len(rows) > 0 {
		return rows[0]
	}
	return cur
}

// startRemote dispatches a long-running remote op under a cancelable child
// context and stores its cancel func so esc can abort it while busy.
func (m Model) startRemote(build remoteFunc) (tea.Model, tea.Cmd) {
	opCtx, cancel := context.WithCancel(m.ctx)
	m.cancelOp = cancel
	m.busy = true
	return m, build(opCtx, m.repo)
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

// reloadMain bumps the request token so any in-flight load for the prior
// selection is dropped as stale, then loads the main pane for the new one.
func (m Model) reloadMain() (Model, tea.Cmd) {
	m.reqSeq++
	return m, m.loadMainForSelection()
}

// loadMainForSelection returns a Cmd to refresh the main pane for the current
// selection, stamped with the current request token.
func (m Model) loadMainForSelection() tea.Cmd {
	switch m.focus {
	case PanelFiles:
		if i := m.cursor[PanelFiles]; i < len(m.files) {
			f := m.files[i]
			return loadDiff(m.ctx, m.repo, f.Path, f.IsStaged(), f.Untracked, m.reqSeq)
		}
	case PanelCommits:
		if i := m.cursor[PanelCommits]; i < len(m.commits) {
			return loadShow(m.ctx, m.repo, m.commits[i].Hash, m.reqSeq)
		}
	case PanelBranches:
		if i := m.cursor[PanelBranches]; i < len(m.branches) {
			return loadBranchLog(m.ctx, m.repo, m.branches[i].Name, m.reqSeq)
		}
	}
	return nil
}
