package ui

import (
	"context"
	"fmt"
	"strings"
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
		m.merging = msg.merging
		return m.reloadMain()
	case branchesLoadedMsg:
		m.branches = msg.branches
		return m, nil
	case commitAuthorsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		if msg.branch == m.commitSearch.Branch {
			m.authors = msg.authors
			m.syncCommitSearchSelection()
		}
		return m, nil
	case commitSearchLoadedMsg:
		m.busy = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.commits = msg.commits
		m.cursor[PanelCommits] = 0
		m.scroll[PanelCommits] = 0
		m.commitSearch.Active = true
		m.commitSearch.Summary = msg.summary
		if m.focus == PanelCommits {
			updated, cmd := m.reloadMain()
			if len(updated.commits) == 0 {
				// no commit to preview; clear the stale diff so mainContent shows the empty-pane copy
				updated.viewport.SetContent("")
			}
			return updated, cmd
		}
		return m, nil
	case commitsLoadedMsg:
		m.busy = false // clears the busy set by clearCommitSearch; other loadCommits sites are already idle
		m.commits = msg.commits
		m.commitSearch.Active = false
		m.commitSearch.Summary = ""
		if m.cursor[PanelCommits] >= len(m.commits) {
			if len(m.commits) == 0 {
				m.cursor[PanelCommits] = 0
			} else {
				m.cursor[PanelCommits] = len(m.commits) - 1
			}
		}
		// when the Commits panel is focused (e.g. after clearing a search), refresh
		// the preview so the highlighted commit and the diff pane match. Mirrors
		// stashesLoadedMsg; redundant with statusLoadedMsg's reload after a mutation,
		// but reloadMain's request token drops the stale load.
		if m.focus == PanelCommits {
			updated, cmd := m.reloadMain()
			if len(updated.commits) == 0 {
				// no commit to preview; clear the stale diff so mainContent shows the empty-pane copy
				updated.viewport.SetContent("")
			}
			return updated, cmd
		}
		return m, nil
	case stashesLoadedMsg:
		m.stashes = msg.stashes
		if m.cursor[PanelStashes] >= len(m.stashes) {
			if len(m.stashes) == 0 {
				m.cursor[PanelStashes] = 0
			} else {
				m.cursor[PanelStashes] = len(m.stashes) - 1
			}
		}
		if m.focus == PanelStashes {
			return m.reloadMain()
		}
		return m, nil
	case amendPrefillMsg:
		if msg.err != nil {
			return m, nil // e.g. empty repo (no HEAD): stay put
		}
		m.amending = true
		m.subject.SetValue(msg.subject)
		m.body.SetValue(msg.body)
		m.subject.CursorEnd()
		m.mode = ModeCommitting
		m.commitField = fieldSubject
		m.subject.Focus()
		m.body.Blur()
		return m, nil
	case diffLoadedMsg:
		// drop responses for a selection we've already navigated away from
		if msg.seq == m.reqSeq {
			m.viewport.SetContent(msg.text)
			m.mainLoading = false
		}
		return m, nil
	case stashShowLoadedMsg:
		if msg.seq == m.reqSeq {
			m.viewport.SetContent(msg.text)
			m.mainLoading = false
		}
		return m, nil
	case errMsg:
		m.err = msg.err
		m.busy = false
		return m, nil

	case editorDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		return m, loadStatus(m.ctx, m.repo)

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
		m.notice = msg.notice
		// refresh the data that a mutation could have changed
		return m, tea.Batch(
			loadStatus(m.ctx, m.repo),
			loadBranches(m.ctx, m.repo),
			loadCommits(m.ctx, m.repo),
			loadStashes(m.ctx, m.repo),
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
	m.notice = ""
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
			return m.submitCommit()
		case tea.KeyEsc:
			m.resetCommit()
			return m, nil
		case tea.KeyTab:
			m.toggleCommitField()
			return m, nil
		}
		var cmd tea.Cmd
		if m.commitField == fieldSubject {
			m.subject, cmd = m.subject.Update(msg)
		} else {
			m.body, cmd = m.body.Update(msg)
		}
		return m, cmd
	}

	if m.mode == ModeStashing {
		switch msg.Type {
		case tea.KeyCtrlD:
			return m.submitStash()
		case tea.KeyEsc:
			m.resetStashMessage()
			return m, nil
		}
		var cmd tea.Cmd
		m.stashMessage, cmd = m.stashMessage.Update(msg)
		return m, cmd
	}

	if m.mode == ModeCommitSearch {
		return m.handleCommitSearchKey(msg)
	}

	switch msg.String() {
	case keyQuit, keyQuitAlt:
		return m, tea.Quit
	case keyCancel:
		if m.busy && m.cancelOp != nil {
			m.cancelOp()
			return m, nil
		}
		if m.focus == PanelCommits && m.commitSearch.Active {
			return m.clearCommitSearch()
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
	case keyStashes:
		m.focus = PanelStashes
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
	case keyStashSave:
		return m.openStashMessage()
	case keyStashApply:
		return m.applySelectedStash()
	case keyStashPop:
		return m.popSelectedStash()
	case keyDiscard:
		if m.focus == PanelStashes {
			return m.dropSelectedStash()
		}
		return m.discardSelected()
	case keyAbortMerge:
		if !m.merging {
			return m, nil
		}
		m.mode = ModeConfirming
		m.confirm = confirmReq{
			prompt: "Abort the merge? Conflict resolutions will be discarded. [y/n]",
			action: mergeAbort(m.ctx, m.repo),
		}
		return m, nil
	case keyEditConflict:
		return m.editConflict()
	case keySearch:
		return m.openCommitSearch()
	case keyCommit:
		m.mode = ModeCommitting
		m.commitField = fieldSubject
		m.subject.Focus()
		m.body.Blur()
		return m, nil
	case keyAmend:
		return m, loadHeadMessage(m.ctx, m.repo)
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
	}
	return m, nil
}

