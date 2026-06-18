package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/kael02/loom/internal/git"
)

func TestWindowLines(t *testing.T) {
	lines := []string{"a", "b", "c", "d", "e"}

	t.Run("fits entirely", func(t *testing.T) {
		vis, below := windowLines(lines, 0, 10)
		if len(vis) != 5 || below != 0 {
			t.Fatalf("vis=%v below=%d, want all/0", vis, below)
		}
	})

	t.Run("windowed from offset", func(t *testing.T) {
		vis, below := windowLines(lines, 1, 2)
		if len(vis) != 2 || vis[0] != "b" || vis[1] != "c" || below != 2 {
			t.Fatalf("vis=%v below=%d, want [b c]/2", vis, below)
		}
	})

	t.Run("zero height", func(t *testing.T) {
		vis, below := windowLines(lines, 0, 0)
		if len(vis) != 0 || below != 0 {
			t.Fatalf("vis=%v below=%d, want []/0", vis, below)
		}
	})
}

func TestStyledPanelLinesMarksSelectedRow(t *testing.T) {
	m := newTestModel()
	m.focus = PanelFiles
	m.files = []git.FileStatus{
		{Path: "a.go", Worktree: 'M'},
		{Path: "b.go", Worktree: 'M'},
	}
	m.cursor[PanelFiles] = 1

	styled := m.styledPanelLines(PanelFiles, m.panelLines(PanelFiles), 30)
	if len(styled) != 3 {
		t.Fatalf("got %d rows, want 3", len(styled))
	}
	// the selected row is the full-width highlight bar; the non-selected row is not.
	if w := lipgloss.Width(styled[2]); w != 30 {
		t.Errorf("selected row width = %d, want 30 (highlight bar)", w)
	}
	if w := lipgloss.Width(styled[1]); w >= 30 {
		t.Errorf("non-selected row should be narrower than the bar: width %d", w)
	}
	// gutter keeps non-selected rows aligned with the selected row's text.
	if !strings.HasPrefix(styled[1], caretGutter) {
		t.Errorf("non-selected row missing aligned gutter: %q", styled[1])
	}
}

func TestStyledPanelLinesHighlightOnlyOnFocusedPanel(t *testing.T) {
	m := newTestModel()
	m.focus = PanelBranches // not Files
	m.files = []git.FileStatus{{Path: "a.go", Worktree: 'M'}}
	m.cursor[PanelFiles] = 0

	styled := m.styledPanelLines(PanelFiles, m.panelLines(PanelFiles), 30)
	// an unfocused panel's cursor row is not the full-width highlight bar
	if w := lipgloss.Width(styled[0]); w >= 30 {
		t.Errorf("unfocused panel row should not be highlighted full-width: width %d", w)
	}
}

func TestFilePanelRowsGroupFilesByReviewState(t *testing.T) {
	m := newTestModel()
	m.files = []git.FileStatus{
		{Path: "unstaged.go", Worktree: 'M'},
		{Path: "staged.go", Staged: 'M'},
		{Path: "new.go", Untracked: true},
		{Path: "conflict.go", Unmerged: true, Conflict: "UU"},
		{Path: "also-unstaged.go", Worktree: 'D'},
	}

	rows := m.filePanelRows()
	got := make([]string, len(rows))
	for i, row := range rows {
		got[i] = row.text
	}
	want := []string{
		"Conflicts",
		"!  conflict.go",
		"Staged",
		"+  staged.go",
		"Unstaged",
		"M  unstaged.go",
		"D  also-unstaged.go",
		"Untracked",
		"?  new.go",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("filePanelRows = %#v, want %#v", got, want)
	}

	indexes := []int{}
	for _, row := range rows {
		if row.kind == panelRowItem {
			indexes = append(indexes, row.itemIndex)
		}
	}
	wantIndexes := []int{3, 1, 0, 4, 2}
	if len(indexes) != len(wantIndexes) {
		t.Fatalf("item indexes = %#v, want %#v", indexes, wantIndexes)
	}
	for i := range wantIndexes {
		if indexes[i] != wantIndexes[i] {
			t.Fatalf("item indexes = %#v, want %#v", indexes, wantIndexes)
		}
	}
}

