package ui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kael02/loom/internal/git"
)

func newTestModel() Model {
	return NewModel(context.Background(), git.NewTestRepo(&git.StubRunner{}))
}

func TestUpdate_WindowSizeStoresDimensions(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	got := updated.(Model)
	if got.w != 120 || got.h != 40 {
		t.Errorf("dimensions = %dx%d, want 120x40", got.w, got.h)
	}
}

func TestUpdate_StatusLoadedPopulatesFiles(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(statusLoadedMsg{
		files:  []git.FileStatus{{Path: "a.go"}},
		branch: git.BranchInfo{Name: "main"},
	})
	got := updated.(Model)
	if len(got.files) != 1 || got.files[0].Path != "a.go" {
		t.Errorf("files not populated: %+v", got.files)
	}
}

func TestUpdate_TabCyclesFocus(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if updated.(Model).focus != PanelBranches {
		t.Errorf("focus = %v, want PanelBranches", updated.(Model).focus)
	}
}

func TestUpdate_JMovesCursorDown(t *testing.T) {
	m := newTestModel()
	m.files = []git.FileStatus{{Path: "a"}, {Path: "b"}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if updated.(Model).cursor[PanelFiles] != 1 {
		t.Errorf("cursor = %d, want 1", updated.(Model).cursor[PanelFiles])
	}
}

func TestUpdate_SpaceStagesUnstagedFile(t *testing.T) {
	m := newTestModel()
	m.files = []git.FileStatus{{Path: "a.go", Worktree: 'M'}} // unstaged
	m.focus = PanelFiles
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if !updated.(Model).busy {
		t.Error("expected busy=true after staging")
	}
	if cmd == nil {
		t.Fatal("expected a stage command")
	}
	if _, ok := cmd().(gitDoneMsg); !ok {
		t.Error("expected stage cmd to yield gitDoneMsg")
	}
}

func TestUpdate_DEntersConfirming(t *testing.T) {
	m := newTestModel()
	m.files = []git.FileStatus{{Path: "a.go", Worktree: 'M'}}
	m.focus = PanelFiles
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if updated.(Model).mode != ModeConfirming {
		t.Error("expected ModeConfirming after 'd'")
	}
}

func TestUpdate_ConfirmYDispatchesAction(t *testing.T) {
	m := newTestModel()
	m.mode = ModeConfirming
	m.confirm = confirmReq{prompt: "discard?", action: func() tea.Msg { return gitDoneMsg{cmd: "git restore"} }}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if updated.(Model).mode != ModeNormal {
		t.Error("expected return to ModeNormal")
	}
	if cmd == nil {
		t.Fatal("expected the confirmed action cmd")
	}
}

func TestUpdate_GitDoneClearsBusyAndChainsRefresh(t *testing.T) {
	m := newTestModel()
	m.busy = true
	updated, cmd := m.Update(gitDoneMsg{cmd: "git add a.go"})
	got := updated.(Model)
	if got.busy {
		t.Error("expected busy=false after gitDoneMsg")
	}
	if len(got.cmdLog) != 1 || got.cmdLog[0].text != "git add a.go" {
		t.Errorf("cmdLog = %v", got.cmdLog)
	}
	if cmd == nil {
		t.Error("expected a refresh cmd")
	}
}

func TestUpdate_GitDoneStoresRemoteOutput(t *testing.T) {
	m := newTestModel()
	m.busy = true
	updated, _ := m.Update(gitDoneMsg{cmd: "git push", output: "Everything up-to-date"})
	got := updated.(Model)
	if len(got.cmdLog) != 1 {
		t.Fatalf("cmdLog len = %d, want 1", len(got.cmdLog))
	}
	if got.cmdLog[0].output != "Everything up-to-date" {
		t.Errorf("cmdLog output = %q, want %q", got.cmdLog[0].output, "Everything up-to-date")
	}
}

func TestUpdate_RemoteFailureSetsConcisePointerError(t *testing.T) {
	m := newTestModel()
	m.busy = true
	updated, _ := m.Update(gitDoneMsg{
		cmd:    "git push",
		output: "! [rejected] main -> main (fetch first)\nerror: failed to push some refs",
		err:    errFake("git push: exit status 1"),
	})
	got := updated.(Model)
	if got.err == nil {
		t.Fatal("expected an error after a failed remote op")
	}
	msg := got.err.Error()
	if !strings.Contains(msg, "git push") {
		t.Errorf("error %q should name the command", msg)
	}
	if !strings.Contains(msg, "press "+keyLog) {
		t.Errorf("error %q should point to the command-log key %q", msg, keyLog)
	}
	if strings.Contains(msg, "\n") {
		t.Errorf("footer/rail error must stay single-line, got %q", msg)
	}
}

func TestUpdate_MutationFailureKeepsRawError(t *testing.T) {
	m := newTestModel()
	m.busy = true
	// a fast mutation carries no output; its concise git error should pass through
	updated, _ := m.Update(gitDoneMsg{cmd: "git restore a.go", err: errFake("git restore: pathspec error")})
	if got := updated.(Model).err; got == nil || got.Error() != "git restore: pathspec error" {
		t.Errorf("mutation error = %v, want the raw git error", got)
	}
}

func TestUpdate_CanceledRemoteOpClearsBusyWithoutError(t *testing.T) {
	m := newTestModel()
	m.busy = true
	_, cancel := context.WithCancel(context.Background())
	m.cancelOp = cancel

	updated, _ := m.Update(gitDoneMsg{cmd: "git fetch", canceled: true})
	got := updated.(Model)
	if got.busy {
		t.Error("expected busy=false after a canceled op")
	}
	if got.err != nil {
		t.Errorf("a canceled op should not set an error, got %v", got.err)
	}
	if got.cancelOp != nil {
		t.Error("expected cancelOp cleared after the op completes")
	}
	if len(got.cmdLog) != 1 || !strings.Contains(got.cmdLog[0].text, "canceled") {
		t.Errorf("expected the log entry to note cancellation: %+v", got.cmdLog)
	}
}

func TestUpdate_CEntersCommitting(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if updated.(Model).mode != ModeCommitting {
		t.Error("expected ModeCommitting after 'c'")
	}
}

func TestUpdate_CommitCtrlDDispatchesCommit(t *testing.T) {
	m := newTestModel()
	m.mode = ModeCommitting
	m.subject.SetValue("my message")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	if updated.(Model).mode != ModeNormal {
		t.Error("expected return to ModeNormal after commit")
	}
	if cmd == nil {
		t.Fatal("expected commit cmd")
	}
}

func TestModel_StagedCountAndAnyStaged(t *testing.T) {
	m := newTestModel()
	m.files = []git.FileStatus{
		{Path: "a", Staged: 'M'},     // staged
		{Path: "b", Worktree: 'M'},   // unstaged change
		{Path: "c", Staged: 'A'},     // staged
		{Path: "d", Untracked: true}, // new, not staged
	}
	if got := m.stagedCount(); got != 2 {
		t.Errorf("stagedCount = %d, want 2", got)
	}
	if !m.anyStaged() {
		t.Error("anyStaged = false, want true")
	}

	m.files = []git.FileStatus{{Path: "b", Worktree: 'M'}}
	if m.anyStaged() {
		t.Error("anyStaged = true with nothing staged, want false")
	}
}

func TestUpdate_CommitCtrlD_NothingStaged_CommitsAll(t *testing.T) {
	m := newTestModel()
	m.files = []git.FileStatus{{Path: "a.go", Worktree: 'M'}} // changed but unstaged
	m.mode = ModeCommitting
	m.subject.SetValue("msg")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	if cmd == nil {
		t.Fatal("expected a commit cmd")
	}
	done, ok := cmd().(gitDoneMsg)
	if !ok {
		t.Fatalf("expected gitDoneMsg, got %T", cmd())
	}
	if done.cmd != "git add -A && git commit" {
		t.Errorf("cmd = %q, want stage-all commit", done.cmd)
	}
}

func TestUpdate_CommitCtrlD_WithStaged_CommitsIndexOnly(t *testing.T) {
	m := newTestModel()
	m.files = []git.FileStatus{{Path: "a.go", Staged: 'M'}} // already staged
	m.mode = ModeCommitting
	m.subject.SetValue("msg")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	if cmd == nil {
		t.Fatal("expected a commit cmd")
	}
	done := cmd().(gitDoneMsg)
	if done.cmd != "git commit" {
		t.Errorf("cmd = %q, want plain commit", done.cmd)
	}
}

func TestUpdate_CommitEscCancels(t *testing.T) {
	m := newTestModel()
	m.mode = ModeCommitting
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.(Model).mode != ModeNormal {
		t.Error("expected ModeNormal after esc")
	}
}

func TestUpdate_EscWhileBusyCancelsRemoteOp(t *testing.T) {
	m := newTestModel()
	ctx, cancel := context.WithCancel(context.Background())
	m.busy = true
	m.cancelOp = cancel

	m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if ctx.Err() == nil {
		t.Error("esc while busy should cancel the in-flight remote op's context")
	}
}

func TestUpdate_EscWithoutCancelableOpIsNoop(t *testing.T) {
	m := newTestModel()
	m.busy = true // a fast mutation set busy but stored no cancel func
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !updated.(Model).busy {
		t.Error("esc with no cancelable op should leave state unchanged (no panic, still busy)")
	}
}

func TestUpdate_RemoteOpStoresCancelFunc(t *testing.T) {
	m := newTestModel()
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	got := updated.(Model)
	if !got.busy {
		t.Error("expected busy=true on fetch")
	}
	if got.cancelOp == nil {
		t.Error("expected a cancel func stored for the in-flight remote op")
	}
	if cmd == nil {
		t.Fatal("expected a fetch cmd")
	}
}

func TestUpdate_PushKeyDispatchesPush(t *testing.T) {
	m := newTestModel()
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("P")})
	if !updated.(Model).busy {
		t.Error("expected busy=true on push")
	}
	if cmd == nil {
		t.Fatal("expected push cmd")
	}
}

