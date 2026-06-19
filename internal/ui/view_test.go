package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/kael02/loom/internal/git"
)

func TestPanelTitleIncludesCount(t *testing.T) {
	if got := panelTitle("Files", 3); got != "Files 3" {
		t.Fatalf("panelTitle = %q, want %q", got, "Files 3")
	}
}

func TestPanelLinesShowEmptyCopy(t *testing.T) {
	m := newTestModel()

	tests := []struct {
		name string
		p    Panel
		want string
	}{
		{name: "files", p: PanelFiles, want: "No changes"},
		{name: "branches", p: PanelBranches, want: "No local branches"},
		{name: "commits", p: PanelCommits, want: "No commits"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.panelLines(tt.p)
			if len(got) != 1 || got[0] != tt.want {
				t.Fatalf("panelLines(%v) = %#v, want [%q]", tt.p, got, tt.want)
			}
		})
	}
}

func TestPanelLinesRenderRows(t *testing.T) {
	m := newTestModel()
	m.files = []git.FileStatus{{Path: "internal/ui/view.go", Worktree: 'M'}}
	m.branches = []git.Branch{{Name: "main", Current: true}}
	m.commits = []git.Commit{{Hash: "37527eeabcd", Subject: "help overlay"}}
	m.stashes = []git.Stash{{Ref: "stash@{0}", Message: "On main: parser cleanup", Age: "12 minutes ago"}}

	if got := strings.Join(m.panelLines(PanelFiles), "\n"); !strings.Contains(got, "M  internal/ui/view.go") {
		t.Fatalf("files panel lines = %q", got)
	}
	if got := strings.Join(m.panelLines(PanelBranches), "\n"); !strings.Contains(got, "* main") {
		t.Fatalf("branches panel lines = %q", got)
	}
	if got := strings.Join(m.panelLines(PanelCommits), "\n"); !strings.Contains(got, "37527ee help overlay") {
		t.Fatalf("commits panel lines = %q", got)
	}
	if got := strings.Join(m.panelLines(PanelStashes), "\n"); !strings.Contains(got, "stash@{0}") || !strings.Contains(got, "On main: parser cleanup") || !strings.Contains(got, "12 minutes ago") {
		t.Fatalf("stashes panel lines = %q", got)
	}
}

func TestRenderPanelFitsRequestedHeight(t *testing.T) {
	m := newTestModel()
	got := m.renderPanel("Files 1", PanelFiles, []panelRow{{text: "M  internal/ui/view.go", kind: panelRowItem, itemIndex: 0}}, 40, 13)
	if height := lipgloss.Height(got); height > 13 {
		t.Fatalf("renderPanel height = %d, want <= 13", height)
	}
}

