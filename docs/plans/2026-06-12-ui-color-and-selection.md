# UI Color & Selection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the selected list row unmistakable (full-width, high-contrast bar) and add a single cyan accent across the chrome to establish hierarchy.

**Architecture:** Styling only, confined to `internal/ui/panels.go` and `internal/ui/view.go`. No new model state, no behavior changes. The governing rule: **text-producing functions stay plain; color is applied only at the render boundary**, and every test-asserted substring stays inside one styled run so the suite passes with or without a TTY.

**Tech Stack:** Go, Bubble Tea, Lipgloss. Spec: `docs/specs/2026-06-12-ui-color-and-selection-design.md`.

---

## Project-Specific Conventions

- **Run all UI tests:** `go test ./internal/ui/...`
- **Run one test:** `go test ./internal/ui/ -run TestName -v`
- **Build:** `go build ./...`
- **Commits:** Per the repo owner's git workflow, do **not** commit per task. End each task at the test checkpoint. A single approval-gated commit is the final task.
- lipgloss renders without ANSI codes in a non-TTY (the test environment), but `lipgloss.Width` counts visible cells either way — the full-width test relies on `Width`, not on inspecting codes.

## File Map

| File | Responsibility | Change |
|---|---|---|
| `internal/ui/panels.go` | Palette + list/panel rendering | Add accent palette; brighten+bold selection; full-width selection bar; focused title color |
| `internal/ui/view.go` | Top bar, status rail, footer | Color tabs/branch/state; rail headers; (optional) footer keys |
| `internal/ui/panels_test.go` | Panel render tests | Update `styledPanelLines` calls for new arg; add full-width test |
| `internal/ui/view_test.go` | View/footer tests | Only if footer (Task 5) is done: relax two footer substring asserts |

---

## Task 1: Accent palette foundation

Adds the single accent color and the styles built from it, brightens the selection, and points the focus border at the accent. No behavior change — verified by build + existing tests staying green.

**Files:**
- Modify: `internal/ui/panels.go` (the `var (...)` style block, ~lines 11–27)

- [ ] **Step 1: Add the accent constant above the style block**

Insert immediately before the `var (` block (after the imports):

```go
// accentColor is the single accent used for focus, active tab, the caret,
// section headers, and diff hunks.
const accentColor = lipgloss.Color("14")
```

- [ ] **Step 2: Update the style block to use the accent and a bolder selection**

Replace the existing `var (...)` block (lines ~11–27) with:

```go
var (
	borderFocused = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accentColor)
	borderBlur = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238"))
	cursorStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("31")).
			Foreground(lipgloss.Color("15")).
			Bold(true)
	caretStyle  = lipgloss.NewStyle().Foreground(accentColor)
	mutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	addStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	delStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	hunkStyle   = lipgloss.NewStyle().Foreground(accentColor)
	accentStyle = lipgloss.NewStyle().Foreground(accentColor)
	titleStyle  = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	strongStyle = lipgloss.NewStyle().Bold(true)
)
```

Changes from current: `borderFocused` `7 → accentColor`; `cursorStyle` background `24 → 31` plus `Bold(true)`; `caretStyle`/`hunkStyle` now reference `accentColor` (same value `14`, now DRY); three new styles `accentStyle`, `titleStyle`, `strongStyle`.

- [ ] **Step 3: Build and run the full suite (no behavior should change)**

Run: `go build ./... && go test ./internal/ui/...`
Expected: build succeeds; all tests PASS (these are style-only changes; no asserted text or layout changed).

---

## Task 2: Full-width selection bar (TDD)

The core fix. Thread the panel content width into `styledPanelLines` and render the selected row as a bar that fills the row.

**Files:**
- Modify: `internal/ui/panels.go` — `styledPanelLines` (~lines 147–169), `renderPanel` (~lines 171–201)
- Test: `internal/ui/panels_test.go` — update two existing tests, add one

- [ ] **Step 1: Update existing tests and add the failing full-width test**

In `internal/ui/panels_test.go`, both existing calls to `styledPanelLines` take a new width argument. Update them:

In `TestStyledPanelLinesMarksSelectedRow`, change:
```go
	styled := m.styledPanelLines(PanelFiles, m.panelLines(PanelFiles))
```
to:
```go
	styled := m.styledPanelLines(PanelFiles, m.panelLines(PanelFiles), 30)
```

In `TestStyledPanelLinesCaretOnlyOnFocusedPanel`, change:
```go
	styled := m.styledPanelLines(PanelFiles, m.panelLines(PanelFiles))
```
to:
```go
	styled := m.styledPanelLines(PanelFiles, m.panelLines(PanelFiles), 30)
```

Add this new test at the end of the file:
```go
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
```

- [ ] **Step 2: Change the signature so it compiles, but DON'T fill the bar yet**

In `internal/ui/panels.go`, change `styledPanelLines` to accept `width` but keep rendering the selection at text width for now (this makes the new test fail for the right reason):

```go
func (m Model) styledPanelLines(p Panel, lines []string, width int) []string {
	out := make([]string, 0, len(lines))
	sel := m.cursor[p]
	empty := emptyPanelLine(p)
	for i, l := range lines {
		if len(lines) == 1 && l == empty {
			out = append(out, caretGutter+mutedStyle.Render(l))
			continue
		}
		if i == sel && m.focus == p {
			out = append(out, caretStyle.Render(caretSelected)+cursorStyle.Render(l))
			continue
		}
		if p == PanelFiles && i < len(m.files) && len(l) >= markerWidth {
			marker := lipgloss.NewStyle().Foreground(markerColor(m.files[i])).Render(l[:markerWidth])
			l = marker + l[markerWidth:]
		}
		out = append(out, caretGutter+l)
	}
	return out
}
```

Update the caller in `renderPanel` to pass the content width. Change:
```go
	styled := m.styledPanelLines(p, lines)
```
to:
```go
	styled := m.styledPanelLines(p, lines, contentW)
```

- [ ] **Step 3: Run the new test to verify it fails**

Run: `go test ./internal/ui/ -run TestStyledPanelLinesSelectionFillsWidth -v`
Expected: FAIL — selected row width is the caret + text length (e.g. `9`), not `30`.

- [ ] **Step 4: Implement the full-width bar**

In `styledPanelLines`, replace the selected-row branch:
```go
		if i == sel && m.focus == p {
			out = append(out, caretStyle.Render(caretSelected)+cursorStyle.Render(l))
			continue
		}
```
with:
```go
		if i == sel && m.focus == p {
			barW := width - lipgloss.Width(caretSelected)
			if barW < 0 {
				barW = 0
			}
			out = append(out, caretStyle.Render(caretSelected)+cursorStyle.Width(barW).Render(l))
			continue
		}
```

- [ ] **Step 5: Color the focused panel title (spec §5.2)**

In `renderPanel` (`internal/ui/panels.go`), the content is currently assembled as:
```go
	content := title + "\n\n" + strings.Join(visible, "\n")
```
Replace with:
```go
	shownTitle := title
	if m.focus == p {
		shownTitle = titleStyle.Render(title)
	}
	content := shownTitle + "\n\n" + strings.Join(visible, "\n")
```
(The focus *border* is already handled — `renderPanel` selects `borderFocused`, which Task 1 pointed at the accent.)

- [ ] **Step 6: Run the new test, then the full suite**

Run: `go test ./internal/ui/ -run TestStyledPanelLinesSelectionFillsWidth -v`
Expected: PASS

Run: `go test ./internal/ui/...`
Expected: all PASS (the two updated tests still check caret presence/gutter, which are unchanged).

---

## Task 3: Color the top bar

Active tab, branch summary, and command state get color. Each asserted substring stays inside one styled run, so no test edits are needed.

**Files:**
- Modify: `internal/ui/view.go` — `topBar` (~111), `branchSummary` (~120), `workflowTab` (~137), `commandStateText` (~145)

- [ ] **Step 1: Add `commandState` and make `commandStateText` derive from it**