func TestUpdate_EnterSwitchesBranch(t *testing.T) {
	m := newTestModel()
	m.focus = PanelBranches
	m.branches = []git.Branch{{Name: "main", Current: true}, {Name: "feat/login"}}
	m.cursor[PanelBranches] = 1
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !updated.(Model).busy {
		t.Error("expected busy=true on switch")
	}
	if cmd == nil {
		t.Fatal("expected switch cmd")
	}
}

func TestMoveCursorScrollsToKeepSelectionVisible(t *testing.T) {
	m := newTestModel()
	m.focus = PanelFiles
	m.files = make([]git.FileStatus, 50)
	m.listHeight = 10

	for i := 0; i < 15; i++ {
		m.moveCursor(1)
	}

	if m.cursor[PanelFiles] != 15 {
		t.Fatalf("cursor = %d, want 15", m.cursor[PanelFiles])
	}
	if m.scroll[PanelFiles] != 7 { // selected visual row 16 - 10 + 1
		t.Fatalf("scroll = %d, want 7", m.scroll[PanelFiles])
	}
}

func TestSelectedPanelRowForGroupedFiles(t *testing.T) {
	m := newTestModel()
	m.focus = PanelFiles
	m.files = []git.FileStatus{
		{Path: "unstaged.go", Worktree: 'M'},
		{Path: "staged.go", Staged: 'M'},
		{Path: "new.go", Untracked: true},
	}

	m.cursor[PanelFiles] = 1
	if got := m.selectedPanelRow(PanelFiles); got != 1 {
		t.Fatalf("selected staged visual row = %d, want 1", got)
	}

	m.cursor[PanelFiles] = 0
	if got := m.selectedPanelRow(PanelFiles); got != 3 {
		t.Fatalf("selected unstaged visual row = %d, want 3", got)
	}

	m.cursor[PanelFiles] = 2
	if got := m.selectedPanelRow(PanelFiles); got != 5 {
		t.Fatalf("selected untracked visual row = %d, want 5", got)
	}
}

