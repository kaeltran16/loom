# loom Focus Mode Wiring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **Git note (user override):** The repository owner's rules forbid committing without explicit approval and require batching into a single commit. Each task therefore ends with a **Checkpoint** (run the full suite, stage with `git add`) instead of a commit. The single commit happens once, at the end, only after the owner approves. Do not run `git commit` mid-plan.

**Goal:** Render the already-designed Focus Mode + Status Rail layout and add timestamped command feedback, a command-log overlay, and cursor-follow list scrolling — all inside `internal/ui`.

**Architecture:** loom is a Bubble Tea app where `View()` is a pure `Model → string` function. The rail/top-bar content helpers already exist and are tested but unreachable; this plan gives them a render slot, replaces the three-panel stacked body with a single focused workflow list plus preview plus rail, and adds the three behaviour features. `internal/git` is untouched.

**Tech Stack:** Go, Bubble Tea, Bubbles (`viewport`, `spinner`, `textarea`), Lipgloss.

---

## File Structure

| File | Responsibility after this plan |
|------|--------------------------------|
| `internal/ui/model.go` | State. Adds `cmdEntry` type, `cmdLog []cmdEntry`, `scroll map[Panel]int`, `listHeight int`; keeps `showLog`. |
| `internal/ui/update.go` | Reducer. Appends timestamped command entries; clamps scroll on cursor move; `x`/`?` toggles are mutually exclusive. |
| `internal/ui/view.go` | Focus Mode `View()` assembly; top bar; help + command-log overlays; footer; `formatCmdEntry`. |
| `internal/ui/panels.go` | Row formatting, styles, the pure list-windowing helper, and `renderPanel`/`renderStatusRail` taking outer widths. |
| `internal/ui/keys.go` | Unchanged (`keyLog = "x"` stays). |
| `internal/git/*` | **Untouched.** |

Task order: **1** data (timestamps) → **2** list scrolling → **3** command-log overlay → **4** Focus Mode assembly. Each task leaves the build and tests green.

---

## Task 1: Timestamped command log (`cmdEntry`)

Introduce a timestamped command-log entry and thread it through every reader/writer of `cmdLog`.

**Files:**
- Modify: `internal/ui/model.go` (field + type)
- Modify: `internal/ui/update.go` (append with time)
- Modify: `internal/ui/view.go` (`formatCmdEntry`, `recentCommandLines`, `statusRailContent`, `footerStatus`)
- Test: `internal/ui/view_test.go` (new format test + fix existing cmdLog tests)
- Test: `internal/ui/update_test.go` (fix `cmdLog[0]` assertion)

- [ ] **Step 1: Write the failing test for the formatter**

Add to `internal/ui/view_test.go` (and add `"time"` to its imports):

```go
func TestFormatCmdEntryShowsHHMM(t *testing.T) {
	e := cmdEntry{at: time.Date(2026, 6, 11, 10, 21, 0, 0, time.UTC), text: "git fetch"}
	if got := formatCmdEntry(e); got != "10:21 git fetch" {
		t.Fatalf("formatCmdEntry = %q, want %q", got, "10:21 git fetch")
	}
}
```

- [ ] **Step 2: Run it to confirm it fails to compile**

Run: `go test ./internal/ui/ -run TestFormatCmdEntryShowsHHMM -v`
Expected: FAIL — `undefined: cmdEntry` / `undefined: formatCmdEntry`.

- [ ] **Step 3: Add the `cmdEntry` type and change the field**

In `internal/ui/model.go`, add `"time"` to imports. Add the type above `Model` and change the field:

```go
// cmdEntry is one git command we ran, with when it completed.
type cmdEntry struct {
	at   time.Time
	text string
}
```

Change `cmdLog []string` to:

```go
	cmdLog   []cmdEntry
```

- [ ] **Step 4: Add `formatCmdEntry` and update the view readers**

In `internal/ui/view.go`, add the formatter near `recentCommandLines`:

```go
func formatCmdEntry(e cmdEntry) string {
	return e.at.Format("15:04") + " " + e.text
}
```

Change `recentCommandLines` to format entries (it already returns newest-first, capped):

```go
func (m Model) recentCommandLines(max int) []string {
	if max <= 0 || len(m.cmdLog) == 0 {
		return nil
	}
	lines := make([]string, 0, max)
	for i := len(m.cmdLog) - 1; i >= 0 && len(lines) < max; i-- {
		lines = append(lines, formatCmdEntry(m.cmdLog[i]))
	}
	return lines
}
```

