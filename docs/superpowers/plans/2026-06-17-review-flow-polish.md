# Review Flow Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Polish Loom's existing Files review workflow so changed files are grouped by review state, selected-file actions are clearer, and the main pane shows review position without adding Git capabilities.

**Architecture:** Keep `m.cursor[PanelFiles]` as an index into `m.files` so existing file actions remain stable. Add a render-only row projection for the Files panel that inserts non-selectable group headers and maps each visible file row back to the original file index. Update view helpers to reuse the same selected-file state/action helpers for the footer, status rail, and main title.

**Tech Stack:** Go 1.22, Bubble Tea, Bubbles viewport, Lip Gloss, existing `internal/git.FileStatus`, existing `go test ./internal/ui`.

---

## Pre-Implementation Notes

The repository may already contain unrelated staged or unstaged changes. Before each task, run:

```bash
git status --short
```

Only stage files touched by the current task. Do not revert unrelated changes.

The approved spec is:

```text
docs/superpowers/specs/2026-06-17-review-flow-polish-design.md
```

## File Structure

- Modify `internal/ui/panels.go`
  - Add `panelRow` metadata and Files grouping helpers.
  - Keep existing plain-text helpers where useful.
  - Update panel rendering to style headers, empty rows, and selected file rows correctly.
- Modify `internal/ui/update.go`
  - Keep `cursor[PanelFiles]` as a real file index.
  - Update scroll math so grouped visual rows stay in view.
  - Set and clear a small loading flag for main-pane loading copy.
- Modify `internal/ui/model.go`
  - Add `mainLoading bool` for loading-state copy.
- Modify `internal/ui/view.go`
  - Use selected-file state/action helpers in the status rail, footer hints, and main title.
  - Render review position as `N of M` for file selections.
  - Render specific empty/loading copy.
- Modify `internal/ui/panels_test.go`
  - Add pure tests for file row grouping and selected-row styling after headers.
- Modify `internal/ui/update_test.go`
  - Add reducer tests for cursor and scroll behavior with grouped rows.
  - Add loading-state tests for `mainLoading`.
- Modify `internal/ui/view_test.go`
  - Update expected footer/title strings and add staged/unstaged/untracked/conflict context tests.

Do not modify `internal/git` for this work.

---

### Task 1: Add Files Review Row Projection Tests

**Files:**
- Modify: `internal/ui/panels_test.go`
- Planned implementation target: `internal/ui/panels.go`

- [ ] **Step 1: Write failing tests for grouped file rows**

Add these tests to `internal/ui/panels_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/ui -run "TestFilePanelRows" -count=1
```

Expected: FAIL because `filePanelRows`, `panelRowItem`, and `panelRowEmpty` do not exist yet.

- [ ] **Step 3: Commit only the failing tests**

```bash
git add internal/ui/panels_test.go
git commit -m "test(ui): cover review file row grouping"
```

---

### Task 2: Implement Files Review Row Projection

**Files:**
- Modify: `internal/ui/panels.go`
- Test: `internal/ui/panels_test.go`

- [ ] **Step 1: Add row metadata types and grouping helpers**

In `internal/ui/panels.go`, add these declarations near the panel-line helpers:

```go
type panelRowKind int

const (
	panelRowItem panelRowKind = iota
	panelRowHeader
	panelRowEmpty
)

type panelRow struct {
	text      string
	kind      panelRowKind
	itemIndex int
}

type fileReviewGroup struct {
	label string
	match func(git.FileStatus) bool
}

var fileReviewGroups = []fileReviewGroup{
	{label: "Conflicts", match: func(f git.FileStatus) bool { return f.Unmerged }},
	{label: "Staged", match: func(f git.FileStatus) bool { return !f.Unmerged && f.IsStaged() }},
	{label: "Unstaged", match: func(f git.FileStatus) bool {
		return !f.Unmerged && !f.IsStaged() && !f.Untracked
	}},
	{label: "Untracked", match: func(f git.FileStatus) bool { return !f.Unmerged && f.Untracked }},
}
```

Then add:

```go
func (m Model) filePanelRows() []panelRow {
	if len(m.files) == 0 {
		return []panelRow{{text: emptyPanelLine(PanelFiles), kind: panelRowEmpty, itemIndex: -1}}
	}

	rows := []panelRow{}
	for _, group := range fileReviewGroups {
		start := len(rows)
		rows = append(rows, panelRow{text: group.label, kind: panelRowHeader, itemIndex: -1})
		for i, f := range m.files {
			if group.match(f) {
				rows = append(rows, panelRow{text: fileLine(f), kind: panelRowItem, itemIndex: i})
			}
		}
		if len(rows) == start+1 {
			rows = rows[:start]
		}
	}
	return rows
}

func (m Model) panelRows(p Panel) []panelRow {
	switch p {
	case PanelFiles:
		return m.filePanelRows()
	case PanelBranches:
		if len(m.branches) == 0 {
			return []panelRow{{text: emptyPanelLine(p), kind: panelRowEmpty, itemIndex: -1}}
		}
		rows := make([]panelRow, len(m.branches))
		for i, b := range m.branches {
			rows[i] = panelRow{text: branchLine(b), kind: panelRowItem, itemIndex: i}
		}
		return rows
	case PanelCommits:
		if len(m.commits) == 0 {
			return []panelRow{{text: emptyPanelLine(p), kind: panelRowEmpty, itemIndex: -1}}
		}
		rows := make([]panelRow, len(m.commits))
		for i, c := range m.commits {
			rows[i] = panelRow{text: commitLine(c), kind: panelRowItem, itemIndex: i}
		}
		return rows
	default:
		return nil
	}
}

func panelRowTexts(rows []panelRow) []string {
	lines := make([]string, len(rows))
	for i, row := range rows {
		lines[i] = row.text
	}
	return lines
}
```

Update `panelLines` to delegate through `panelRows`:

```go
func (m Model) panelLines(p Panel) []string {
	return panelRowTexts(m.panelRows(p))
}
```

- [ ] **Step 2: Run focused grouping tests**

Run:

```bash
go test ./internal/ui -run "TestFilePanelRows|TestPanelLines" -count=1
```

Expected: PASS for the new grouping tests. Existing `TestPanelLinesRenderRows` should still pass because grouped output still contains the file row text.

- [ ] **Step 3: Run panel tests**

Run:

```bash
go test ./internal/ui -run "Test.*Panel|TestMarkerColor" -count=1
```

Expected: Some styling/rendering tests may fail because rendering still treats header rows like selectable item rows. Fix that in Task 3, not here.

- [ ] **Step 4: Commit helper implementation**

```bash
git add internal/ui/panels.go internal/ui/panels_test.go
git commit -m "feat(ui): group files by review state"
```

---

### Task 3: Render Group Headers Without Breaking Selection

**Files:**
- Modify: `internal/ui/panels.go`
- Modify: `internal/ui/panels_test.go`
- Modify: `internal/ui/view.go`

- [ ] **Step 1: Write failing tests for selected file styling after headers**

Add these tests to `internal/ui/panels_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/ui -run "TestStyledPanelRows" -count=1
```

Expected: FAIL because `styledPanelRows` does not exist yet.

- [ ] **Step 3: Replace line-only styling with row-aware styling**

In `internal/ui/panels.go`, replace `styledPanelLines` with this row-aware function and keep a compatibility wrapper for branch and commit tests that still pass plain lines:

```go
func (m Model) styledPanelLines(p Panel, lines []string, width int) []string {
	if p == PanelFiles {
		return m.styledPanelRows(p, m.panelRows(p), width)
	}
	rows := make([]panelRow, len(lines))
	empty := emptyPanelLine(p)
	for i, line := range lines {
		kind := panelRowItem
		itemIndex := i
		if len(lines) == 1 && line == empty {
			kind = panelRowEmpty
			itemIndex = -1
		}
		rows[i] = panelRow{text: line, kind: kind, itemIndex: itemIndex}
	}
	return m.styledPanelRows(p, rows, width)
}

func (m Model) styledPanelRows(p Panel, rows []panelRow, width int) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.kind == panelRowEmpty {
			out = append(out, caretGutter+mutedStyle.Render(row.text))
			continue
		}
		if row.kind == panelRowHeader {
			out = append(out, caretGutter+mutedStyle.Render(row.text))
			continue
		}

		selected := m.focus == p && row.itemIndex == m.cursor[p]
		if selected {
			bar := normalizePanelRowText(caretGutter + row.text)
			if p == PanelCommits {
				out = append(out, cursorStyle.Width(width).Render(bar))
				continue
			}
			bar = oneLinePanelRow(bar, width)
			if pad := width - lipgloss.Width(bar); pad > 0 {
				bar += strings.Repeat(" ", pad)
			}
			out = append(out, cursorStyle.Inline(true).MaxWidth(width).Render(bar))
			continue
		}

		line := row.text
		if p == PanelFiles && row.itemIndex >= 0 && row.itemIndex < len(m.files) && len(line) >= markerWidth {
			marker := lipgloss.NewStyle().Foreground(markerColor(m.files[row.itemIndex])).Render(line[:markerWidth])
			line = marker + line[markerWidth:]
		}
		if p == PanelCommits {
			out = append(out, lipgloss.NewStyle().Width(width).Render(normalizePanelRowText(caretGutter+line)))
			continue
		}
		out = append(out, oneLinePanelRow(caretGutter+line, width))
	}
	return out
}
```