func TestMoveCursorScrollsGroupedFilesByVisibleRows(t *testing.T) {
	m := newTestModel()
	m.focus = PanelFiles
	m.listHeight = 2
	m.files = []git.FileStatus{
		{Path: "unstaged.go", Worktree: 'M'},
		{Path: "staged.go", Staged: 'M'},
		{Path: "new.go", Untracked: true},
	}

	m.moveCursor(1)
	if m.cursor[PanelFiles] != 1 {
		t.Fatalf("cursor = %d, want staged file index 1", m.cursor[PanelFiles])
	}
	if m.scroll[PanelFiles] != 0 {
		t.Fatalf("scroll after staged file = %d, want 0", m.scroll[PanelFiles])
	}

	m.moveCursor(-1)
	if m.cursor[PanelFiles] != 0 {
		t.Fatalf("cursor = %d, want unstaged file index 0", m.cursor[PanelFiles])
	}
	if m.scroll[PanelFiles] != 2 {
		t.Fatalf("scroll after unstaged visual row = %d, want 2", m.scroll[PanelFiles])
	}
}

func TestUpdate_QuestionTogglesHelp(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if !updated.(Model).showHelp {
		t.Error("expected showHelp=true after '?'")
	}
}

func TestUpdate_XOpensCommandLogAndClosesHelp(t *testing.T) {
	m := newTestModel()
	m.showHelp = true
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	got := updated.(Model)
	if !got.showLog || got.showHelp {
		t.Fatalf("showLog=%v showHelp=%v, want true/false", got.showLog, got.showHelp)
	}
}