In `statusRailContent`, change the `Last:` line:

```go
	} else if len(m.cmdLog) > 0 {
		sections = append(sections, "Last: "+formatCmdEntry(m.cmdLog[len(m.cmdLog)-1]))
	}
```

In `footerStatus`, change the last-command branch (this function is removed in Task 4, but must compile now):

```go
	if len(m.cmdLog) > 0 {
		return status + "   " + mutedStyle.Render("last: "+formatCmdEntry(m.cmdLog[len(m.cmdLog)-1]))
	}
```

- [ ] **Step 5: Run the formatter test**

Run: `go test ./internal/ui/ -run TestFormatCmdEntryShowsHHMM -v`
Expected: PASS.

- [ ] **Step 6: Update the writer in `update.go`**

In `internal/ui/update.go`, add `"time"` to imports and change the `gitDoneMsg` append:

```go
		m.cmdLog = append(m.cmdLog, cmdEntry{at: time.Now(), text: msg.cmd})
```

- [ ] **Step 7: Fix the existing tests that used `[]string` for cmdLog**

In `internal/ui/update_test.go`, change the assertion in `TestUpdate_GitDoneClearsBusyAndChainsRefresh`:

```go
	if len(got.cmdLog) != 1 || got.cmdLog[0].text != "git add a.go" {
		t.Errorf("cmdLog = %v", got.cmdLog)
	}
```

In `internal/ui/view_test.go`, fix the three cmdLog literals. In `TestFooterStatusShowsErrorBusyAndLastCommand`:

```go
	m.busy = false
	m.cmdLog = []cmdEntry{{at: time.Date(2026, 6, 11, 10, 21, 0, 0, time.UTC), text: "git fetch"}}
	if got := m.footerStatus(); !strings.Contains(got, "last: 10:21 git fetch") {
		t.Fatalf("last command footerStatus = %q", got)
	}
```

In `TestRecentCommandLinesAreNewestFirstAndCapped`, replace the body with fixed-time entries:

```go
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
```

In `TestStatusRailContentShowsWorkflowCommandRecentAndSelectedContext`, change the cmdLog setup and the `Last:` expectation:

```go
	m.cmdLog = []cmdEntry{
		{at: time.Date(2026, 6, 11, 10, 20, 0, 0, time.UTC), text: "git fetch"},
		{at: time.Date(2026, 6, 11, 10, 21, 0, 0, time.UTC), text: "git add README.md"},
	}
```

and within the `want` slice change `"Last: git add README.md"` to:

```go
		"Last: 10:21 git add README.md",
```

(The bare `"git add README.md"` and `"git fetch"` entries under `Recent` still match as substrings of the timestamped lines, so leave them.)

- [ ] **Step 8: Run the full ui suite (Checkpoint)**

Run: `go build ./... ; go test ./internal/ui/ -v`
Expected: PASS (all tests, including the edited ones).
Then stage: `git add internal/ui/model.go internal/ui/update.go internal/ui/view.go internal/ui/view_test.go internal/ui/update_test.go`
Do **not** commit.

---

## Task 2: Cursor-follow list scrolling

Add a pure windowing helper, give `renderPanel` an outer-width contract plus scrolling with an overflow hint, and track a scroll offset that keeps the cursor visible.

**Files:**
- Modify: `internal/ui/model.go` (`scroll`, `listHeight`, `NewModel` init)
- Modify: `internal/ui/update.go` (`moveCursor` clamps scroll), `model.go` `layout()` sets `listHeight`
- Modify: `internal/ui/panels.go` (`windowLines`, `styledPanelLines`, rewritten `renderPanel`, `renderStatusRail`)
- Test: `internal/ui/panels_test.go` (new file) and `internal/ui/update_test.go`

- [ ] **Step 1: Write the failing test for `windowLines`**

Create `internal/ui/panels_test.go`:

```go
package ui

import "testing"

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
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/ui/ -run TestWindowLines -v`
Expected: FAIL — `undefined: windowLines`.

- [ ] **Step 3: Implement `windowLines`**

In `internal/ui/panels.go`:

```go
// windowLines returns the slice of lines visible in a window of the given
// height starting at offset, plus how many lines remain hidden below it.
func windowLines(lines []string, offset, height int) (visible []string, hiddenBelow int) {
	if height <= 0 || len(lines) == 0 {
		return nil, 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(lines)-1 {
		offset = len(lines) - 1
	}
	end := offset + height
	if end > len(lines) {
		end = len(lines)
	}
	return lines[offset:end], len(lines) - end
}
```

- [ ] **Step 4: Run the windowing test**

Run: `go test ./internal/ui/ -run TestWindowLines -v`
Expected: PASS.

- [ ] **Step 5: Write the failing test for the overflow hint**

Add to `internal/ui/panels_test.go` (add imports `"strings"` and `"github.com/kael02/loom/internal/git"`):

```go
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
```

- [ ] **Step 6: Run it to confirm it fails**

Run: `go test ./internal/ui/ -run TestRenderPanelShowsOverflowHint -v`
Expected: FAIL — no `more` hint (current `renderPanel` clips silently).

- [ ] **Step 7: Add scroll state to the model**

In `internal/ui/model.go`, add two fields to `Model`:

```go
	scroll     map[Panel]int
	listHeight int
```

In `NewModel`, initialise the map alongside `cursor`:

```go
		cursor:   map[Panel]int{},
		scroll:   map[Panel]int{},
```

In `layout()`, compute the focused list's content height (used by `moveCursor`):

```go
	listH := bodyH - borderBlur.GetVerticalFrameSize() - 2 // border + title + blank
	if listH < 0 {
		listH = 0
	}
	m.listHeight = listH
```

(Place this after `bodyH` is computed in `layout()`.)

- [ ] **Step 8: Clamp scroll when the cursor moves**

In `internal/ui/update.go`, extend `moveCursor` so the offset follows the cursor:

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

	off := m.scroll[m.focus]
	if c < off {
		off = c
	}
	if m.listHeight > 0 && c >= off+m.listHeight {
		off = c - m.listHeight + 1
	}
	m.scroll[m.focus] = off
}
```

- [ ] **Step 9: Rewrite `renderPanel` to window and hint, with an outer-width contract**

In `internal/ui/panels.go`, replace `renderPanel` and extract the styling loop. `w`/`h` are now the panel's **outer** size (border + padding included):

```go
func (m Model) styledPanelLines(p Panel, lines []string) []string {
	out := make([]string, 0, len(lines))
	sel := m.cursor[p]
	empty := emptyPanelLine(p)
	for i, l := range lines {
		if len(lines) == 1 && l == empty {
			out = append(out, mutedStyle.Render(l))
			continue
		}
		if i == sel && m.focus == p {
			l = cursorStyle.Render(l)
		}
		out = append(out, l)
	}
	return out
}