func TestFilePanelRowsOmitEmptyGroups(t *testing.T) {
	m := newTestModel()
	m.files = []git.FileStatus{
		{Path: "staged.go", Staged: 'M'},
	}

	rows := m.filePanelRows()
	got := make([]string, len(rows))
	for i, row := range rows {
		got[i] = row.text
	}
	want := []string{"Staged", "+  staged.go"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("filePanelRows = %#v, want %#v", got, want)
	}
}

func TestFilePanelRowsEmptyCopy(t *testing.T) {
	m := newTestModel()

	rows := m.filePanelRows()
	if len(rows) != 1 {
		t.Fatalf("rows len = %d, want 1", len(rows))
	}
	if rows[0].kind != panelRowEmpty || rows[0].text != "No changes" {
		t.Fatalf("empty row = %#v, want No changes empty row", rows[0])
	}
}

func TestStyledPanelRowsSelectsFileByOriginalIndex(t *testing.T) {
	m := newTestModel()
	m.focus = PanelFiles
	m.files = []git.FileStatus{
		{Path: "unstaged.go", Worktree: 'M'},
		{Path: "staged.go", Staged: 'M'},
	}
	m.cursor[PanelFiles] = 0

	styled := m.styledPanelRows(PanelFiles, m.panelRows(PanelFiles), 32)
	if len(styled) != 4 {
		t.Fatalf("styled rows len = %d, want 4", len(styled))
	}
	if w := lipgloss.Width(styled[3]); w != 32 {
		t.Fatalf("selected unstaged file row width = %d, want 32", w)
	}
	if w := lipgloss.Width(styled[1]); w >= 32 {
		t.Fatalf("staged row should not be selected, width = %d", w)
	}
}

func TestStyledPanelRowsRenderHeadersMutedAndUnselected(t *testing.T) {
	m := newTestModel()
	m.focus = PanelFiles
	m.files = []git.FileStatus{
		{Path: "staged.go", Staged: 'M'},
	}
	m.cursor[PanelFiles] = 0

	styled := m.styledPanelRows(PanelFiles, m.panelRows(PanelFiles), 32)
	if len(styled) != 2 {
		t.Fatalf("styled rows len = %d, want 2", len(styled))
	}
	if !strings.Contains(styled[0], "Staged") {
		t.Fatalf("header row missing label: %q", styled[0])
	}
	if w := lipgloss.Width(styled[0]); w >= 32 {
		t.Fatalf("header row should not render as selected bar, width = %d", w)
	}
}

func TestStyledPanelLinesWrapsCommitRowsTheSameWhenSelected(t *testing.T) {
	m := newTestModel()
	m.focus = PanelCommits
	m.commits = []git.Commit{
		{Hash: "abcdef1234567890", Subject: "this subject is long enough to wrap without being truncated"},
		{Hash: "123456abcdef7890", Subject: "this subject is long enough to wrap without being truncated"},
	}
	m.cursor[PanelCommits] = 1

	const width = 32
	styled := m.styledPanelLines(PanelCommits, m.panelLines(PanelCommits), width)
	if len(styled) != 2 {
		t.Fatalf("got %d rows, want 2", len(styled))
	}
	unselectedLines := strings.Split(styled[0], "\n")
	selectedLines := strings.Split(styled[1], "\n")
	if len(unselectedLines) < 2 {
		t.Fatalf("unselected row did not wrap: %q", styled[0])
	}
	if len(selectedLines) != len(unselectedLines) {
		t.Fatalf("selected row wrapped to %d lines, want %d\nselected=%q\nunselected=%q", len(selectedLines), len(unselectedLines), styled[1], styled[0])
	}
	for _, row := range styled {
		for _, line := range strings.Split(row, "\n") {
			if w := lipgloss.Width(line); w != width {
				t.Fatalf("visual line width = %d, want %d in %q", w, width, row)
			}
		}
		if !strings.Contains(row, "without being") || !strings.Contains(row, "truncated") {
			t.Fatalf("row should preserve the full subject: %q", row)
		}
	}
}