func TestUpdate_QuestionOpensHelpAndClosesCommandLog(t *testing.T) {
	m := newTestModel()
	m.showLog = true
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	got := updated.(Model)
	if !got.showHelp || got.showLog {
		t.Fatalf("showHelp=%v showLog=%v, want true/false", got.showHelp, got.showLog)
	}
}

func TestUpdate_LFocusesMainPaneHReturnsToList(t *testing.T) {
	m := newTestModel()
	entered, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if !entered.(Model).mainFocused {
		t.Fatal("expected mainFocused=true after 'l'")
	}
	back, _ := entered.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	if back.(Model).mainFocused {
		t.Fatal("expected mainFocused=false after 'h'")
	}
}

func TestUpdate_JScrollsDiffWhenMainFocused(t *testing.T) {
	m := newTestModel()
	m.files = []git.FileStatus{{Path: "a"}, {Path: "b"}}
	m.focus = PanelFiles
	m.mainFocused = true
	m.viewport.Width = 80
	m.viewport.Height = 4
	m.viewport.SetContent(strings.Repeat("x\n", 30))

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	got := updated.(Model)
	if got.viewport.YOffset != 1 {
		t.Fatalf("YOffset = %d, want 1 (diff should scroll)", got.viewport.YOffset)
	}
	if got.cursor[PanelFiles] != 0 {
		t.Fatalf("list cursor moved while main focused: %d", got.cursor[PanelFiles])
	}
}

func TestUpdate_KScrollsDiffUpWhenMainFocused(t *testing.T) {
	m := newTestModel()
	m.mainFocused = true
	m.viewport.Width = 80
	m.viewport.Height = 4
	m.viewport.SetContent(strings.Repeat("x\n", 30))
	m.viewport.SetYOffset(5)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if got := updated.(Model).viewport.YOffset; got != 4 {
		t.Fatalf("YOffset = %d, want 4 (diff should scroll up)", got)
	}
}

func TestUpdate_SwitchingPanelReturnsFocusToList(t *testing.T) {
	m := newTestModel()
	m.mainFocused = true
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if updated.(Model).mainFocused {
		t.Fatal("expected mainFocused=false after Tab switches panel")
	}
}

func TestUpdate_SelectionLoadStampsAdvancingSeq(t *testing.T) {
	m := newTestModel()
	m.focus = PanelFiles
	m.files = []git.FileStatus{{Path: "a"}, {Path: "b"}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected a diff-load cmd after moving the cursor")
	}
	if got.reqSeq == 0 {
		t.Fatal("expected reqSeq to advance past zero on a new selection load")
	}
	msg, ok := cmd().(diffLoadedMsg)
	if !ok {
		t.Fatalf("expected diffLoadedMsg, got %T", cmd())
	}
	if msg.seq != got.reqSeq {
		t.Errorf("dispatched seq = %d, want current reqSeq = %d", msg.seq, got.reqSeq)
	}
}

func TestUpdate_CurrentDiffResponseApplied(t *testing.T) {
	m := newTestModel()
	m.w, m.h = 80, 24
	m.layout()
	m.reqSeq = 5

	updated, _ := m.Update(diffLoadedMsg{text: "FRESH", seq: 5})

	if !strings.Contains(updated.(Model).viewport.View(), "FRESH") {
		t.Error("diff response matching the current token should update the viewport")
	}
}

func TestUpdate_StaleDiffResponseIgnored(t *testing.T) {
	m := newTestModel()
	m.w, m.h = 80, 24
	m.layout()
	m.reqSeq = 5 // a newer request has already been issued

	updated, _ := m.Update(diffLoadedMsg{text: "STALE", seq: 4})

	if strings.Contains(updated.(Model).viewport.View(), "STALE") {
		t.Error("stale diff response (seq 4 < current 5) should not update the viewport")
	}
}