In `internal/ui/view.go`, replace `commandStateText` (lines ~145–153):
```go
func (m Model) commandStateText() string {
	if m.err != nil {
		return "Error"
	}
	if m.busy {
		return "Working..."
	}
	return "Ready"
}
```
with:
```go
// commandState returns the command-state label and the style that colors it.
// commandStateText keeps returning plain text so its exact-match test holds.
func (m Model) commandState() (string, lipgloss.Style) {
	switch {
	case m.err != nil:
		return "Error", delStyle
	case m.busy:
		return "Working...", warnStyle
	default:
		return "Ready", addStyle
	}
}

func (m Model) commandStateText() string {
	text, _ := m.commandState()
	return text
}
```

- [ ] **Step 2: Color the top bar parts**

Replace `topBar` (lines ~111–118):
```go
func (m Model) topBar() string {
	parts := []string{
		m.branchSummary(),
		m.workflowTabs(),
		m.commandStateText(),
	}
	return strings.Join(parts, " | ")
}
```
with:
```go
func (m Model) topBar() string {
	stateText, stateStyle := m.commandState()
	parts := []string{
		m.branchSummary(),
		m.workflowTabs(),
		stateStyle.Render(stateText),
	}
	return strings.Join(parts, mutedStyle.Render(" | "))
}
```

Replace `branchSummary` (lines ~120–126):
```go
func (m Model) branchSummary() string {
	branch := m.branch.Name
	if branch == "" {
		branch = "(no branch)"
	}
	return fmt.Sprintf("%s ↑%d ↓%d", branch, m.branch.Ahead, m.branch.Behind)
}
```
with (whole summary rendered as one run, keeping `"main ↑2 ↓1"` contiguous):
```go
func (m Model) branchSummary() string {
	branch := m.branch.Name
	if branch == "" {
		branch = "(no branch)"
	}
	return strongStyle.Render(fmt.Sprintf("%s ↑%d ↓%d", branch, m.branch.Ahead, m.branch.Behind))
}
```

Replace `workflowTab` (lines ~137–143):
```go
func (m Model) workflowTab(p Panel, key, name string, count int) string {
	label := fmt.Sprintf("%s %s %d", key, name, count)
	if m.focus == p {
		return "[" + label + "]"
	}
	return label
}
```
with:
```go
func (m Model) workflowTab(p Panel, key, name string, count int) string {
	label := fmt.Sprintf("%s %s %d", key, name, count)
	if m.focus == p {
		return titleStyle.Render("[" + label + "]")
	}
	return mutedStyle.Render(label)
}
```

- [ ] **Step 3: Run the suite**

Run: `go test ./internal/ui/...`
Expected: all PASS. `TestTopBarShowsBranchWorkflowCountsAndCommandState` still finds `"main ↑2 ↓1"`, `"[1 Files 2]"`, `"2 Branches 2"`, `"3 Commits 1"`, `"Ready"` (each is one styled run). `TestCommandStateText` still gets exact `"Ready"/"Working..."/"Error"`.

---

## Task 4: Color the status rail headers and state

Section headers and the state word get the accent. Header text and casing are unchanged (tests assert `Contains("Workflow")`, so no small-caps).

**Files:**
- Modify: `internal/ui/view.go` — `statusRailContent` (~238–270)

- [ ] **Step 1: Wrap the headers and the state word**