func TestListPaneYieldsFocusToMainPane(t *testing.T) {
	m := newTestModel()
	m.focus = PanelFiles
	if !m.listPaneFocused(PanelFiles) {
		t.Fatal("list pane should hold focus when mainFocused is false")
	}
	m.mainFocused = true
	if m.listPaneFocused(PanelFiles) {
		t.Fatal("list pane should yield focus to the main pane when mainFocused")
	}
}

func TestMarkerColor(t *testing.T) {
	tests := []struct {
		name string
		f    git.FileStatus
		want lipgloss.Color
	}{
		{"staged", git.FileStatus{Staged: 'M'}, colStaged},
		{"modified worktree", git.FileStatus{Worktree: 'M'}, colModified},
		{"untracked", git.FileStatus{Untracked: true}, colUntracked},
		{"unmerged", git.FileStatus{Unmerged: true}, colUnmerged},
		{"deleted worktree", git.FileStatus{Worktree: 'D'}, colDeleted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := markerColor(tt.f); got != tt.want {
				t.Errorf("markerColor(%+v) = %q, want %q", tt.f, got, tt.want)
			}
		})
	}
}

func TestClassifyDiffLine(t *testing.T) {
	tests := []struct {
		line string
		want diffKind
	}{
		{"@@ -10,6 +10,9 @@ func View()", kindHunk},
		{"+new line", kindAdd},
		{"-old line", kindDel},
		{"+++ b/view.go", kindMeta},
		{"--- a/view.go", kindMeta},
		{"diff --git a/view.go b/view.go", kindMeta},
		{"index 1234567..89abcde 100644", kindMeta},
		{"commit 0d1ac66cafef00d", kindMeta},
		{"Author: Kael <k@example.com>", kindMeta},
		{"Date:   Wed Jun 11 10:00:00 2026", kindMeta},
		{" context line", kindContext},
		{"", kindContext},
	}
	for _, tt := range tests {
		if got := classifyDiffLine(tt.line); got != tt.want {
			t.Errorf("classifyDiffLine(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestRenderPanelShowsOverflowHint(t *testing.T) {
	m := newTestModel()
	m.focus = PanelFiles
	m.files = make([]git.FileStatus, 20)
	for i := range m.files {
		m.files[i] = git.FileStatus{Path: "f.go", Worktree: 'M'}
	}
	got := m.renderPanel("Files 20", PanelFiles, m.panelRows(PanelFiles), 30, 8)
	if !strings.Contains(got, "more") {
		t.Fatalf("expected overflow hint in:\n%s", got)
	}
}

func TestStyledPanelLinesSelectionFillsWidth(t *testing.T) {
	m := newTestModel()
	m.focus = PanelFiles
	m.files = []git.FileStatus{{Path: "a.go", Worktree: 'M'}}
	m.cursor[PanelFiles] = 0

	const width = 30
	styled := m.styledPanelLines(PanelFiles, m.panelLines(PanelFiles), width)

	// the selected row's caret + highlight bar should span the full panel width
	if w := lipgloss.Width(styled[1]); w != width {
		t.Fatalf("selected row width = %d, want %d (full-width bar)", w, width)
	}
}

func TestConflictLabel(t *testing.T) {
	cases := map[string]string{
		"UU": "both modified",
		"AA": "both added",
		"DD": "both deleted",
		"AU": "added by us",
		"UD": "deleted by them",
		"UA": "added by them",
		"DU": "deleted by us",
		"":   "unmerged",
		"ZZ": "unmerged",
	}
	for code, want := range cases {
		if got := conflictLabel(code); got != want {
			t.Errorf("conflictLabel(%q) = %q, want %q", code, got, want)
		}
	}
}