func (m Model) handleCommitSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		return m.applyCommitSearch()
	case tea.KeyEsc:
		m.resetCommitSearchEditor()
		return m, nil
	case tea.KeyTab:
		m.nextCommitSearchField(1)
		return m, nil
	case tea.KeyShiftTab:
		m.nextCommitSearchField(-1)
		return m, nil
	}

	switch msg.String() {
	case keyDown, keyDownAlt:
		if m.commitSearch.Field == searchFieldQuery {
			break
		}
		if m.moveCommitSearchChoice(1) {
			return m, loadCommitAuthors(m.ctx, m.repo, m.commitSearch.Branch)
		}
		return m, nil
	case keyUp, keyUpAlt:
		if m.commitSearch.Field == searchFieldQuery {
			break
		}
		if m.moveCommitSearchChoice(-1) {
			return m, loadCommitAuthors(m.ctx, m.repo, m.commitSearch.Branch)
		}
		return m, nil
	}

	if m.commitSearch.Field == searchFieldQuery {
		var cmd tea.Cmd
		m.commitQuery, cmd = m.commitQuery.Update(msg)
		return m, cmd
	}
	return m, nil
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

// editConflict opens the selected file in the editor, but only when it is an
// unmerged conflict in the Files panel.
func (m Model) editConflict() (tea.Model, tea.Cmd) {
	if m.focus != PanelFiles {
		return m, nil
	}
	i := m.cursor[PanelFiles]
	if i >= len(m.files) || !m.files[i].Unmerged {
		return m, nil
	}
	return m, openEditor(m.ctx, m.repo, m.files[i].Path)
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

	selectedRow := m.selectedPanelRow(m.focus)
	off := m.scroll[m.focus]
	if selectedRow < off {
		off = selectedRow
	}
	if m.listHeight > 0 && selectedRow >= off+m.listHeight {
		off = selectedRow - m.listHeight + 1
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
	case PanelStashes:
		return len(m.stashes)
	}
	return 0
}

// reloadMain bumps the request token so any in-flight load for the prior
// selection is dropped as stale, then loads the main pane for the new one.
func (m Model) reloadMain() (Model, tea.Cmd) {
	m.reqSeq++
	cmd := m.loadMainForSelection()
	m.mainLoading = cmd != nil
	if cmd != nil {
		m.viewport.SetContent("")
	}
	return m, cmd
}