Replace `statusRailContent` (lines ~238–270):
```go
func (m Model) statusRailContent() string {
	sections := []string{
		"Status Rail",
		"",
		"Workflow",
		fmt.Sprintf("Files: %d changed", len(m.files)),
		fmt.Sprintf("Branches: %d local", len(m.branches)),
		fmt.Sprintf("Commits: %d loaded", len(m.commits)),
		"",
		"Command",
		m.commandStateText(),
	}

	if m.err != nil {
		sections = append(sections, m.err.Error())
	} else if len(m.cmdLog) > 0 {
		sections = append(sections, "Last: "+formatCmdEntry(m.cmdLog[len(m.cmdLog)-1]))
	}

	recent := m.recentCommandLines(recentCommandMax)
	if len(recent) > 0 {
		sections = append(sections, "", "Recent")
		sections = append(sections, recent...)
	}

	selected := m.selectedContextLines()
	if len(selected) > 0 {
		sections = append(sections, "", "Selected")
		sections = append(sections, selected...)
	}

	return strings.Join(sections, "\n")
}
```
with:
```go
func (m Model) statusRailContent() string {
	stateText, stateStyle := m.commandState()
	sections := []string{
		accentStyle.Render("Status Rail"),
		"",
		accentStyle.Render("Workflow"),
		fmt.Sprintf("Files: %d changed", len(m.files)),
		fmt.Sprintf("Branches: %d local", len(m.branches)),
		fmt.Sprintf("Commits: %d loaded", len(m.commits)),
		"",
		accentStyle.Render("Command"),
		stateStyle.Render(stateText),
	}

	if m.err != nil {
		sections = append(sections, m.err.Error())
	} else if len(m.cmdLog) > 0 {
		sections = append(sections, "Last: "+formatCmdEntry(m.cmdLog[len(m.cmdLog)-1]))
	}

	recent := m.recentCommandLines(recentCommandMax)
	if len(recent) > 0 {
		sections = append(sections, "", accentStyle.Render("Recent"))
		sections = append(sections, recent...)
	}

	selected := m.selectedContextLines()
	if len(selected) > 0 {
		sections = append(sections, "", accentStyle.Render("Selected"))
		sections = append(sections, selected...)
	}

	return strings.Join(sections, "\n")
}
```

- [ ] **Step 2: Run the suite**

Run: `go test ./internal/ui/...`
Expected: all PASS. `TestStatusRailContentShowsWorkflowCommandRecentAndSelectedContext` and `TestStatusRailContentShowsError` still find each header/state/error string (each is one styled run).

---

## Task 5 (OPTIONAL): Color the footer key hints

Lowest-priority polish. Refactors `footerActions` to build from structured `keyHint` pairs so the plain join reproduces the exact current strings (existing exact-match test untouched), while a parallel styled join colors the keys. This is the only task that splits a `label + key` boundary across color runs, so it carries two test relaxations. Skip this task entirely to keep the footer plain.

**Files:**
- Modify: `internal/ui/view.go` — add `keyHint`, `footerHints`, rewrite `footerActions`, add `styledFooterActions`, update `footer` (~364–394)
- Test: `internal/ui/view_test.go` — relax `TestFooterByState` idle assert and the footer substring in `TestViewRendersFocusMode`