func TestReloadMainSetsAndClearsLoadingState(t *testing.T) {
	m := newTestModel()
	m.focus = PanelFiles
	m.files = []git.FileStatus{{Path: "a.go", Worktree: 'M'}}

	loading, cmd := m.reloadMain()
	if cmd == nil {
		t.Fatal("expected a diff load command")
	}
	if !loading.mainLoading {
		t.Fatal("reloadMain should set mainLoading while a load is in flight")
	}

	done, _ := loading.Update(diffLoadedMsg{text: "diff", seq: loading.reqSeq})
	if done.(Model).mainLoading {
		t.Fatal("matching diffLoadedMsg should clear mainLoading")
	}
}

func TestUpdate_GAndShiftGJumpDiffToTopAndBottom(t *testing.T) {
	m := newTestModel()
	m.mainFocused = true
	m.viewport.Width = 80
	m.viewport.Height = 4
	m.viewport.SetContent(strings.Repeat("x\n", 30))

	bottom, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if !bottom.(Model).viewport.AtBottom() {
		t.Fatalf("after G, want viewport at bottom, YOffset=%d", bottom.(Model).viewport.YOffset)
	}

	top, _ := bottom.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if !top.(Model).viewport.AtTop() {
		t.Fatalf("after g, want viewport at top, YOffset=%d", top.(Model).viewport.YOffset)
	}
}

func TestUpdate_CommitTabTogglesField(t *testing.T) {
	m := newTestModel()
	entered, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	mc := entered.(Model)
	if mc.commitField != fieldSubject {
		t.Fatalf("commitField after c = %d, want fieldSubject", mc.commitField)
	}
	toggled, _ := mc.Update(tea.KeyMsg{Type: tea.KeyTab})
	if toggled.(Model).commitField != fieldBody {
		t.Errorf("commitField after Tab = %d, want fieldBody", toggled.(Model).commitField)
	}
	back, _ := toggled.(Model).Update(tea.KeyMsg{Type: tea.KeyTab})
	if back.(Model).commitField != fieldSubject {
		t.Errorf("commitField after second Tab = %d, want fieldSubject", back.(Model).commitField)
	}
}

func TestUpdate_CommitEmptySubjectIsNoop(t *testing.T) {
	m := newTestModel()
	m.mode = ModeCommitting
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	if updated.(Model).mode != ModeCommitting {
		t.Error("empty-subject Ctrl-D should stay in the editor")
	}
	if cmd != nil {
		t.Error("empty-subject Ctrl-D should not dispatch a commit")
	}
}

func TestUpdate_GitDoneSetsNotice(t *testing.T) {
	m := newTestModel()
	m.busy = true
	updated, _ := m.Update(gitDoneMsg{cmd: "git commit", notice: "Committed a1b2c3d feat: x"})
	if got := updated.(Model).notice; got != "Committed a1b2c3d feat: x" {
		t.Errorf("notice = %q, want the success line", got)
	}
}

func TestUpdate_NoticeClearsOnNextKey(t *testing.T) {
	m := newTestModel()
	m.notice = "Committed a1b2c3d feat: x"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if updated.(Model).notice != "" {
		t.Error("notice should clear on the next key")
	}
}

func TestUpdate_CapitalCLoadsHeadMessage(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("C")})
	if cmd == nil {
		t.Fatal("expected a head-message load cmd")
	}
	if _, ok := cmd().(amendPrefillMsg); !ok {
		t.Errorf("expected amendPrefillMsg, got %T", cmd())
	}
}

func TestUpdate_AmendPrefillEntersCommitting(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(amendPrefillMsg{subject: "feat: x", body: "why"})
	got := updated.(Model)
	if got.mode != ModeCommitting || !got.amending {
		t.Fatalf("mode=%v amending=%v, want committing+amending", got.mode, got.amending)
	}
	if got.subject.Value() != "feat: x" || got.body.Value() != "why" {
		t.Errorf("prefill = (%q,%q), want (feat: x, why)", got.subject.Value(), got.body.Value())
	}
}

func TestUpdate_AmendPrefillErrorStaysNormal(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(amendPrefillMsg{err: errFake("no HEAD")})
	got := updated.(Model)
	if got.mode != ModeNormal || got.amending {
		t.Errorf("mode=%v amending=%v, want normal/false on prefill error", got.mode, got.amending)
	}
}