// loadMainForSelection returns a Cmd to refresh the main pane for the current
// selection, stamped with the current request token.
func (m Model) loadMainForSelection() tea.Cmd {
	switch m.focus {
	case PanelFiles:
		if i := m.cursor[PanelFiles]; i < len(m.files) {
			f := m.files[i]
			return loadDiff(m.ctx, m.repo, f.Path, f.IsStaged(), m.reqSeq)
		}
	case PanelCommits:
		if i := m.cursor[PanelCommits]; i < len(m.commits) {
			return loadShow(m.ctx, m.repo, m.commits[i].Hash, m.reqSeq)
		}
	case PanelBranches:
		if i := m.cursor[PanelBranches]; i < len(m.branches) {
			return loadBranchLog(m.ctx, m.repo, m.branches[i].Name, m.reqSeq)
		}
	case PanelStashes:
		if i := m.cursor[PanelStashes]; i < len(m.stashes) {
			return loadStashShow(m.ctx, m.repo, m.stashes[i].Ref, m.reqSeq)
		}
	}
	return nil
}

func (m *Model) toggleCommitField() {
	if m.commitField == fieldSubject {
		m.commitField = fieldBody
		m.subject.Blur()
		m.body.Focus()
		return
	}
	m.commitField = fieldSubject
	m.body.Blur()
	m.subject.Focus()
}

func (m *Model) resetCommit() {
	m.subject.Reset()
	m.body.Reset()
	m.subject.Blur()
	m.body.Blur()
	m.commitField = fieldSubject
	m.amending = false
	m.mode = ModeNormal
}

func (m Model) openStashMessage() (tea.Model, tea.Cmd) {
	if !m.canOpenStashMessage() {
		return m, nil
	}
	m.mode = ModeStashing
	m.stashMessage.Focus()
	return m, nil
}

func (m Model) canOpenStashMessage() bool {
	switch m.focus {
	case PanelStashes:
		return true
	case PanelFiles:
		return len(m.files) > 0 && m.unmergedCount() == 0
	default:
		return false
	}
}

func (m *Model) resetStashMessage() {
	m.stashMessage.Reset()
	m.stashMessage.Blur()
	m.mode = ModeNormal
}

func (m Model) submitStash() (tea.Model, tea.Cmd) {
	message := strings.TrimSpace(m.stashMessage.Value())
	if message == "" {
		return m, nil
	}
	m.resetStashMessage()
	m.busy = true
	return m, stashPush(m.ctx, m.repo, message)
}

func (m Model) applySelectedStash() (tea.Model, tea.Cmd) {
	if m.focus != PanelStashes {
		return m, nil
	}
	s, ok := m.selectedStash()
	if !ok {
		return m, nil
	}
	m.busy = true
	return m, stashApply(m.ctx, m.repo, s.Ref)
}

func (m Model) popSelectedStash() (tea.Model, tea.Cmd) {
	if m.focus != PanelStashes {
		return m, nil
	}
	s, ok := m.selectedStash()
	if !ok {
		return m, nil
	}
	m.busy = true
	return m, stashPop(m.ctx, m.repo, s.Ref)
}

func (m Model) dropSelectedStash() (tea.Model, tea.Cmd) {
	if m.focus != PanelStashes {
		return m, nil
	}
	s, ok := m.selectedStash()
	if !ok {
		return m, nil
	}
	m.mode = ModeConfirming
	m.confirm = confirmReq{
		prompt: "Drop " + s.Ref + "? [y/n]",
		action: stashDrop(m.ctx, m.repo, s.Ref),
	}
	return m, nil
}

// submitCommit assembles the message and dispatches the right commit command.
// An empty subject is a no-op so Ctrl-D never commits a blank summary. An amend
// of an already-pushed commit is gated behind a confirm.
func (m Model) submitCommit() (tea.Model, tea.Cmd) {
	subject := strings.TrimSpace(m.subject.Value())
	if subject == "" {
		return m, nil
	}
	full := buildCommitMessage(m.subject.Value(), m.body.Value())
	amending := m.amending
	pushed := m.amendPushed()
	m.resetCommit()
	if amending {
		action := commitAmend(m.ctx, m.repo, subject, full)
		if pushed {
			m.mode = ModeConfirming
			m.confirm = confirmReq{
				prompt: "Amend pushed commit? Needs a force-push. [y/n]",
				action: action,
			}
			return m, nil
		}
		m.busy = true
		return m, action
	}
	m.busy = true
	if m.anyStaged() {
		return m, commit(m.ctx, m.repo, subject, full)
	}
	return m, commitAll(m.ctx, m.repo, subject, full)
}

// amendPushed reports whether HEAD is already on the upstream, so amending it
// would require a force-push.
func (m Model) amendPushed() bool {
	return m.branch.Upstream != "" && m.branch.Ahead == 0
}
