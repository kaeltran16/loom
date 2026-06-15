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
	m.input.SetValue("my message")
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
	m.input.SetValue("msg")
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
	m.input.SetValue("msg")
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
	if m.scroll[PanelFiles] != 6 { // 15 - 10 + 1
		t.Fatalf("scroll = %d, want 6", m.scroll[PanelFiles])
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