Update `renderPanel` to accept `[]panelRow`:

```go
func (m Model) renderPanel(title string, p Panel, rows []panelRow, w, h int) string {
	style := borderBlur
	if m.listPaneFocused(p) {
		style = borderFocused
	}
	style = style.Padding(0, 1)

	contentW := w - style.GetHorizontalFrameSize()
	if contentW < 0 {
		contentW = 0
	}
	contentH := h - style.GetVerticalFrameSize()
	if contentH < 0 {
		contentH = 0
	}

	listRows := contentH - 2
	if listRows < 0 {
		listRows = 0
	}

	styled := m.styledPanelRows(p, rows, contentW-style.GetHorizontalPadding())
	visible, hiddenBelow := windowLines(styled, m.scroll[p], listRows)
	if hiddenBelow > 0 && listRows > 0 {
		visible, hiddenBelow = windowLines(styled, m.scroll[p], listRows-1)
		visible = append(visible, mutedStyle.Render(fmt.Sprintf("... +%d more", hiddenBelow)))
	}

	shownTitle := title
	if m.listPaneFocused(p) {
		shownTitle = titleStyle.Render(title)
	}
	content := shownTitle + "\n\n" + strings.Join(visible, "\n")
	return style.Width(contentW).Height(contentH).MaxHeight(h).Render(content)
}
```

Update the call site in `internal/ui/view.go`:

```go
list := m.renderPanel(panelTitle(panelName(m.focus), m.focusLen()), m.focus, m.panelRows(m.focus), listOuter, bodyH)
```

Update existing tests that call `renderPanel` directly so they pass rows:

```go
got := m.renderPanel("Files 1", PanelFiles, []panelRow{{text: "M  internal/ui/view.go", kind: panelRowItem, itemIndex: 0}}, 40, 13)
```

For overflow tests, use:

```go
rows := m.panelRows(PanelFiles)
got := m.renderPanel("Files 20", PanelFiles, rows, 30, 8)
```

- [ ] **Step 4: Run focused render tests**

Run:

```bash
go test ./internal/ui -run "TestStyledPanel|TestRenderPanel|TestViewFits" -count=1
```

Expected: PASS. If any test still expects the Unicode overflow marker, update the expectation to look for `"more"` only.

- [ ] **Step 5: Commit row-aware rendering**

```bash
git add internal/ui/panels.go internal/ui/panels_test.go internal/ui/view.go internal/ui/view_test.go
git commit -m "feat(ui): render grouped review rows"
```

---

### Task 4: Keep Grouped File Selection Visible While Scrolling

**Files:**
- Modify: `internal/ui/update.go`
- Modify: `internal/ui/update_test.go`
- Modify: `internal/ui/panels.go`

- [ ] **Step 1: Write failing tests for visual row scroll mapping**

Add these tests to `internal/ui/update_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/ui -run "TestSelectedPanelRowForGroupedFiles|TestMoveCursorScrollsGroupedFilesByVisibleRows" -count=1
```

Expected: FAIL because `selectedPanelRow` does not exist and `moveCursor` still scrolls by real file index.

- [ ] **Step 3: Add visual row helpers**

In `internal/ui/panels.go`, add:

```go
func (m Model) selectedPanelRow(p Panel) int {
	rows := m.panelRows(p)
	selected := m.cursor[p]
	for i, row := range rows {
		if row.kind == panelRowItem && row.itemIndex == selected {
			return i
		}
	}
	if len(rows) == 0 {
		return 0
	}
	return len(rows) - 1
}
```

In `internal/ui/update.go`, replace the scroll portion of `moveCursor` with visual-row math:

```go
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
```

- [ ] **Step 4: Run focused update tests**

Run:

```bash
go test ./internal/ui -run "TestSelectedPanelRowForGroupedFiles|TestMoveCursorScrollsGroupedFilesByVisibleRows|TestMoveCursorScrollsToKeepSelectionVisible|TestUpdate_JMovesCursorDown" -count=1
```

Expected: PASS.

- [ ] **Step 5: Run all UI tests**

Run:

```bash
go test ./internal/ui -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit scroll behavior**

```bash
git add internal/ui/panels.go internal/ui/update.go internal/ui/update_test.go
git commit -m "fix(ui): keep grouped review selection visible"
```

---

### Task 5: Add Selected-File State And Action Copy

**Files:**
- Modify: `internal/ui/view.go`
- Modify: `internal/ui/view_test.go`

- [ ] **Step 1: Write failing footer and selected-context tests**

Update `TestFooterActionsByFocusAndMode` in `internal/ui/view_test.go` by replacing the `"files focus"` case with these cases:

```go
{
	name: "files focus unstaged with nothing staged",
	setup: func(m *Model) {
		m.focus = PanelFiles
		m.files = []git.FileStatus{{Path: "a.go", Worktree: 'M'}}
	},
	want: "Files: space stage · d discard · c commit all · ? help · q quit",
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
	want: "Files: space stage · d discard · c commit staged · ? help · q quit",
},
{
	name: "files focus staged",
	setup: func(m *Model) {
		m.focus = PanelFiles
		m.files = []git.FileStatus{{Path: "a.go", Staged: 'M'}}
	},
	want: "Files: space unstage · c commit staged · ? help · q quit",
},
{
	name: "files focus untracked",
	setup: func(m *Model) {
		m.focus = PanelFiles
		m.files = []git.FileStatus{{Path: "a.go", Untracked: true}}
	},
	want: "Files: space stage · d discard · ? help · q quit",
},
```

Add conflict-specific selected-context coverage:

```go
func TestSelectedContextLinesShowConcreteFileActions(t *testing.T) {
	tests := []struct {
		name  string
		files []git.FileStatus
		want  []string
	}{
		{
			name:  "unstaged",
			files: []git.FileStatus{{Path: "a.go", Worktree: 'M'}},
			want:  []string{"a.go", "unstaged file", "actions: space stage, d discard, c commit all"},
		},
		{
			name:  "staged",
			files: []git.FileStatus{{Path: "a.go", Staged: 'M'}},
			want:  []string{"a.go", "staged file", "actions: space unstage, c commit staged"},
		},
		{
			name:  "untracked",
			files: []git.FileStatus{{Path: "a.go", Untracked: true}},
			want:  []string{"a.go", "untracked file", "actions: space stage, d discard"},
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/ui -run "TestFooterActionsByFocusAndMode|TestSelectedContextLinesShowConcreteFileActions" -count=1
```

Expected: FAIL because Files footer and selected context still use generic action copy.

- [ ] **Step 3: Add helper methods for selected file copy**

In `internal/ui/view.go`, add these helpers near `selectedContextLines`:

```go
func (m Model) selectedFile() (git.FileStatus, bool) {
	i := m.cursor[PanelFiles]
	if i < 0 || i >= len(m.files) {
		return git.FileStatus{}, false
	}
	return m.files[i], true
}

func fileReviewState(f git.FileStatus) string {
	switch {
	case f.Unmerged:
		return "conflict: " + conflictLabel(f.Conflict)
	case f.Untracked:
		return "untracked file"
	case f.IsStaged():
		return "staged file"
	default:
		return "unstaged file"
	}
}

func (m Model) commitHint() string {
	if m.anyStaged() {
		return "commit staged"
	}
	return "commit all"
}

func (m Model) selectedFileActionLine(f git.FileStatus) string {
	switch {
	case f.Unmerged:
		return "actions: e edit, space resolve, A abort, c commit"
	case f.Untracked:
		return "actions: space stage, d discard"
	case f.IsStaged():
		return "actions: space unstage, c commit staged"
	default:
		return "actions: space stage, d discard, c " + m.commitHint()
	}
}

func (m Model) selectedFileFooterHints(f git.FileStatus) []keyHint {
	switch {
	case f.Unmerged:
		return []keyHint{{"e", "edit"}, {"space", "resolve"}, {"A", "abort"}, {"c", "commit"}, {"?", "help"}, {"q", "quit"}}
	case f.Untracked:
		return []keyHint{{"space", "stage"}, {"d", "discard"}, {"?", "help"}, {"q", "quit"}}
	case f.IsStaged():
		return []keyHint{{"space", "unstage"}, {"c", "commit staged"}, {"?", "help"}, {"q", "quit"}}
	default:
		return []keyHint{{"space", "stage"}, {"d", "discard"}, {"c", m.commitHint()}, {"?", "help"}, {"q", "quit"}}
	}
}
```

Add `internal/git` to the import list in `view.go`:

```go
import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kael02/loom/internal/git"
)
```

- [ ] **Step 4: Wire helpers into selected context and footer**

Replace the `PanelFiles` branch in `selectedContextLines` with:

```go
case PanelFiles:
	f, ok := m.selectedFile()
	if !ok {
		return []string{"No file selected", "working tree clean"}
	}
	return []string{f.Path, fileReviewState(f), m.selectedFileActionLine(f)}
```

Replace the `PanelFiles` branch in `footerHints` with:

```go
case PanelFiles:
	if f, ok := m.selectedFile(); ok {
		return "Files", m.selectedFileFooterHints(f)
	}
	return "Files", []keyHint{{"?", "help"}, {"q", "quit"}}
```

This removes the broad `if m.merging` footer branch. Conflict hints now appear only when the selected file is an actual conflict.

- [ ] **Step 5: Update existing conflict footer test**

Replace `TestFooterConflictHints` with:

```go
func TestFooterConflictHints(t *testing.T) {
	m := newTestModel()
	m.merging = true
	m.focus = PanelFiles
	m.files = []git.FileStatus{{Path: "a.go", Unmerged: true, Conflict: "UU"}}
	if got := m.footerActions(); got != "Files: e edit · space resolve · A abort · c commit · ? help · q quit" {
		t.Errorf("footer = %q", got)
	}
}
```

- [ ] **Step 6: Run focused view tests**

Run:

```bash
go test ./internal/ui -run "TestFooterActionsByFocusAndMode|TestFooterConflictHints|TestSelectedContext" -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit action-copy changes**

```bash
git add internal/ui/view.go internal/ui/view_test.go
git commit -m "feat(ui): clarify selected file actions"
```

---

### Task 6: Polish Main Pane Title, Position, And Loading Copy

**Files:**
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/update.go`
- Modify: `internal/ui/view.go`
- Modify: `internal/ui/view_test.go`
- Modify: `internal/ui/update_test.go`

- [ ] **Step 1: Write failing tests for title and loading copy**

Update the Files cases in `TestMainTitleForFocusedSelection`:

```go
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
```

Update `TestMainContentIncludesHeading` to expect:

```go
if !strings.Contains(got, "a.go | unstaged | 1 of 1") {
	t.Fatalf("mainContent missing heading: %q", got)
}
```

Add these tests to `internal/ui/view_test.go`:

```go
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
```

Add this test to `internal/ui/update_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/ui -run "TestMainTitle|TestMainContentShowsLoadingDiffCopy|TestReloadMainSetsAndClearsLoadingState" -count=1
```

Expected: FAIL because `mainLoading` and the new title format do not exist.

- [ ] **Step 3: Add loading state**

In `internal/ui/model.go`, add this field to `Model` near `viewport`:

```go
mainLoading bool // true while the main pane is waiting for the latest selection load
```

In `internal/ui/update.go`, update `diffLoadedMsg` handling:

```go
case diffLoadedMsg:
	if msg.seq == m.reqSeq {
		m.viewport.SetContent(msg.text)
		m.mainLoading = false
	}
	return m, nil
```

Update `reloadMain`:

```go
func (m Model) reloadMain() (Model, tea.Cmd) {
	m.reqSeq++
	cmd := m.loadMainForSelection()
	m.mainLoading = cmd != nil
	if cmd != nil {
		m.viewport.SetContent("")
	}
	return m, cmd
}
```

- [ ] **Step 4: Add title state helpers**

In `internal/ui/view.go`, add:

```go
func fileTitleState(f git.FileStatus) string {
	switch {
	case f.Unmerged:
		return "conflict"
	case f.Untracked:
		return "untracked"
	case f.IsStaged():
		return "staged"
	default:
		return "unstaged"
	}
}

func (m Model) selectedFilePosition() (int, int) {
	if len(m.files) == 0 {
		return 0, 0
	}
	i := m.cursor[PanelFiles]
	if i < 0 {
		i = 0
	}
	if i >= len(m.files) {
		i = len(m.files) - 1
	}
	return i + 1, len(m.files)
}
```

Replace the Files branch in `mainTitle`:

```go
case PanelFiles:
	f, ok := m.selectedFile()
	if !ok {
		return "Working tree clean"
	}
	pos, total := m.selectedFilePosition()
	return fmt.Sprintf("%s | %s | %d of %d", f.Path, fileTitleState(f), pos, total)
```

- [ ] **Step 5: Update empty body and loading copy**

Update `mainContent` body handling:

```go
body := m.viewport.View()
if m.mainLoading && m.focus == PanelFiles {
	body = "Loading diff..."
} else if strings.TrimSpace(body) == "" {
	body = m.emptyMainBody()
}
```

Update `emptyMainBody` Files copy:

```go
case PanelFiles:
	return "No diff for this file"
```

- [ ] **Step 6: Update view tests with old heading expectations**

In `TestViewRendersFocusMode`, replace:

```go
"Diff: internal/ui/view.go (unstaged)",
```

with:

```go
"internal/ui/view.go | unstaged | 1 of 1",
```

- [ ] **Step 7: Run focused tests**

Run:

```bash
go test ./internal/ui -run "TestMainTitle|TestMainContent|TestReloadMainSetsAndClearsLoadingState|TestViewRendersFocusMode" -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit main-pane polish**

```bash
git add internal/ui/model.go internal/ui/update.go internal/ui/view.go internal/ui/view_test.go internal/ui/update_test.go
git commit -m "feat(ui): polish review main pane state"
```

---

### Task 7: Final Verification And Documentation Sync

**Files:**
- Inspect: `README.md`
- Modify only if verification exposes stale copy: `README.md`
- Modify only if verification exposes stale assertions: `internal/ui/view_test.go`
- Verify: all touched files

- [ ] **Step 1: Check whether README key/copy needs a minimal sync**

Run:

```bash
Select-String -Path README.md -Pattern "space|d|c|Files|help"
```

Expected: README key table still describes the same keys. If no behavior changed, no README edit is required.

- [ ] **Step 2: Run gofmt**

Run:

```bash
gofmt -w internal/ui/panels.go internal/ui/update.go internal/ui/view.go internal/ui/model.go internal/ui/panels_test.go internal/ui/update_test.go internal/ui/view_test.go
```

Expected: command exits 0.

- [ ] **Step 3: Run focused UI tests**

Run:

```bash
go test ./internal/ui -count=1
```

Expected: PASS.

- [ ] **Step 4: Run all tests**

Run:

```bash
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 5: Inspect final diff**

Run:

```bash
git diff -- internal/ui/panels.go internal/ui/update.go internal/ui/view.go internal/ui/model.go internal/ui/panels_test.go internal/ui/update_test.go internal/ui/view_test.go README.md
```

Expected: Diff only covers review-flow polish and any required README wording sync.

- [ ] **Step 6: Commit final cleanup only if files changed**

If `gofmt`, README sync, or test expectation cleanup changed files, run:

```bash
git add internal/ui/panels.go internal/ui/update.go internal/ui/view.go internal/ui/model.go internal/ui/panels_test.go internal/ui/update_test.go internal/ui/view_test.go README.md
git commit -m "chore(ui): verify review flow polish"
```

If no files changed, do not create an empty commit.

---

## Implementation Order

1. Add failing grouping tests.
2. Add row projection helpers.
3. Make rendering row-aware.
4. Fix scroll math around grouped visual rows.
5. Clarify selected-file action copy.
6. Polish main title, review position, and loading copy.
7. Run full verification and sync docs only if behavior-facing copy changed.

## Final Verification Checklist

- `go test ./internal/ui -count=1` passes.
- `go test ./... -count=1` passes.
- Files are grouped as `Conflicts`, `Staged`, `Unstaged`, `Untracked`.
- Empty groups are omitted.
- Cursor selection remains a real `m.files` index.
- File actions target the intended selected file.
- Footer hints change for staged, unstaged, untracked, and conflicted files.
- Main title uses `path | state | N of M`.
- Loading copy says `Loading diff...` only while a current file diff load is pending.
- No `internal/git` files were changed.