func TestUpdate_AmendSubmitPushedRoutesToConfirm(t *testing.T) {
	m := newTestModel()
	m.mode = ModeCommitting
	m.amending = true
	m.subject.SetValue("feat: x")
	m.branch = git.BranchInfo{Name: "main", Upstream: "origin/main", Ahead: 0}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	got := updated.(Model)
	if got.mode != ModeConfirming {
		t.Fatalf("mode = %v, want ModeConfirming for a pushed amend", got.mode)
	}
	if got.confirm.action == nil {
		t.Error("expected the amend action stored on the confirm request")
	}
	if cmd != nil {
		t.Error("the confirm should not dispatch until y is pressed")
	}
}

func TestUpdate_AmendSubmitNotPushedDispatches(t *testing.T) {
	m := newTestModel()
	m.mode = ModeCommitting
	m.amending = true
	m.subject.SetValue("feat: x")
	m.branch = git.BranchInfo{Name: "main", Upstream: "origin/main", Ahead: 2}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	if !updated.(Model).busy {
		t.Error("expected busy=true on a direct amend")
	}
	if cmd == nil {
		t.Fatal("expected an amend cmd")
	}
	if done, ok := cmd().(gitDoneMsg); !ok || done.cmd != "git commit --amend" {
		t.Errorf("expected git commit --amend gitDoneMsg, got %#v", cmd())
	}
}

// end-to-end safety path: a pushed amend must route through the confirm AND,
// once confirmed with y, actually dispatch the amend command.
func TestUpdate_AmendPushedConfirmYDispatchesAmend(t *testing.T) {
	m := newTestModel()
	m.mode = ModeCommitting
	m.amending = true
	m.subject.SetValue("feat: x")
	m.branch = git.BranchInfo{Name: "main", Upstream: "origin/main", Ahead: 0}

	confirming, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	if confirming.(Model).mode != ModeConfirming {
		t.Fatalf("expected ModeConfirming after Ctrl-D on a pushed amend, got %v", confirming.(Model).mode)
	}

	done, cmd := confirming.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if !done.(Model).busy {
		t.Error("expected busy=true after confirming the amend with y")
	}
	if cmd == nil {
		t.Fatal("expected the amend cmd to dispatch after y")
	}
	if msg, ok := cmd().(gitDoneMsg); !ok || msg.cmd != "git commit --amend" {
		t.Errorf("expected git commit --amend gitDoneMsg after y, got %#v", cmd())
	}
}

func TestUpdate_StatusLoadedSetsMerging(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(statusLoadedMsg{merging: true})
	if !updated.(Model).merging {
		t.Error("expected merging=true after statusLoadedMsg{merging:true}")
	}
}

func TestUpdate_AbortMergeEntersConfirm(t *testing.T) {
	m := newTestModel()
	m.merging = true
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("A")})
	got := updated.(Model)
	if got.mode != ModeConfirming {
		t.Fatalf("mode = %v, want ModeConfirming", got.mode)
	}
	if got.confirm.action == nil {
		t.Error("expected an abort action stored on the confirm request")
	}
	if cmd != nil {
		t.Error("abort should not dispatch until y is pressed")
	}
}

func TestUpdate_AbortMergeIgnoredWhenNotMerging(t *testing.T) {
	m := newTestModel() // merging is false
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("A")})
	if updated.(Model).mode != ModeNormal {
		t.Error("A should be a no-op when not merging")
	}
}

func TestUpdate_EditConflictDispatchesWhenUnmerged(t *testing.T) {
	m := newTestModel()
	m.files = []git.FileStatus{{Path: "a.go", Unmerged: true}}
	m.focus = PanelFiles
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if cmd == nil {
		t.Fatal("expected an open-editor cmd for an unmerged file")
	}
}

func TestUpdate_EditConflictNoopWhenNotUnmerged(t *testing.T) {
	m := newTestModel()
	m.files = []git.FileStatus{{Path: "a.go", Worktree: 'M'}}
	m.focus = PanelFiles
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if cmd != nil {
		t.Error("e on a non-conflicted file should be a no-op")
	}
}

func TestUpdate_EditorDoneRefreshes(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(editorDoneMsg{})
	if cmd == nil {
		t.Error("expected a status refresh after editorDoneMsg")
	}
}

func TestUpdate_EditorDoneErrorSurfaces(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(editorDoneMsg{err: errFake("boom")})
	if updated.(Model).err == nil {
		t.Error("expected err set on editor failure")
	}
}