- [ ] **Step 1: Relax the two footer substring assertions (so they're TTY-robust)**

In `internal/ui/view_test.go`, in `TestFooterByState`, replace the idle block:
```go
	// Wide + idle: key hints only.
	idle := newTestModel()
	idle.focus = PanelFiles
	if got := idle.footer(true); got != idle.footerActions() {
		t.Fatalf("idle footer = %q, want plain actions", got)
	}
```
with:
```go
	// Wide + idle: key hints render (now styled, so assert the hints, not exact text).
	idle := newTestModel()
	idle.focus = PanelFiles
	if got := idle.footer(true); !strings.Contains(got, "stage") || !strings.Contains(got, "quit") {
		t.Fatalf("idle footer = %q, want key hints", got)
	}
```

In `TestViewRendersFocusMode`, change the footer expectation in the `want` slice:
```go
		"Files: space stage",                   // footer key hints
```
to (a single styled run, robust in a TTY):
```go
		"stage",                                // footer key hint
```

- [ ] **Step 2: Run to confirm the relaxed asserts still pass against the current code**

Run: `go test ./internal/ui/ -run 'TestFooterByState|TestViewRendersFocusMode' -v`
Expected: PASS (footer is still plain at this point; `"stage"` and `"quit"` are present).

- [ ] **Step 3: Refactor the footer to structured hints with a styled renderer**

In `internal/ui/view.go`, replace `footer` and `footerActions` (lines ~364–394) with:
```go
type keyHint struct {
	key  string
	desc string
}

func (m Model) footerHints() (label string, hints []keyHint) {
	switch m.mode {
	case ModeConfirming:
		return "Confirm", []keyHint{{"y", "yes"}, {"n", "no"}, {"esc", "cancel"}}
	case ModeCommitting:
		return "Commit", []keyHint{{"ctrl+d", "submit"}, {"esc", "cancel"}}
	}
	switch m.focus {
	case PanelFiles:
		return "Files", []keyHint{{"space", "stage"}, {"d", "discard"}, {"c", "commit"}, {"?", "help"}, {"q", "quit"}}
	case PanelBranches:
		return "Branches", []keyHint{{"enter", "switch"}, {"c", "commit"}, {"f", "fetch"}, {"p", "pull"}, {"P", "push"}, {"?", "help"}, {"q", "quit"}}
	case PanelCommits:
		return "Commits", []keyHint{{"c", "commit"}, {"f", "fetch"}, {"p", "pull"}, {"P", "push"}, {"?", "help"}, {"q", "quit"}}
	default:
		return "", []keyHint{{"?", "help"}, {"q", "quit"}}
	}
}

func (m Model) footerActions() string {
	label, hints := m.footerHints()
	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = h.key + " " + h.desc
	}
	body := strings.Join(parts, " · ")
	if label == "" {
		return body
	}
	return label + ": " + body
}

func (m Model) styledFooterActions() string {
	label, hints := m.footerHints()
	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = accentStyle.Render(h.key) + " " + mutedStyle.Render(h.desc)
	}
	body := strings.Join(parts, mutedStyle.Render(" · "))
	if label == "" {
		return body
	}
	return mutedStyle.Render(label+": ") + body
}

func (m Model) footer(railVisible bool) string {
	actions := m.styledFooterActions()
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

- [ ] **Step 4: Run the suite**

Run: `go test ./internal/ui/...`
Expected: all PASS. `TestFooterActionsByFocusAndMode` is unchanged and still gets exact strings (the plain join reproduces them); the two relaxed asserts pass.

---

## Task 6: Manual visual check + single commit (approval-gated)

- [ ] **Step 1: Build and eyeball it in a real terminal**

Run: `go build ./... && ./loom` (or `go run .`)
Expected: selected row is a full-width bright bar with a cyan caret; focused panel border/title are cyan; active tab cyan, others muted; rail headers cyan; state word colored. Resize narrow (<110 cols) to confirm the rail hides and nothing misaligns.

- [ ] **Step 2: Final full test run**

Run: `go test ./...`
Expected: all PASS.

- [ ] **Step 3: Present for commit approval**

Show the diff summary (files M, one-line each) and propose a commit message, e.g.:
`feat(ui): add accent palette and full-width selection highlight`

Then ask for explicit approval before committing (per the repo owner's git workflow — never auto-commit). Batch all changes into this single commit.

---

## Self-Review

**1. Spec coverage** (against `docs/specs/2026-06-12-ui-color-and-selection-design.md`):
- §4 Palette → Task 1.
- §5.1 Selection bar → Task 2 (Steps 1–4, 6).
- §5.2 Focus cue → Task 1 (border, via `borderFocused`) + Task 2 Step 5 (focused title, via `titleStyle` in `renderPanel`).
- §5.3 Top bar → Task 3.
- §5.4 Status rail → Task 4.
- §5.5 Footer (optional) → Task 5.
- §6 Testing strategy → Tasks 2 (full-width test, signature update) and 5 (relaxed asserts); Tasks 1/3/4 covered by unchanged tests.

**2. Placeholder scan:** none — every step has concrete code and commands.

**3. Type consistency:** `commandState() (string, lipgloss.Style)` defined in Task 3 and reused in Task 4; `styledPanelLines(p, lines, width)` signature defined in Task 2 and its only caller (`renderPanel`) updated in the same task; `keyHint`/`footerHints`/`styledFooterActions` all defined together in Task 5. `titleStyle`/`accentStyle`/`strongStyle` defined in Task 1, used in Tasks 2–4.