func (m Model) renderPanel(title string, p Panel, lines []string, w, h int) string {
	style := borderBlur
	if m.focus == p {
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

	listRows := contentH - 2 // title + blank line
	if listRows < 0 {
		listRows = 0
	}

	styled := m.styledPanelLines(p, lines)
	visible, hiddenBelow := windowLines(styled, m.scroll[p], listRows)
	if hiddenBelow > 0 && listRows > 0 {
		visible, hiddenBelow = windowLines(styled, m.scroll[p], listRows-1)
		visible = append(visible, mutedStyle.Render(fmt.Sprintf("… +%d more", hiddenBelow)))
	}

	content := title + "\n\n" + strings.Join(visible, "\n")
	return style.Width(contentW).Height(contentH).MaxHeight(contentH).Render(content)
}
```

- [ ] **Step 10: Give `renderStatusRail` the same outer-width contract**

In `internal/ui/view.go`, replace `renderStatusRail`:

```go
func (m Model) renderStatusRail(w, h int) string {
	style := borderBlur.Padding(0, 1)
	contentW := w - style.GetHorizontalFrameSize()
	if contentW < 0 {
		contentW = 0
	}
	contentH := h - style.GetVerticalFrameSize()
	if contentH < 0 {
		contentH = 0
	}
	return style.Width(contentW).Height(contentH).MaxHeight(contentH).Render(m.statusRailContent())
}
```

- [ ] **Step 11: Write the failing test for scroll-follow**

Add to `internal/ui/update_test.go`:

```go
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
```

- [ ] **Step 12: Run the new tests**

Run: `go test ./internal/ui/ -run "TestRenderPanelShowsOverflowHint|TestMoveCursorScrollsToKeepSelectionVisible" -v`
Expected: PASS.

- [ ] **Step 13: Run the full ui suite (Checkpoint)**

Run: `go build ./... ; go test ./internal/ui/ -v`
Expected: PASS. The existing `View()` still renders the stacked layout (changed in Task 4) but `renderPanel`'s new outer-width contract keeps `TestRenderPanelFitsRequestedHeight` and `TestViewFitsTerminalHeight` green.
Stage: `git add internal/ui/model.go internal/ui/update.go internal/ui/panels.go internal/ui/view.go internal/ui/panels_test.go internal/ui/update_test.go`
Do **not** commit.

---

## Task 3: Command-log overlay on `x`

Repurpose the dead `x` key as a full-screen, timestamped command-log overlay, mutually exclusive with help.

**Files:**
- Modify: `internal/ui/view.go` (extract `helpOverlay`, add `commandLogOverlay`)
- Modify: `internal/ui/update.go` (mutual exclusion of `showLog`/`showHelp`)
- Test: `internal/ui/view_test.go`, `internal/ui/update_test.go`

- [ ] **Step 1: Write the failing test for the overlay content**

Add to `internal/ui/view_test.go`:

```go
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
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/ui/ -run TestCommandLogOverlayShowsTimestampedHistoryNewestFirst -v`
Expected: FAIL — `undefined: commandLogOverlay`.

- [ ] **Step 3: Extract `helpOverlay` and add `commandLogOverlay`**

In `internal/ui/view.go`, replace the inline help block at the top of `View()` (the `if m.showHelp { ... }` body) with a call, and add the two methods. First, the `View()` head becomes:

```go
	if m.showHelp {
		return m.helpOverlay()
	}
	if m.showLog {
		return m.commandLogOverlay()
	}
```

Then add:

```go
func (m Model) helpOverlay() string {
	help := strings.Join([]string{
		"loom — keys",
		"",
		"1/2/3 or Tab   focus Files / Branches / Commits",
		"j/k or ↑/↓      move cursor",
		"space           stage / unstage file",
		"d               discard (confirm y)",
		"enter           switch branch",
		"c               commit (Ctrl-D send, Esc cancel)",
		"f / p / P       fetch / pull / push",
		"x               command log",
		"? / q           close help / quit",
	}, "\n")
	return borderFocused.Width(m.w - 2).Height(m.h - 2).Render(help)
}

func (m Model) commandLogOverlay() string {
	lines := []string{"loom — command log", ""}
	if len(m.cmdLog) == 0 {
		lines = append(lines, mutedStyle.Render("no commands run yet"))
	} else {
		for i := len(m.cmdLog) - 1; i >= 0; i-- {
			lines = append(lines, formatCmdEntry(m.cmdLog[i]))
		}
	}
	return borderFocused.Width(m.w - 2).Height(m.h - 2).Render(strings.Join(lines, "\n"))
}
```

- [ ] **Step 4: Run the overlay test**

Run: `go test ./internal/ui/ -run TestCommandLogOverlayShowsTimestampedHistoryNewestFirst -v`
Expected: PASS.

- [ ] **Step 5: Write the failing test for mutual exclusion**

Add to `internal/ui/update_test.go`:

```go
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
```

- [ ] **Step 6: Run them to confirm they fail**

Run: `go test ./internal/ui/ -run "TestUpdate_XOpensCommandLogAndClosesHelp|TestUpdate_QuestionOpensHelpAndClosesCommandLog" -v`
Expected: FAIL — `x` and `?` do not yet close the other overlay.

- [ ] **Step 7: Make the toggles mutually exclusive**

In `internal/ui/update.go`, change the two cases in `handleKey`:

```go
	case keyLog:
		m.showLog = !m.showLog
		m.showHelp = false
		return m, nil
	case keyHelp:
		m.showHelp = !m.showHelp
		m.showLog = false
		return m, nil
```

- [ ] **Step 8: Run the mutual-exclusion tests**

Run: `go test ./internal/ui/ -run "TestUpdate_XOpensCommandLogAndClosesHelp|TestUpdate_QuestionOpensHelpAndClosesCommandLog" -v`
Expected: PASS.

- [ ] **Step 9: Run the full ui suite (Checkpoint)**

Run: `go build ./... ; go test ./internal/ui/ -v`
Expected: PASS.
Stage: `git add internal/ui/view.go internal/ui/update.go internal/ui/view_test.go internal/ui/update_test.go`
Do **not** commit.

---

## Task 4: Focus Mode `View()` assembly

Replace the stacked body with top bar + focused list + preview + rail, auto-hide the rail below width 110, and shrink the footer to key hints (deleting the now-dead `footerStatus`).

**Files:**
- Modify: `internal/ui/view.go` (`View`, `footer`, delete `footerStatus`)
- Modify: `internal/ui/model.go` (`layout()` subtracts the top-bar row)
- Test: `internal/ui/view_test.go` (replace `TestViewIncludesPolishedStructure` and `TestFooterStatusShowsErrorBusyAndLastCommand`; add width/narrow tests)

- [ ] **Step 1: Write the failing Focus Mode View test**

In `internal/ui/view_test.go`, replace `TestViewIncludesPolishedStructure` with:

```go
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
		"[1 Files 1]",                          // focused workflow tab in the top bar
		"main ↑2 ↓0",                           // branch summary in the top bar
		"Status Rail",                          // rail visible at width 120
		"Diff: internal/ui/view.go (unstaged)", // preview heading
		"Files: space stage",                   // footer key hints
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
```

- [ ] **Step 2: Write the failing width-fit and narrow-fallback tests**

Add to `internal/ui/view_test.go`:

```go
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
```

- [ ] **Step 3: Write the failing footer test (replacing the footerStatus test)**

In `internal/ui/view_test.go`, replace `TestFooterStatusShowsErrorBusyAndLastCommand` with a test of the new `footer`:

```go
func TestFooterByState(t *testing.T) {
	// Wide + idle: key hints only.
	idle := newTestModel()
	idle.focus = PanelFiles
	if got := idle.footer(true); got != idle.footerActions() {
		t.Fatalf("idle footer = %q, want plain actions", got)
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
```

- [ ] **Step 4: Run the new tests to confirm they fail**

Run: `go test ./internal/ui/ -run "TestViewRendersFocusMode|TestViewFitsTerminalWidth|TestViewNarrowHidesRailWithinHeight|TestFooterByState" -v`
Expected: FAIL — old `View()` still stacks panels; `footer` takes no argument yet.

- [ ] **Step 5: Subtract the top-bar row in `layout()`**

In `internal/ui/model.go` `layout()`, change the body-height line to account for the top bar plus footer:

```go
	bodyH := m.h - topBarHeight - 1 // top bar row + footer row
	if bodyH < 0 {
		bodyH = 0
	}
```

(The `listHeight` computation added in Task 2 stays directly below this and now uses the corrected `bodyH`.)

- [ ] **Step 6: Rewrite `View()` for Focus Mode**

In `internal/ui/view.go`, replace the body of `View()` after the overlay guards with:

```go
	railVisible := m.w >= minRailWindowWide

	bodyH := m.h - topBarHeight - 1
	if bodyH < 0 {
		bodyH = 0
	}

	railOuter := 0
	if railVisible {
		railOuter = statusRailWidth
	}
	listOuter := m.w / 4
	if listOuter < 26 {
		listOuter = 26
	}
	mainOuter := m.w - listOuter - railOuter
	if mainOuter < 0 {
		mainOuter = 0
	}

	list := m.renderPanel(panelTitle(panelName(m.focus), m.focusLen()), m.focus, m.panelLines(m.focus), listOuter, bodyH)

	mainStyle := borderBlur
	mainInnerW := mainOuter - mainStyle.GetHorizontalFrameSize()
	if mainInnerW < 0 {
		mainInnerW = 0
	}
	mainInnerH := bodyH - mainStyle.GetVerticalFrameSize()
	if mainInnerH < 0 {
		mainInnerH = 0
	}
	vm := m
	if vm.mode != ModeCommitting {
		vm.viewport.Width = mainInnerW
		vm.viewport.Height = mainInnerH - mainHeaderHeight
		if vm.viewport.Height < 0 {
			vm.viewport.Height = 0
		}
	}
	main := mainStyle.Width(mainInnerW).Height(mainInnerH).MaxHeight(mainInnerH).Render(vm.mainContent())

	cols := []string{list, main}
	if railVisible {
		cols = append(cols, m.renderStatusRail(railOuter, bodyH))
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, cols...)

	return m.topBar() + "\n" + body + "\n" + m.footer(railVisible)
```

Remove the now-unused `sideW`, `panelHeights`, `splitPanelHeights` usage from `View()`. Delete the `splitPanelHeights` function if nothing else references it (grep first: `go vet ./internal/ui/` will flag an unused function only if unexported and unused — confirm with `grep`).

- [ ] **Step 7: Replace `footer` and delete `footerStatus`**

In `internal/ui/view.go`, replace `footer` and delete `footerStatus` entirely:

```go
func (m Model) footer(railVisible bool) string {
	actions := m.footerActions()
	switch {
	case m.busy:
		return m.spinner.View() + " working…   " + actions
	case !railVisible && m.err != nil:
		return delStyle.Render("error: "+m.err.Error()) + "   " + actions
	default:
		return actions
	}
}
```

Delete the entire `func (m Model) footerStatus() string { ... }`. (Its responsibilities now live in `topBar` and `statusRailContent`.)

- [ ] **Step 8: Run the Focus Mode tests**

Run: `go test ./internal/ui/ -run "TestViewRendersFocusMode|TestViewFitsTerminalWidth|TestViewNarrowHidesRailWithinHeight|TestFooterByState" -v`
Expected: PASS. If `TestViewFitsTerminalWidth` fails by a few columns, the cause is lipgloss's padding/width convention — the column outers already sum to `m.w` and each `renderX` subtracts its own frame, so a failure means a frame subtraction is too small; widen the subtraction until the rendered width is `<= m.w` (slight column slack is acceptable).

- [ ] **Step 9: Run the full ui suite + vet (Checkpoint)**

Run: `go build ./... ; go vet ./internal/ui/ ; go test ./internal/ui/ -v`
Expected: PASS, no vet warnings (no leftover dead `footerStatus`/`splitPanelHeights`).
Stage: `git add internal/ui/view.go internal/ui/model.go internal/ui/view_test.go`
Do **not** commit.

---

## Final: build, run, and request commit approval

- [ ] **Step 1: Full build and test**

Run: `go build -o loom.exe . ; go test ./... -v`
Expected: PASS across all packages.

- [ ] **Step 2: Manual smoke (optional but recommended)**

Run `./loom.exe` inside this repo. Confirm: top bar shows branch + tabs + state; only the focused list shows; `1`/`2`/`3`/`Tab` switch workflows; `j`/`k` scroll a long Files list with the cursor staying visible and `… +N more` appearing; `x` opens the command log; `?` opens help and closes the log; resize narrow (<110 cols) hides the rail without breaking height.

- [ ] **Step 3: Self-review the diff**

Run: `git diff --staged`
Check: no debug prints, no commented-out code, no leftover `footerStatus`/`splitPanelHeights`, every new test asserts behaviour (not internals).

- [ ] **Step 4: Request commit approval (owner rule)**

Present the staged files and a single conventional-commit message (suggested: `feat(ui): wire focus mode layout with rail, timestamps, and scrolling`) and the design/plan docs, then ask the owner to approve the single commit. Do **not** commit before approval.

---

## Self-Review (plan vs spec)

**Spec coverage:**
- Focus Mode layout (top bar / focused list / preview / rail) → Task 4. ✓
- Auto-hide rail at width 110 → Task 4 (`railVisible`, `TestViewNarrowHidesRailWithinHeight`). ✓
- Timestamped recent/last/overlay command feedback → Task 1 + Task 3. ✓
- `x` repurposed as command-log overlay, mutually exclusive with help → Task 3. ✓
- Cursor-follow list scrolling with overflow hint → Task 2. ✓
- Footer = key hints; feedback moved out of footer; error fallback on narrow → Task 4 (`footer`, delete `footerStatus`). ✓
- `internal/git` untouched → no task modifies it. ✓
- Testing: windowing, timestamp formatting, responsive rail, overlay, height/width fit → covered across tasks. ✓

**Placeholder scan:** No TBD/TODO; every code step shows complete code. ✓

**Type consistency:** `cmdEntry{at, text}` introduced in Task 1 and used identically in Task 3 (`formatCmdEntry`, `commandLogOverlay`) and the tests. `renderPanel(title, p, lines, w, h)` outer-width contract defined in Task 2 and called consistently in Task 4. `footer(railVisible bool)` defined and called in Task 4. `scroll`/`listHeight` defined in Task 2, used in `moveCursor` (Task 2) and `layout` (Task 2/4). ✓
