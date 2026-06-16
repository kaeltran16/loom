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
	if len(styled) != 2 {
		t.Fatalf("got %d rows, want 2", len(styled))
	}
	// the selected row is the full-width highlight bar; the non-selected row is not.
	if w := lipgloss.Width(styled[1]); w != 30 {
		t.Errorf("selected row width = %d, want 30 (highlight bar)", w)
	}
	if w := lipgloss.Width(styled[0]); w >= 30 {
		t.Errorf("non-selected row should be narrower than the bar: width %d", w)
	}
	// gutter keeps non-selected rows aligned with the selected row's text.
	if !strings.HasPrefix(styled[0], caretGutter) {
		t.Errorf("non-selected row missing aligned gutter: %q", styled[0])
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

func TestRenderPanelShowsOverflowHint(t *testing.T) {
	m := newTestModel()
	m.focus = PanelFiles
	m.files = make([]git.FileStatus, 20)
	for i := range m.files {
		m.files[i] = git.FileStatus{Path: "f.go", Worktree: 'M'}
	}
	lines := m.panelLines(PanelFiles)

	got := m.renderPanel("Files 20", PanelFiles, lines, 30, 8)
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
	if w := lipgloss.Width(styled[0]); w != width {
		t.Fatalf("selected row width = %d, want %d (full-width bar)", w, width)
	}
}