func TestMainTitleForFocusedSelection(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Model)
		want  string
	}{
		{
			name: "unstaged file diff",
			setup: func(m *Model) {
				m.focus = PanelFiles
				m.files = []git.FileStatus{{Path: "internal/ui/view.go", Worktree: 'M'}}
			},
			want: "internal/ui/view.go | unstaged | 1 of 1",
		},
		{
			name: "staged file diff",
			setup: func(m *Model) {
				m.focus = PanelFiles
				m.files = []git.FileStatus{
					{Path: "a.go", Worktree: 'M'},
					{Path: "README.md", Staged: 'M'},
				}
				m.cursor[PanelFiles] = 1
			},
			want: "README.md | staged | 2 of 2",
		},
		{
			name: "branch log",
			setup: func(m *Model) {
				m.focus = PanelBranches
				m.branches = []git.Branch{{Name: "main", Current: true}}
			},
			want: "Branch log: main",
		},
		{
			name: "commit detail",
			setup: func(m *Model) {
				m.focus = PanelCommits
				m.commits = []git.Commit{{Hash: "37527eeabcd", Subject: "help overlay"}}
			},
			want: "Commit: 37527ee",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel()
			tt.setup(&m)
			if got := m.mainTitle(); got != tt.want {
				t.Fatalf("mainTitle = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCommitScopeHint(t *testing.T) {
	tests := []struct {
		name  string
		files []git.FileStatus
		want  string
	}{
		{"nothing staged", []git.FileStatus{{Path: "a", Worktree: 'M'}}, "Nothing staged — committing all changes"},
		{"one staged", []git.FileStatus{{Path: "a", Staged: 'M'}}, "Committing 1 staged file"},
		{"several staged", []git.FileStatus{{Path: "a", Staged: 'M'}, {Path: "b", Staged: 'A'}}, "Committing 2 staged files"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel()
			m.files = tt.files
			if got := m.commitScopeHint(); got != tt.want {
				t.Fatalf("commitScopeHint = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMainContentCommittingShowsScopeHint(t *testing.T) {
	m := newTestModel()
	m.mode = ModeCommitting
	m.files = []git.FileStatus{{Path: "a.go", Worktree: 'M'}} // unstaged
	got := m.mainContent()
	if !strings.Contains(got, "Commit message") {
		t.Fatalf("mainContent missing prompt heading: %q", got)
	}
	if !strings.Contains(got, "committing all changes") {
		t.Fatalf("mainContent missing scope hint: %q", got)
	}
}

func TestViewCommittingFillsPaneWithinBounds(t *testing.T) {
	m := newTestModel()
	m.w = 120
	m.h = 40
	m.layout()
	m.mode = ModeCommitting
	m.files = []git.FileStatus{{Path: "internal/ui/view.go", Worktree: 'M'}}
	m.subject.SetValue("subject line")

	got := m.View()
	if h := lipgloss.Height(got); h > m.h {
		t.Fatalf("committing View height = %d, want <= %d", h, m.h)
	}
	if w := lipgloss.Width(got); w > m.w {
		t.Fatalf("committing View width = %d, want <= %d", w, m.w)
	}
}

func TestMainTitleForEmptySelection(t *testing.T) {
	m := newTestModel()
	m.focus = PanelFiles
	if got := m.mainTitle(); got != "Working tree clean" {
		t.Fatalf("mainTitle = %q, want %q", got, "Working tree clean")
	}
}

func TestMainContentIncludesHeading(t *testing.T) {
	m := newTestModel()
	m.focus = PanelFiles
	m.files = []git.FileStatus{{Path: "a.go", Worktree: 'M'}}
	m.viewport.Width = 80
	m.viewport.Height = 10
	m.viewport.SetContent("+new line")

	got := m.mainContent()
	if !strings.Contains(got, "a.go | unstaged | 1 of 1") {
		t.Fatalf("mainContent missing heading: %q", got)
	}
	if !strings.Contains(got, "+new line") {
		t.Fatalf("mainContent missing diff: %q", got)
	}
}

func TestMainTitleShowsConflictAndUntrackedState(t *testing.T) {
	tests := []struct {
		name string
		file git.FileStatus
		want string
	}{
		{"conflict", git.FileStatus{Path: "a.go", Unmerged: true, Conflict: "UU"}, "a.go | conflict | 1 of 1"},
		{"untracked", git.FileStatus{Path: "a.go", Untracked: true}, "a.go | untracked | 1 of 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel()
			m.focus = PanelFiles
			m.files = []git.FileStatus{tt.file}
			if got := m.mainTitle(); got != tt.want {
				t.Fatalf("mainTitle = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMainContentShowsLoadingDiffCopy(t *testing.T) {
	m := newTestModel()
	m.focus = PanelFiles
	m.files = []git.FileStatus{{Path: "a.go", Worktree: 'M'}}
	m.viewport.Width = 80
	m.viewport.Height = 10
	m.mainLoading = true

	got := m.mainContent()
	if !strings.Contains(got, "Loading diff...") {
		t.Fatalf("mainContent should show loading copy:\n%s", got)
	}
}

func TestScrollStatusReflectsPosition(t *testing.T) {
	m := newTestModel()
	m.viewport.Width = 80
	m.viewport.Height = 4

	// content that fits → no indicator
	m.viewport.SetContent("one\ntwo")
	if got := m.scrollStatus(); got != "" {
		t.Fatalf("scrollStatus with fitting content = %q, want empty", got)
	}

	// overflowing content
	m.viewport.SetContent(strings.Repeat("x\n", 30))

	m.viewport.GotoTop()
	if top := m.scrollStatus(); !strings.Contains(top, "↓") || strings.Contains(top, "↑") {
		t.Fatalf("at top = %q, want ↓ (more below) and no ↑", top)
	}

	m.viewport.GotoBottom()
	if bot := m.scrollStatus(); !strings.Contains(bot, "↑") || strings.Contains(bot, "↓") {
		t.Fatalf("at bottom = %q, want ↑ (more above) and no ↓", bot)
	}
}

func TestFooterActionsByFocusAndMode(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Model)
		want  string
	}{
		{
			name: "files focus unstaged with nothing staged",
			setup: func(m *Model) {
				m.focus = PanelFiles
				m.files = []git.FileStatus{{Path: "a.go", Worktree: 'M'}}
			},
			want: "Files: space stage · d discard · s stash · c commit all · ? help · q quit",
		},
		{
			name: "files focus unstaged with staged changes elsewhere",
			setup: func(m *Model) {
				m.focus = PanelFiles
				m.files = []git.FileStatus{
					{Path: "a.go", Worktree: 'M'},
					{Path: "b.go", Staged: 'M'},
				}
			},
			want: "Files: space stage · d discard · s stash · c commit staged · ? help · q quit",
		},
		{
			name: "files focus staged",
			setup: func(m *Model) {
				m.focus = PanelFiles
				m.files = []git.FileStatus{{Path: "a.go", Staged: 'M'}}
			},
			want: "Files: space unstage · s stash · c commit staged · ? help · q quit",
		},
		{
			name: "files focus untracked",
			setup: func(m *Model) {
				m.focus = PanelFiles
				m.files = []git.FileStatus{{Path: "a.go", Untracked: true}}
			},
			want: "Files: space stage · d discard · s stash · ? help · q quit",
		},
		{
			name: "branches focus",
			setup: func(m *Model) {
				m.focus = PanelBranches
			},
			want: "Branches: enter switch · c commit · f fetch · p pull · P push · ? help · q quit",
		},
		{
			name: "commits focus",
			setup: func(m *Model) {
				m.focus = PanelCommits
			},
			want: "Commits: space mark · y cherry-pick · / search · c commit · f fetch · p pull · P push · ? help · q quit",
		},
		{
			name: "stashes focus",
			setup: func(m *Model) {
				m.focus = PanelStashes
				m.stashes = []git.Stash{{Ref: "stash@{0}", Message: "On main: save"}}
			},
			want: "Stashes: s save · a apply · o pop · d drop · ? help · q quit",
		},
		{
			name: "confirming mode",
			setup: func(m *Model) {
				m.mode = ModeConfirming
			},
			want: "Confirm: y yes · n no · esc cancel",
		},
		{
			name: "committing mode",
			setup: func(m *Model) {
				m.mode = ModeCommitting
			},
			want: "Commit: ctrl+d submit · tab switch · esc cancel",
		},
		{
			name: "stashing mode",
			setup: func(m *Model) {
				m.mode = ModeStashing
			},
			want: "Stash: ctrl+d save · esc cancel",
		},
		{
			name: "main pane focused",
			setup: func(m *Model) {
				m.focus = PanelFiles
				m.mainFocused = true
			},
			want: "Diff: j/k scroll · g/G top/bot · h back · ? help · q quit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel()
			tt.setup(&m)
			if got := m.footerActions(); got != tt.want {
				t.Fatalf("footerActions = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFooterByState(t *testing.T) {
	// Wide + idle: key hints render (now styled, so assert the hints, not exact text).
	idle := newTestModel()
	idle.focus = PanelFiles
	idle.files = []git.FileStatus{{Path: "a.go", Worktree: 'M'}}
	if got := idle.footer(true); !strings.Contains(got, "stage") || !strings.Contains(got, "quit") {
		t.Fatalf("idle footer = %q, want key hints", got)
	}

	// Busy: spinner-fed working label plus actions.
	busy := newTestModel()
	busy.focus = PanelFiles
	busy.busy = true
	if got := busy.footer(true); !strings.Contains(got, "working") {
		t.Fatalf("busy footer = %q, want a working label", got)
	}

	// Narrow + error: error summary surfaces in the footer.
	failed := newTestModel()
	failed.focus = PanelFiles
	failed.err = errFake("git push failed")
	if got := failed.footer(false); !strings.Contains(got, "error: git push failed") {
		t.Fatalf("narrow error footer = %q, want error summary", got)
	}
}

func TestFooterActionsCommitsShowsCherryPickHints(t *testing.T) {
	m := newTestModel()
	m.focus = PanelCommits

	if got := m.footerActions(); got != "Commits: space mark · y cherry-pick · / search · c commit · f fetch · p pull · P push · ? help · q quit" {
		t.Fatalf("footerActions = %q", got)
	}
}

type errFake string

func (e errFake) Error() string { return string(e) }

func TestFooterBusyShowsCancelHintWhenCancelable(t *testing.T) {
	m := newTestModel()
	m.focus = PanelFiles
	m.busy = true
	m.cancelOp = func() {} // a cancelable remote op is in flight

	if got := m.footer(true); !strings.Contains(got, "esc") {
		t.Errorf("busy cancelable footer should hint esc to cancel: %q", got)
	}
}

func TestFooterBusyWithoutCancelableOpHasNoCancelHint(t *testing.T) {
	m := newTestModel()
	m.focus = PanelFiles
	m.busy = true // fast mutation, nothing to cancel

	if got := m.footer(true); strings.Contains(got, "esc") {
		t.Errorf("non-cancelable busy footer should not mention esc: %q", got)
	}
}

func TestTopBarShowsBranchWorkflowCountsAndCommandState(t *testing.T) {
	m := newTestModel()
	m.branch = git.BranchInfo{Name: "main", Ahead: 2, Behind: 1}
	m.files = []git.FileStatus{{Path: "a.go", Worktree: 'M'}, {Path: "b.go", Staged: 'A'}}
	m.branches = []git.Branch{{Name: "main", Current: true}, {Name: "feature/ui"}}
	m.commits = []git.Commit{{Hash: "abcdef123", Subject: "first"}}
	m.focus = PanelFiles

	got := m.topBar()
	for _, want := range []string{
		"main ↑2 ↓1",
		"[1 Files 2]",
		"2 Branches 2",
		"3 Commits 1",
		"4 Stashes 0",
		"Ready",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("topBar missing %q: %q", want, got)
		}
	}
}

func TestTopBarIncludesStashesWorkflow(t *testing.T) {
	m := newTestModel()
	m.branch = git.BranchInfo{Name: "main"}
	m.stashes = []git.Stash{{Ref: "stash@{0}", Message: "On main: save point"}}
	m.focus = PanelStashes

	got := m.topBar()

	for _, want := range []string{"1 Files 0", "2 Branches 0", "3 Commits 0", "[4 Stashes 1]"} {
		if !strings.Contains(got, want) {
			t.Fatalf("topBar missing %q: %q", want, got)
		}
	}
}

func TestCommandStateText(t *testing.T) {
	ready := newTestModel()
	if got := ready.commandStateText(); got != "Ready" {
		t.Fatalf("ready commandStateText = %q", got)
	}

	busy := newTestModel()
	busy.busy = true
	if got := busy.commandStateText(); got != "Working..." {
		t.Fatalf("busy commandStateText = %q", got)
	}

	failed := newTestModel()
	failed.err = errFake("git push failed")
	if got := failed.commandStateText(); got != "Error" {
		t.Fatalf("error commandStateText = %q", got)
	}
}

func TestFormatCmdEntryShowsHHMM(t *testing.T) {
	e := cmdEntry{at: time.Date(2026, 6, 11, 10, 21, 0, 0, time.UTC), text: "git fetch"}
	if got := formatCmdEntry(e); got != "10:21 git fetch" {
		t.Fatalf("formatCmdEntry = %q, want %q", got, "10:21 git fetch")
	}
}

func TestRecentCommandLinesAreNewestFirstAndCapped(t *testing.T) {
	m := newTestModel()
	m.cmdLog = []cmdEntry{
		{at: time.Date(2026, 6, 11, 10, 20, 0, 0, time.UTC), text: "git fetch"},
		{at: time.Date(2026, 6, 11, 10, 21, 0, 0, time.UTC), text: "git add README.md"},
		{at: time.Date(2026, 6, 11, 10, 22, 0, 0, time.UTC), text: "git commit"},
	}

	got := m.recentCommandLines(2)
	want := []string{"10:22 git commit", "10:21 git add README.md"}
	if len(got) != len(want) {
		t.Fatalf("recentCommandLines length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("recentCommandLines[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSelectedContextLines(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Model)
		want  []string
	}{
		{
			name: "unstaged file",
			setup: func(m *Model) {
				m.focus = PanelFiles
				m.files = []git.FileStatus{{Path: "internal/ui/view.go", Worktree: 'M'}}
			},
			want: []string{"internal/ui/view.go", "unstaged file", "actions: space stage, d discard, s stash, c commit all"},
		},
		{
			name: "staged file",
			setup: func(m *Model) {
				m.focus = PanelFiles
				m.files = []git.FileStatus{{Path: "README.md", Staged: 'A'}}
			},
			want: []string{"README.md", "staged file", "actions: space unstage, s stash, c commit staged"},
		},
		{
			name: "branch",
			setup: func(m *Model) {
				m.focus = PanelBranches
				m.branches = []git.Branch{{Name: "main", Current: true}}
			},
			want: []string{"main", "current branch", "actions: commit, fetch, pull, push"},
		},
		{
			name: "commit",
			setup: func(m *Model) {
				m.focus = PanelCommits
				m.commits = []git.Commit{{Hash: "abcdef123456", Subject: "focus mode"}}
			},
			want: []string{"abcdef1", "focus mode", "actions: space mark, y cherry-pick, / search, c commit, fetch, pull, push"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel()
			tt.setup(&m)
			got := strings.Join(m.selectedContextLines(), "\n")
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Fatalf("selectedContextLines missing %q: %q", want, got)
				}
			}
		})
	}
}

func TestSelectedContextLinesShowConcreteFileActions(t *testing.T) {
	tests := []struct {
		name  string
		files []git.FileStatus
		want  []string
	}{
		{
			name:  "unstaged",
			files: []git.FileStatus{{Path: "a.go", Worktree: 'M'}},
			want:  []string{"a.go", "unstaged file", "actions: space stage, d discard, s stash, c commit all"},
		},
		{
			name:  "staged",
			files: []git.FileStatus{{Path: "a.go", Staged: 'M'}},
			want:  []string{"a.go", "staged file", "actions: space unstage, s stash, c commit staged"},
		},
		{
			name:  "untracked",
			files: []git.FileStatus{{Path: "a.go", Untracked: true}},
			want:  []string{"a.go", "untracked file", "actions: space stage, d discard, s stash"},
		},
		{
			name:  "conflict",
			files: []git.FileStatus{{Path: "a.go", Unmerged: true, Conflict: "UU"}},
			want:  []string{"a.go", "conflict: both modified", "actions: e edit, space resolve, A abort, c commit"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel()
			m.focus = PanelFiles
			m.files = tt.files
			got := strings.Join(m.selectedContextLines(), "\n")
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Fatalf("selectedContextLines missing %q:\n%s", want, got)
				}
			}
		})
	}
}

func TestSelectedContextLinesShowsCommitMetadata(t *testing.T) {
	m := newTestModel()
	m.focus = PanelCommits
	m.commits = []git.Commit{{Hash: "abcdef123456", Subject: "focus mode", Author: "Kael", RelTime: "2 hours ago"}}

	got := strings.Join(m.selectedContextLines(), "\n")
	for _, want := range []string{"abcdef1", "focus mode", "Kael", "2 hours ago"} {
		if !strings.Contains(got, want) {
			t.Fatalf("selectedContextLines missing %q:\n%s", want, got)
		}
	}
}

func TestSelectedContextLinesShowsCherryPickSelectionCount(t *testing.T) {
	m := newTestModel()
	m.focus = PanelCommits
	m.commits = []git.Commit{{Hash: "abcdef123456", Subject: "fix auth"}}
	m.selectedCommits = map[string]bool{"abcdef123456": true}

	got := strings.Join(m.selectedContextLines(), "\n")
	for _, want := range []string{"abcdef1", "fix auth", "Cherry-pick: 1 selected", "actions: space mark, y cherry-pick, / search, c commit, fetch, pull, push"} {
		if !strings.Contains(got, want) {
			t.Fatalf("selected context missing %q:\n%s", want, got)
		}
	}
}

func TestSelectedContextLinesShowsStashActions(t *testing.T) {
	m := newTestModel()
	m.focus = PanelStashes
	m.stashes = []git.Stash{{Ref: "stash@{0}", Message: "On main: save", Branch: "main", Age: "3 minutes ago"}}

	got := strings.Join(m.selectedContextLines(), "\n")

	for _, want := range []string{"stash@{0}", "On main: save", "main", "3 minutes ago", "actions: s save, a apply, o pop, d drop"} {
		if !strings.Contains(got, want) {
			t.Fatalf("selected context missing %q:\n%s", want, got)
		}
	}
}

func TestStatusRailContentShowsWorkflowCommandRecentAndSelectedContext(t *testing.T) {
	m := newTestModel()
	m.focus = PanelFiles
	m.files = []git.FileStatus{{Path: "internal/ui/view.go", Worktree: 'M'}}
	m.branches = []git.Branch{{Name: "main", Current: true}, {Name: "feature/ui"}}
	m.commits = []git.Commit{{Hash: "abcdef123", Subject: "focus mode"}}
	m.cmdLog = []cmdEntry{
		{at: time.Date(2026, 6, 11, 10, 20, 0, 0, time.UTC), text: "git fetch"},
		{at: time.Date(2026, 6, 11, 10, 21, 0, 0, time.UTC), text: "git add README.md"},
	}

	got := m.statusRailContent()
	for _, want := range []string{
		"Status Rail",
		"Workflow",
		"Files: 1 changed",
		"Branches: 2 local",
		"Commits: 1 loaded",
		"Command",
		"Ready",
		"Last: 10:21 git add README.md",
		"Recent",
		"git add README.md",
		"git fetch",
		"Selected",
		"internal/ui/view.go",
		"unstaged file",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("statusRailContent missing %q:\n%s", want, got)
		}
	}
}

func TestStatusRailContentShowsError(t *testing.T) {
	m := newTestModel()
	m.err = errFake("git push failed")

	got := m.statusRailContent()
	for _, want := range []string{"Command", "Error", "git push failed"} {
		if !strings.Contains(got, want) {
			t.Fatalf("statusRailContent missing %q:\n%s", want, got)
		}
	}
}

func TestStatusRailContentShowsCherryPickSelection(t *testing.T) {
	m := newTestModel()
	m.focus = PanelCommits
	m.commits = []git.Commit{{Hash: "abcdef123456", Subject: "fix auth"}}
	m.selectedCommits = map[string]bool{"abcdef123456": true}

	got := m.statusRailContent()
	if !strings.Contains(got, "Cherry-pick: 1 selected") {
		t.Fatalf("status rail missing cherry-pick selection:\n%s", got)
	}
}

func TestViewRendersFocusMode(t *testing.T) {
	m := newTestModel()
	m.w = 120
	m.h = 40
	m.layout()
	m.branch = git.BranchInfo{Name: "main", Ahead: 2}
	m.focus = PanelFiles
	m.files = []git.FileStatus{{Path: "internal/ui/view.go", Worktree: 'M'}}
	m.branches = []git.Branch{{Name: "main", Current: true}}
	m.commits = []git.Commit{{Hash: "37527eeabcd", Subject: "help overlay"}}
	m.viewport.SetContent("+new line")

	got := m.View()
	for _, want := range []string{
		"[1 Files 1]", // focused workflow tab in the top bar
		"main ↑2 ↓0",  // branch summary in the top bar
		"Status Rail", // rail visible at width 120
		"internal/ui/view.go | unstaged | 1 of 1", // preview heading
		"stage", // footer key hint
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("View missing %q:\n%s", want, got)
		}
	}
	// Only the focused (Files) list renders in the body; the Commits list does not.
	if strings.Contains(got, "37527ee help overlay") {
		t.Fatalf("non-focused commit row leaked into Focus Mode body:\n%s", got)
	}
}

func TestMainTitleForStashSelection(t *testing.T) {
	m := newTestModel()
	m.focus = PanelStashes
	m.stashes = []git.Stash{{Ref: "stash@{0}", Message: "On main: save point", Age: "3 minutes ago"}}

	if got := m.mainTitle(); got != "stash@{0} | On main: save point | 1 of 1" {
		t.Fatalf("mainTitle = %q", got)
	}
}

func TestStashEditorView(t *testing.T) {
	m := newTestModel()
	m.w, m.h = 120, 40
	m.layout()
	m.mode = ModeStashing
	m.stashMessage.SetValue("save point")

	got := m.View()

	for _, want := range []string{"Save stash", "Message", "save point", "Ctrl-D save", "Esc cancel"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stash editor missing %q:\n%s", want, got)
		}
	}
}

func TestViewStashesUsesNormalLayoutBounds(t *testing.T) {
	m := newTestModel()
	m.w = 120
	m.h = 40
	m.layout()
	m.branch = git.BranchInfo{Name: "main"}
	m.focus = PanelStashes
	m.stashes = []git.Stash{{Ref: "stash@{0}", Message: "On main: save", Age: "3 minutes ago"}}
	m.viewport.SetContent("diff --git a/a.go b/a.go\n+new\n")

	got := m.View()

	if height := lipgloss.Height(got); height > m.h {
		t.Fatalf("View height = %d, want <= %d", height, m.h)
	}
	if width := lipgloss.Width(got); width > m.w {
		t.Fatalf("View width = %d, want <= %d", width, m.w)
	}
	for _, want := range []string{"[4 Stashes 1]", "stash@{0}", "Status Rail", "apply"} {
		if !strings.Contains(got, want) {
			t.Fatalf("View missing %q:\n%s", want, got)
		}
	}
}

func TestCommandLogOverlayShowsTimestampedHistoryNewestFirst(t *testing.T) {
	m := newTestModel()
	m.w, m.h = 80, 24
	m.cmdLog = []cmdEntry{
		{at: time.Date(2026, 6, 11, 10, 20, 0, 0, time.UTC), text: "git fetch"},
		{at: time.Date(2026, 6, 11, 10, 22, 0, 0, time.UTC), text: "git commit"},
	}

	got := m.commandLogOverlay()
	if !strings.Contains(got, "command log") {
		t.Fatalf("overlay missing title:\n%s", got)
	}
	ci := strings.Index(got, "10:22 git commit")
	fi := strings.Index(got, "10:20 git fetch")
	if ci < 0 || fi < 0 || ci > fi {
		t.Fatalf("expected newest-first order, commit@%d fetch@%d:\n%s", ci, fi, got)
	}
}

func TestCommandLogOverlayShowsOutput(t *testing.T) {
	m := newTestModel()
	m.w, m.h = 80, 24
	m.cmdLog = []cmdEntry{
		{at: time.Date(2026, 6, 11, 10, 22, 0, 0, time.UTC), text: "git push", output: "Everything up-to-date"},
	}

	got := m.commandLogOverlay()
	if !strings.Contains(got, "git push") {
		t.Fatalf("overlay missing command line:\n%s", got)
	}
	if !strings.Contains(got, "Everything up-to-date") {
		t.Fatalf("overlay should render the command output:\n%s", got)
	}
}

func TestViewInitialLoadingCopy(t *testing.T) {
	m := newTestModel()
	if got := m.View(); got != "Loading repository..." {
		t.Fatalf("View without size = %q, want Loading repository...", got)
	}
}

func TestViewFitsTerminalHeight(t *testing.T) {
	m := newTestModel()
	m.w = 120
	m.h = 40
	m.layout()
	m.branch = git.BranchInfo{Name: "main"}
	m.files = []git.FileStatus{{Path: "internal/ui/view.go", Worktree: 'M'}}
	m.branches = []git.Branch{{Name: "main", Current: true}}
	m.commits = []git.Commit{{Hash: "37527eeabcd", Subject: "help overlay"}}
	m.viewport.SetContent("+new line")

	got := m.View()
	if height := lipgloss.Height(got); height > m.h {
		t.Fatalf("View height = %d, want <= %d", height, m.h)
	}
}

func TestViewFitsTerminalWidth(t *testing.T) {
	m := newTestModel()
	m.w = 120
	m.h = 40
	m.layout()
	m.branch = git.BranchInfo{Name: "main"}
	m.files = []git.FileStatus{{Path: "internal/ui/view.go", Worktree: 'M'}}
	m.branches = []git.Branch{{Name: "main", Current: true}}
	m.commits = []git.Commit{{Hash: "37527eeabcd", Subject: "x"}}
	m.viewport.SetContent("+new line")

	if w := lipgloss.Width(m.View()); w > m.w {
		t.Fatalf("View width = %d, want <= %d", w, m.w)
	}
}

func TestViewNarrowHidesRailWithinHeight(t *testing.T) {
	m := newTestModel()
	m.w = 80 // below minRailWindowWide (110)
	m.h = 24
	m.layout()
	m.branch = git.BranchInfo{Name: "main"}
	m.files = []git.FileStatus{{Path: "a.go", Worktree: 'M'}}
	m.viewport.SetContent("+new line")

	got := m.View()
	if strings.Contains(got, "Status Rail") {
		t.Fatalf("rail should be hidden below width 110:\n%s", got)
	}
	if h := lipgloss.Height(got); h > m.h {
		t.Fatalf("narrow View height = %d, want <= %d", h, m.h)
	}
}

func TestCommandStateShowsNotice(t *testing.T) {
	m := newTestModel()
	m.notice = "Committed a1b2c3d feat: x"
	if got := m.commandStateText(); got != "Committed a1b2c3d feat: x" {
		t.Errorf("commandStateText = %q, want the notice", got)
	}
	m.branch = git.BranchInfo{Name: "main"}
	if got := m.topBar(); !strings.Contains(got, "Committed a1b2c3d feat: x") {
		t.Errorf("topBar should show the notice: %q", got)
	}
}

func TestCommitEditorShowsCounterAndFields(t *testing.T) {
	m := newTestModel()
	m.w, m.h = 120, 40
	m.layout()
	m.mode = ModeCommitting
	m.subject.SetValue("feat: add body")
	got := m.View()
	for _, want := range []string{"Subject", "Body", "14/50", "Tab switch"} {
		if !strings.Contains(got, want) {
			t.Fatalf("commit editor missing %q:\n%s", want, got)
		}
	}
}

func TestAmendScopeHint(t *testing.T) {
	cases := []struct {
		name  string
		files []git.FileStatus
		want  string
	}{
		{"message only", nil, "Amending HEAD (message only)"},
		{"one staged", []git.FileStatus{{Path: "a", Staged: 'M'}}, "Amending HEAD + 1 staged file"},
		{"two staged", []git.FileStatus{{Path: "a", Staged: 'M'}, {Path: "b", Staged: 'A'}}, "Amending HEAD + 2 staged files"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := newTestModel()
			m.files = c.files
			if got := m.amendScopeHint(); got != c.want {
				t.Errorf("amendScopeHint = %q, want %q", got, c.want)
			}
		})
	}
}

func TestCommitEditorAmendShowsWarningWhenPushed(t *testing.T) {
	m := newTestModel()
	m.w, m.h = 120, 40
	m.layout()
	m.mode = ModeCommitting
	m.amending = true
	m.subject.SetValue("feat: x")
	m.branch = git.BranchInfo{Name: "main", Upstream: "origin/main", Ahead: 0}
	got := m.View()
	for _, want := range []string{"Amend commit", "already pushed"} {
		if !strings.Contains(got, want) {
			t.Fatalf("amend editor missing %q:\n%s", want, got)
		}
	}
}

func TestMergeBanner(t *testing.T) {
	m := newTestModel()
	if got := m.mergeBanner(); got != "" {
		t.Errorf("not merging: banner = %q, want empty", got)
	}
	m.merging = true
	m.files = []git.FileStatus{{Path: "a", Unmerged: true}}
	if got := m.mergeBanner(); got != "MERGING — 1 conflict" {
		t.Errorf("banner = %q, want 'MERGING — 1 conflict'", got)
	}
	m.files = []git.FileStatus{{Path: "a", Unmerged: true}, {Path: "b", Unmerged: true}}
	if got := m.mergeBanner(); got != "MERGING — 2 conflicts" {
		t.Errorf("banner = %q, want 'MERGING — 2 conflicts'", got)
	}
	m.files = nil
	if got := m.mergeBanner(); got != "MERGING — ready to commit" {
		t.Errorf("banner = %q, want 'MERGING — ready to commit'", got)
	}
}

func TestFooterConflictHints(t *testing.T) {
	m := newTestModel()
	m.merging = true
	m.focus = PanelFiles
	m.files = []git.FileStatus{{Path: "a.go", Unmerged: true, Conflict: "UU"}}
	if got := m.footerActions(); got != "Files: e edit · space resolve · A abort · c commit · ? help · q quit" {
		t.Errorf("footer = %q", got)
	}
}

func TestSelectedContextShowsConflictKind(t *testing.T) {
	m := newTestModel()
	m.focus = PanelFiles
	m.files = []git.FileStatus{{Path: "a.go", Unmerged: true, Conflict: "UU"}}
	joined := strings.Join(m.selectedContextLines(), "\n")
	if !strings.Contains(joined, "conflict: both modified") {
		t.Errorf("selected context = %q, want conflict kind", joined)
	}
}

func TestCommitSearchEditorViewShowsFieldsAndSelections(t *testing.T) {
	m := newTestModel()
	m.w, m.h = 120, 40
	m.layout()
	m.mode = ModeCommitSearch
	m.commitSearch.Field = searchFieldQuery
	m.commitSearch.Branch = "main"
	m.commitSearch.Author = "Kael"
	m.commitQuery.SetValue("fix auth")

	got := m.View()
	for _, want := range []string{"Commit Search", "Query", "fix auth", "Branch", "main", "Author", "Kael", "Enter search", "Esc cancel"} {
		if !strings.Contains(got, want) {
			t.Fatalf("commit search view missing %q:\n%s", want, got)
		}
	}
}

func TestFooterActionsCommitSearchMode(t *testing.T) {
	m := newTestModel()
	m.mode = ModeCommitSearch

	if got := m.footerActions(); got != "Search: enter apply · tab switch · j/k choose · esc cancel" {
		t.Fatalf("footerActions = %q", got)
	}
}

func TestTopBarShowsCommitSearchSummary(t *testing.T) {
	m := newTestModel()
	m.branch = git.BranchInfo{Name: "main"}
	m.commitSearch.Active = true
	m.commitSearch.Summary = `Search: "fix" | branch main`

	got := m.topBar()
	if !strings.Contains(got, `Search: "fix" | branch main`) {
		t.Fatalf("topBar missing search summary:\n%s", got)
	}
}

func TestStatusRailContentShowsActiveCommitSearch(t *testing.T) {
	m := newTestModel()
	m.focus = PanelCommits
	m.commitSearch.Active = true
	m.commitSearch.Summary = `Search: "fix" | branch main`

	got := m.statusRailContent()
	for _, want := range []string{"Commit Search", `Search: "fix" | branch main`} {
		if !strings.Contains(got, want) {
			t.Fatalf("status rail missing %q:\n%s", want, got)
		}
	}
}

func TestSelectedContextLinesShowsSearchActionsForCommits(t *testing.T) {
	m := newTestModel()
	m.focus = PanelCommits
	m.commits = []git.Commit{{Hash: "abcdef123456", Subject: "fix auth"}}

	got := strings.Join(m.selectedContextLines(), "\n")
	if !strings.Contains(got, "actions: space mark, y cherry-pick, / search, c commit, fetch, pull, push") {
		t.Fatalf("selected context missing search action:\n%s", got)
	}
}

func TestHelpOverlayShowsCherryPickKeys(t *testing.T) {
	m := newTestModel()
	m.w, m.h = 100, 30

	got := m.helpOverlay()
	for _, want := range []string{"space           stage / unstage file, or mark commit", "y               cherry-pick marked commits from Commits"} {
		if !strings.Contains(got, want) {
			t.Fatalf("help overlay missing %q:\n%s", want, got)
		}
	}
}
