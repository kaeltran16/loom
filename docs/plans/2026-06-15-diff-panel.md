# Diff Panel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the flat, line-by-line `colorizeDiff` rendering with a parsed diff model rendered as an editor-style unified diff: line-number gutter, full-row add/del backgrounds, word-level highlighting, syntax colors, trailing-whitespace markers, hunk navigation, and untracked-as-all-adds.

**Architecture:** A pure parser (`git.ParseDiff`) turns `git diff` / `git show` output into a styling-free `git.Diff` model that is the single source of truth. A pure renderer (`renderDiff` in `internal/ui/diffview.go`) projects that model into width-aware, pre-styled viewport lines. Tests assert on the model and on the renderer's **plain-text projection** (`ansi.Strip`) and **cell widths** (`lipgloss.Width`) — never on raw ANSI escape codes.

**Tech Stack:** Go 1.26, Bubble Tea, Bubbles (`viewport`), Lipgloss, `charmbracelet/x/ansi` (already an indirect dep), and `github.com/alecthomas/chroma/v2` (new, added in the final phase). Spec: `docs/specs/2026-06-12-diff-panel-design.md`.

---

## Project-Specific Conventions

- **Run git-package tests:** `go test ./internal/git/...`
- **Run ui-package tests:** `go test ./internal/ui/...`
- **Run everything:** `go test ./...`
- **Build:** `go build ./...`
- **Run one test:** `go test ./internal/ui/ -run TestName -v`
- **Commits — IMPORTANT:** Per the repo owner's git workflow (`CLAUDE.md`), do **NOT** commit per task. End each task at its test checkpoint (green build + green tests). A single approval-gated commit is the final task (Task 14). Do **not** add a co-author.
- **No ANSI assertions.** Lipgloss emits no escape codes in the non-TTY test environment, but `lipgloss.Width` counts visible cells either way, and `ansi.Strip` removes any codes that are present. Width/projection tests work with or without a TTY.
- **Match existing parser style.** New parsers use `bufio.Scanner` over the raw input, mirroring `internal/git/status.go` / `log.go`.

## Decisions & Deviations from the Spec (read before starting)

These resolve under-specified or conflicting points in the spec. If you disagree, raise it before Task 1.

1. **Commit preamble suppressed (v1).** The `git show` header (`commit`/`Author`/`Date`/message) has no field in the spec's `git.Diff` model. `ParseDiff` discards everything before the first `diff --git`. The commit hash is already shown in the pane title. `LineHunk`/`LineMeta` remain in the `LineKind` enum (for tolerance/documentation) but `ParseDiff` does not emit them.
2. **Rich file header is rendered inline** at the top of each file's content (inside the viewport) rather than as a separate "sticky" element above the rule. Reason: a single sticky header cannot represent a multi-file `git show`, and rendering inline keeps `mainTitle()` (and its existing `==` tests) unchanged. The sticky context line stays as today's `mainTitle()`.
3. **ASCII `-` for the deletion count** (e.g. `+12 -3`) instead of the spec's Unicode `−` (U+2212), to avoid copy/encoding pitfalls in test literals. Cosmetic only.
4. **One viewport-size source of truth.** `layout()` and `View()` currently compute the main width differently (a pre-existing bug). Correct truncation requires the render width to equal the display width, so Task 3 extracts `mainViewportSize()` used by both.
5. **`lang` is introduced into the line renderers only in the syntax task (Task 13)**, not threaded speculatively earlier.
6. **The scroll indicator reserves one column** (via `mainViewportSize`) and is rendered for both the diff and branch-log panes.

## File Map

| File | Responsibility | Change |
|---|---|---|
| `internal/git/diff.go` | Diff model + `ParseDiff` + untracked synthesis | **New** |
| `internal/git/diff_test.go` | Parser + synthesis tests | **New** |
| `internal/ui/diffview.go` | `renderDiff` + word-diff + helpers + syntax | **New** |
| `internal/ui/diffview_test.go` | Renderer + word-diff + helper tests | **New** |
| `internal/ui/messages.go` | `diffLoadedMsg{diff}`, new `logLoadedMsg{text}` | Modify |
| `internal/ui/model.go` | `mainDiff`/`mainText`/`hunkRows` fields, `refreshViewport`, `mainViewportSize`, `layout()` | Modify |
| `internal/ui/update.go` | Wire load/log messages + resize → `refreshViewport`; `n`/`N` keys | Modify |
| `internal/ui/commands.go` | Parse via `git.ParseDiff`; untracked file read; `logLoadedMsg` | Modify |
| `internal/ui/view.go` | `mainContent` drops `colorizeDiff`; `mainViewportSize`; scroll track | Modify |
| `internal/ui/panels.go` | Remove `colorizeDiff`/`classifyDiffLine`/`diffMetaPrefixes`/`diffKind`/`hunkStyle` | Modify |
| `internal/ui/keys.go` | `keyHunkNext`/`keyHunkPrev` | Modify |
| `internal/ui/panels_test.go` | Remove `TestClassifyDiffLine` | Modify |
| `go.mod` / `go.sum` | Add `chroma/v2` | Modify (Task 13) |

---

# Phase 1 — Model, parser, and wiring (parity + correct truncation)

## Task 1: Diff model and `ParseDiff`

Build the styling-free model and a tolerant parser shared by `git diff` and `git show`.

**Files:**
- Create: `internal/git/diff.go`
- Create: `internal/git/diff_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/git/diff_test.go`. The leading single space on context lines is significant — preserve it exactly.

```go
package git

import "testing"

// Context lines begin with one space; add/del with +/-. Preserve exact whitespace.
const diffSample = `diff --git a/internal/ui/view.go b/internal/ui/view.go
index 1111111..2222222 100644
--- a/internal/ui/view.go
+++ b/internal/ui/view.go
@@ -10,4 +10,4 @@ func (m Model) View() string {
 context one
-old line
+new line
 context two
`

const diffNewFile = `diff --git a/notes.txt b/notes.txt
new file mode 100644
index 0000000..3333333
--- /dev/null
+++ b/notes.txt
@@ -0,0 +1,2 @@
+first
+second
`

const diffRename = `diff --git a/old/name.go b/new/name.go
similarity index 100%
rename from old/name.go
rename to new/name.go
`

const showSample = `commit abc123def4567890
Author: Kael <k@example.com>
Date:   Wed Jun 11 10:00:00 2026

    fix two files

diff --git a/a.go b/a.go
index aaa1111..bbb2222 100644
--- a/a.go
+++ b/a.go
@@ -1,2 +1,2 @@ package main
-old a
+new a
 keep a
diff --git a/b.md b/b.md
index ccc3333..ddd4444 100644
--- a/b.md
+++ b/b.md
@@ -5,1 +5,2 @@ title
 keep b
+added b
`

func TestParseDiffModification(t *testing.T) {
	d := ParseDiff(diffSample)
	if len(d.Files) != 1 {
		t.Fatalf("want 1 file, got %d", len(d.Files))
	}
	f := d.Files[0]
	if f.Path != "internal/ui/view.go" || f.Lang != "go" {
		t.Fatalf("path/lang = %q/%q", f.Path, f.Lang)
	}
	if f.Adds != 1 || f.Dels != 1 {
		t.Fatalf("adds/dels = %d/%d, want 1/1", f.Adds, f.Dels)
	}
	if len(f.Hunks) != 1 || f.Hunks[0].Header != "func (m Model) View() string {" {
		t.Fatalf("hunk header = %q", f.Hunks[0].Header)
	}
	want := []DiffLine{
		{Kind: LineContext, OldNo: 10, NewNo: 10, Text: "context one"},
		{Kind: LineDel, OldNo: 11, NewNo: 0, Text: "old line"},
		{Kind: LineAdd, OldNo: 0, NewNo: 11, Text: "new line"},
		{Kind: LineContext, OldNo: 12, NewNo: 12, Text: "context two"},
	}
	got := f.Hunks[0].Lines
	if len(got) != len(want) {
		t.Fatalf("want %d lines, got %d: %+v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseDiffNewFile(t *testing.T) {
	d := ParseDiff(diffNewFile)
	f := d.Files[0]
	if f.Path != "notes.txt" || f.Adds != 2 || f.Dels != 0 {
		t.Fatalf("new file = %+v", f)
	}
	if f.Hunks[0].Lines[0] != (DiffLine{Kind: LineAdd, NewNo: 1, Text: "first"}) {
		t.Errorf("line[0] = %+v", f.Hunks[0].Lines[0])
	}
	if f.Hunks[0].Lines[1].NewNo != 2 {
		t.Errorf("line[1] NewNo = %d, want 2", f.Hunks[0].Lines[1].NewNo)
	}
}

func TestParseDiffRename(t *testing.T) {
	d := ParseDiff(diffRename)
	if len(d.Files) != 1 {
		t.Fatalf("want 1 file, got %d", len(d.Files))
	}
	f := d.Files[0]
	if f.Path != "new/name.go" || f.Adds != 0 || f.Dels != 0 || len(f.Hunks) != 0 {
		t.Fatalf("rename = %+v", f)
	}
}

func TestParseShowMultiFile(t *testing.T) {
	d := ParseDiff(showSample)
	if len(d.Files) != 2 {
		t.Fatalf("want 2 files, got %d", len(d.Files))
	}
	if d.Files[0].Path != "a.go" || d.Files[0].Adds != 1 || d.Files[0].Dels != 1 {
		t.Errorf("file[0] = %+v", d.Files[0])
	}
	if d.Files[0].Hunks[0].Header != "package main" {
		t.Errorf("file[0] header = %q", d.Files[0].Hunks[0].Header)
	}
	if d.Files[1].Path != "b.md" || d.Files[1].Adds != 1 || d.Files[1].Dels != 0 {
		t.Errorf("file[1] = %+v", d.Files[1])
	}
	// b.md: " keep b" is context at 5/5; "+added b" is an add at new line 6.
	addLine := d.Files[1].Hunks[0].Lines[1]
	if addLine.Kind != LineAdd || addLine.NewNo != 6 {
		t.Errorf("b.md add line = %+v, want LineAdd NewNo 6", addLine)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/git/ -run TestParseDiff -v`
Expected: **FAIL** — does not compile: `undefined: ParseDiff`, `undefined: DiffLine`, etc.

- [ ] **Step 3: Write the implementation**

Create `internal/git/diff.go`:

```go
package git

import (
	"bufio"
	"path/filepath"
	"strconv"
	"strings"
)

// LineKind classifies a rendered diff line. LineHunk and LineMeta exist for
// tolerance/documentation; ParseDiff carries hunk identity in the Hunk struct
// and suppresses commit/show preamble, so it does not emit those two kinds.
type LineKind int

const (
	LineContext LineKind = iota
	LineAdd
	LineDel
	LineHunk
	LineMeta
)

// DiffLine is one content line of a hunk, with the leading +/-/space stripped.
// OldNo/NewNo are 0 when the line does not exist on that side.
type DiffLine struct {
	Kind  LineKind
	OldNo int
	NewNo int
	Text  string
}

// Hunk is one @@ block. Header is the function context git emits after the
// closing @@ (empty when absent).
type Hunk struct {
	Header string
	Lines  []DiffLine
}

// FileDiff is one file's worth of changes.
type FileDiff struct {
	Path  string
	Lang  string // file extension without the dot, for the syntax lexer
	Adds  int
	Dels  int
	Hunks []Hunk
}

// Diff is the parsed model — the single source of truth for rendering.
type Diff struct {
	Files []FileDiff
}

// ParseDiff parses unified `git diff` or `git show` output. It is pure and
// tolerant: unknown structural lines are skipped, and any show commit preamble
// (before the first "diff --git") is suppressed in v1.
func ParseDiff(raw string) Diff {
	var d Diff
	var cur *FileDiff
	var hunk *Hunk
	var oldNo, newNo int

	flushHunk := func() {
		if cur != nil && hunk != nil {
			cur.Hunks = append(cur.Hunks, *hunk)
			hunk = nil
		}
	}
	flushFile := func() {
		flushHunk()
		if cur != nil {
			d.Files = append(d.Files, *cur)
			cur = nil
		}
	}

	sc := bufio.NewScanner(strings.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flushFile()
			path := parseDiffGitPath(line)
			cur = &FileDiff{Path: path, Lang: langFromPath(path)}
		case cur == nil:
			// commit/show preamble — suppressed in v1
			continue
		case strings.HasPrefix(line, "@@"):
			flushHunk()
			oldNo, newNo = parseHunkHeader(line)
			hunk = &Hunk{Header: hunkContext(line)}
		case hunk == nil:
			// structural lines before the first hunk: index, mode, ---, +++,
			// rename, similarity. Ignored (path/counts come from elsewhere).
			continue
		case strings.HasPrefix(line, "\\"):
			// "\ No newline at end of file" — not a content line
			continue
		case strings.HasPrefix(line, "+"):
			hunk.Lines = append(hunk.Lines, DiffLine{Kind: LineAdd, NewNo: newNo, Text: line[1:]})
			cur.Adds++
			newNo++
		case strings.HasPrefix(line, "-"):
			hunk.Lines = append(hunk.Lines, DiffLine{Kind: LineDel, OldNo: oldNo, Text: line[1:]})
			cur.Dels++
			oldNo++
		default:
			text := line
			if strings.HasPrefix(line, " ") {
				text = line[1:]
			}
			hunk.Lines = append(hunk.Lines, DiffLine{Kind: LineContext, OldNo: oldNo, NewNo: newNo, Text: text})
			oldNo++
			newNo++
		}
	}
	flushFile()
	return d
}

// parseDiffGitPath returns the new-side path from "diff --git a/<old> b/<new>".
// Assumes paths without spaces (v1).
func parseDiffGitPath(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return ""
	}
	return strings.TrimPrefix(fields[len(fields)-1], "b/")
}

// parseHunkHeader reads the old/new start lines from "@@ -a,b +c,d @@".
func parseHunkHeader(line string) (oldStart, newStart int) {
	parts := strings.Fields(line)
	if len(parts) >= 3 {
		oldStart = atoiBefore(strings.TrimPrefix(parts[1], "-"), ',')
		newStart = atoiBefore(strings.TrimPrefix(parts[2], "+"), ',')
	}
	return
}

func atoiBefore(s string, sep byte) int {
	if i := strings.IndexByte(s, sep); i >= 0 {
		s = s[:i]
	}
	n, _ := strconv.Atoi(s)
	return n
}

// hunkContext returns the function context after the closing "@@".
func hunkContext(line string) string {
	if i := strings.Index(line, "@@"); i >= 0 {
		rest := line[i+2:]
		if j := strings.Index(rest, "@@"); j >= 0 {
			return strings.TrimSpace(rest[j+2:])
		}
	}
	return ""
}

// langFromPath derives the syntax language token (extension without the dot).
func langFromPath(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return ""
	}
	return strings.ToLower(ext[1:])
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/git/ -v`
Expected: **PASS** (all `TestParseDiff*` / `TestParseShowMultiFile`, plus the existing git tests stay green).

---

## Task 2: `renderDiff` — plain projection from the model

Create the renderer with its final signature, but render plainly (prefix + content, truncated). Visual styling arrives in Phase 2.

**Files:**
- Create: `internal/ui/diffview.go`
- Create: `internal/ui/diffview_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/ui/diffview_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/kael02/loom/internal/git"
)

func sampleDiff() git.Diff {
	return git.Diff{Files: []git.FileDiff{{
		Path: "x.go", Lang: "go", Adds: 1, Dels: 1,
		Hunks: []git.Hunk{{
			Header: "func View()",
			Lines: []git.DiffLine{
				{Kind: git.LineContext, OldNo: 10, NewNo: 10, Text: "context"},
				{Kind: git.LineDel, OldNo: 11, Text: "old line"},
				{Kind: git.LineAdd, NewNo: 11, Text: "new line"},
			},
		}},
	}}}
}

func TestRenderDiffProjectsModel(t *testing.T) {
	lines, hunkRows := renderDiff(sampleDiff(), 80)
	joined := ansi.Strip(strings.Join(lines, "\n"))
	for _, want := range []string{"x.go", "+1 -1", "func View()", "old line", "new line", "context"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("renderDiff projection missing %q:\n%s", want, joined)
		}
	}
	// one hunk → one band row index, and it points at a line containing the header
	if len(hunkRows) != 1 {
		t.Fatalf("hunkRows = %v, want 1 entry", hunkRows)
	}
	if !strings.Contains(ansi.Strip(lines[hunkRows[0]]), "func View()") {
		t.Fatalf("hunkRows[0] does not point at the band: %q", lines[hunkRows[0]])
	}
}

func TestRenderDiffTruncatesLongLines(t *testing.T) {
	d := git.Diff{Files: []git.FileDiff{{
		Path: "x.go", Lang: "go",
		Hunks: []git.Hunk{{Lines: []git.DiffLine{
			{Kind: git.LineContext, OldNo: 1, NewNo: 1, Text: strings.Repeat("abcdefghij", 20)},
		}}},
	}}}
	lines, _ := renderDiff(d, 30)
	for _, l := range lines {
		if w := lipgloss.Width(l); w > 30 {
			t.Fatalf("line exceeds width: %d > 30 (%q)", w, ansi.Strip(l))
		}
	}
	if !strings.Contains(ansi.Strip(strings.Join(lines, "\n")), "…") {
		t.Fatalf("expected an ellipsis on the truncated line")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/ui/ -run TestRenderDiff -v`
Expected: **FAIL** — does not compile: `undefined: renderDiff`.

- [ ] **Step 3: Write the implementation**

Create `internal/ui/diffview.go`:

```go
package ui

import (
	"fmt"

	"github.com/charmbracelet/x/ansi"
	"github.com/kael02/loom/internal/git"
)

// renderDiff walks the parsed diff and returns the viewport lines plus the row
// index of each hunk band (used by n/N navigation). Pure: all width math and
// styling live here; the model carries no styling.
func renderDiff(d git.Diff, width int) (lines []string, hunkRows []int) {
	for _, f := range d.Files {
		lines = append(lines, renderFileHeader(f, width))
		for _, h := range f.Hunks {
			hunkRows = append(hunkRows, len(lines))
			lines = append(lines, renderHunkBand(h, width))
			for _, ln := range h.Lines {
				lines = append(lines, renderLine(ln, width))
			}
		}
	}
	return lines, hunkRows
}

func renderFileHeader(f git.FileDiff, width int) string {
	return ansi.Truncate(fmt.Sprintf("%s  +%d -%d", f.Path, f.Adds, f.Dels), width, "…")
}

func renderHunkBand(h git.Hunk, width int) string {
	s := "@@"
	if h.Header != "" {
		s = "@@ " + h.Header
	}
	return ansi.Truncate(s, width, "…")
}

func renderLine(ln git.DiffLine, width int) string {
	prefix := " "
	switch ln.Kind {
	case git.LineAdd:
		prefix = "+"
	case git.LineDel:
		prefix = "-"
	}
	return ansi.Truncate(prefix+ln.Text, width, "…")
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/ui/ -run TestRenderDiff -v`
Expected: **PASS**.

---

## Task 3: One viewport-size source of truth (`mainViewportSize`)

Extract the main-pane viewport sizing into a single helper so `layout()` (and therefore `refreshViewport`) sizes the viewport to the same width `View()` displays. This is required for correct truncation and fixes a pre-existing `layout()`/`View()` divergence.

**Files:**
- Modify: `internal/ui/model.go` (`layout()`, add `mainViewportSize`)
- Modify: `internal/ui/view.go:54-71` (use the helper)
- Modify: `internal/ui/update_test.go` (add a sizing test)

- [ ] **Step 1: Write the failing test**

Add to `internal/ui/update_test.go`:

```go
func TestLayoutSizesViewportToMainViewportSize(t *testing.T) {
	m := newTestModel()
	m.w, m.h = 120, 40
	m.layout()
	wantW, wantH := m.mainViewportSize()
	if m.viewport.Width != wantW || m.viewport.Height != wantH {
		t.Fatalf("viewport = %dx%d, want %dx%d", m.viewport.Width, m.viewport.Height, wantW, wantH)
	}
	if wantW <= 0 || wantH <= 0 {
		t.Fatalf("expected a positive viewport at 120x40, got %dx%d", wantW, wantH)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/ui/ -run TestLayoutSizesViewportToMainViewportSize -v`
Expected: **FAIL** — does not compile: `m.mainViewportSize undefined`.

- [ ] **Step 3: Add `mainViewportSize` and rewrite `layout()`**

In `internal/ui/model.go`, replace the entire `layout` function (lines 100-121) with:

```go
// mainViewportSize returns the inner width/height of the diff/main viewport for
// the current window. Single source of truth shared by layout() and View() so
// the render width always equals the displayed width.
func (m Model) mainViewportSize() (w, h int) {
	railOuter := 0
	if m.w >= minRailWindowWide {
		railOuter = statusRailWidth
	}
	listOuter := m.w / 4
	if listOuter < listMinWidth {
		listOuter = listMinWidth
	}
	mainOuter := m.w - listOuter - railOuter
	if mainOuter < 0 {
		mainOuter = 0
	}
	w = mainOuter - borderBlur.GetHorizontalFrameSize()
	if w < 0 {
		w = 0
	}
	bodyH := m.h - topBarHeight - 1
	if bodyH < 0 {
		bodyH = 0
	}
	h = bodyH - borderBlur.GetVerticalFrameSize() - mainHeaderHeight
	if h < 0 {
		h = 0
	}
	return w, h
}

// layout sizes the panes for the current window.
func (m *Model) layout() {
	bodyH := m.h - topBarHeight - 1 // top bar row + footer row
	if bodyH < 0 {
		bodyH = 0
	}
	listH := bodyH - borderBlur.GetVerticalFrameSize() - 2 // border + title + blank
	if listH < 0 {
		listH = 0
	}
	m.listHeight = listH
	m.viewport.Width, m.viewport.Height = m.mainViewportSize()
}
```

- [ ] **Step 4: Rewrite the main-pane block in `View()`**

In `internal/ui/view.go`, replace lines 54-71 (from `mainStyle := borderBlur` through the `main := ...Render(vm.mainContent())` line) with:

```go
	vpW, vpH := m.mainViewportSize()
	vm := m
	if vm.mode != ModeCommitting {
		vm.viewport.Width = vpW
		vm.viewport.Height = vpH
	}
	mainStyle := borderBlur
	mainInnerW := vpW
	mainInnerH := vpH + mainHeaderHeight
	main := mainStyle.Width(mainInnerW).Height(mainInnerH).MaxHeight(bodyH).Render(vm.mainContent())
```

The surrounding `railVisible`, `bodyH`, `listOuter`, `list`, `cols`, and `body` lines (view.go:32-52 and 73-79) are unchanged. The pane outer width is identical to before (`mainInnerW + frame == mainOuter`), so `TestViewFitsTerminalWidth`/`Height` stay green.

- [ ] **Step 5: Run the tests to verify green**

Run: `go test ./internal/ui/...`
Expected: **PASS** — the new sizing test plus every existing view test (`TestViewFitsTerminalWidth`, `TestViewFitsTerminalHeight`, `TestViewRendersFocusMode`, `TestViewNarrowHidesRailWithinHeight`) stay green.

Run: `go build ./...`
Expected: builds clean.

---

## Task 4: Wire the model through messages, model, update, commands, and view

Switch `diffLoadedMsg` to carry the parsed model, add `logLoadedMsg` for the branch-log text path, add model state and `refreshViewport`, route load + resize through it, and remove the dead `colorizeDiff` machinery.

**Files:**
- Modify: `internal/ui/messages.go`
- Modify: `internal/ui/model.go` (fields + `refreshViewport`)
- Modify: `internal/ui/update.go`
- Modify: `internal/ui/commands.go`
- Modify: `internal/ui/view.go` (`mainContent`)
- Modify: `internal/ui/panels.go` (remove dead code)
- Modify: `internal/ui/panels_test.go` (remove dead test)

- [ ] **Step 1: Write the failing test**

Add to `internal/ui/update_test.go`:

```go
func TestUpdate_DiffLoadedRendersModel(t *testing.T) {
	m := newTestModel()
	m.w, m.h = 120, 40
	m.layout()
	d := git.Diff{Files: []git.FileDiff{{
		Path: "x.go", Lang: "go", Adds: 1,
		Hunks: []git.Hunk{{Lines: []git.DiffLine{{Kind: git.LineAdd, NewNo: 1, Text: "hello"}}}},
	}}}
	updated, _ := m.Update(diffLoadedMsg{diff: d})
	got := updated.(Model)
	if got.mainDiff == nil {
		t.Fatal("mainDiff not stored")
	}
	if !strings.Contains(got.viewport.View(), "hello") {
		t.Fatalf("viewport missing rendered content:\n%s", got.viewport.View())
	}
}

func TestUpdate_LogLoadedSetsText(t *testing.T) {
	m := newTestModel()
	m.w, m.h = 120, 40
	m.layout()
	updated, _ := m.Update(logLoadedMsg{text: "abc123  first commit"})
	got := updated.(Model)
	if got.mainDiff != nil {
		t.Fatal("mainDiff should be nil for the log path")
	}
	if !strings.Contains(got.viewport.View(), "first commit") {
		t.Fatalf("viewport missing log text:\n%s", got.viewport.View())
	}
}
```

`strings` is already imported in `update_test.go`? It is not — add `"strings"` to that file's import block.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/ui/ -run 'TestUpdate_DiffLoadedRendersModel|TestUpdate_LogLoadedSetsText' -v`
Expected: **FAIL** — does not compile: `diffLoadedMsg{diff: ...}` unknown field, `logLoadedMsg` undefined, `got.mainDiff` undefined.

- [ ] **Step 3: Update `messages.go`**

In `internal/ui/messages.go`, replace the `diffLoadedMsg` line:

```go
type diffLoadedMsg struct{ diff git.Diff }
type logLoadedMsg struct{ text string }
```

- [ ] **Step 4: Add model fields and `refreshViewport`**

In `internal/ui/model.go`, add three fields to the `Model` struct (after `viewport viewport.Model`):

```go
	mainDiff   *git.Diff // non-nil when the main pane shows a parsed diff
	mainText   string    // plain text for the branch-log path (mainDiff nil)
	hunkRows   []int      // viewport row index of each hunk band, for n/N
```

Add the `"strings"` import to `model.go`, then add this method (e.g. after `layout`):

```go
// refreshViewport re-renders the main pane content at the current viewport
// width: a parsed diff via renderDiff, otherwise the plain branch-log text.
func (m *Model) refreshViewport() {
	if m.mainDiff != nil {
		lines, hunkRows := renderDiff(*m.mainDiff, m.viewport.Width)
		m.viewport.SetContent(strings.Join(lines, "\n"))
		m.hunkRows = hunkRows
		return
	}
	m.viewport.SetContent(m.mainText)
	m.hunkRows = nil
}
```

- [ ] **Step 5: Update `update.go`**

In `internal/ui/update.go`, change the `WindowSizeMsg` case to re-render after layout:

```go
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.layout()
		m.refreshViewport()
		return m, nil
```

Replace the `diffLoadedMsg` case (lines 28-30) with:

```go
	case diffLoadedMsg:
		m.mainDiff = &msg.diff
		m.mainText = ""
		m.refreshViewport()
		return m, nil
	case logLoadedMsg:
		m.mainDiff = nil
		m.mainText = msg.text
		m.refreshViewport()
		return m, nil
```

- [ ] **Step 6: Update `commands.go`**

In `internal/ui/commands.go`, rewrite `loadDiff`, `loadShow`, and `loadBranchLog`:

```go
func loadDiff(ctx context.Context, repo *git.Repo, path string, staged bool) tea.Cmd {
	return func() tea.Msg {
		raw, err := repo.Diff(ctx, path, staged)
		if err != nil {
			return errMsg{err}
		}
		return diffLoadedMsg{diff: git.ParseDiff(raw)}
	}
}

func loadShow(ctx context.Context, repo *git.Repo, hash string) tea.Cmd {
	return func() tea.Msg {
		raw, err := repo.Show(ctx, hash)
		if err != nil {
			return errMsg{err}
		}
		return diffLoadedMsg{diff: git.ParseDiff(raw)}
	}
}

func loadBranchLog(ctx context.Context, repo *git.Repo, name string) tea.Cmd {
	return func() tea.Msg {
		cs, err := repo.Log(ctx, name, 50)
		if err != nil {
			return errMsg{err}
		}
		text := ""
		for _, c := range cs {
			text += c.Hash[:7] + "  " + c.Subject + "\n"
		}
		return logLoadedMsg{text: text}
	}
}
```

- [ ] **Step 7: Update `mainContent` in `view.go` to drop `colorizeDiff`**

In `internal/ui/view.go`, add `"github.com/charmbracelet/x/ansi"` to the import block, then replace the final `return` of `mainContent` (line 311) with:

```go
	title := m.mainTitle()
	body := m.viewport.View()
	if strings.TrimSpace(ansi.Strip(body)) == "" {
		body = m.emptyMainBody()
	}
	return title + "\n" + mutedStyle.Render(strings.Repeat("─", lipgloss.Width(title))) + "\n\n" + body
```

(The `title := m.mainTitle()` and `body := m.viewport.View()` lines at 306-307 are replaced by the block above — delete the old 306-311 lines and paste this. The `ModeCommitting` early-return at the top of `mainContent` is unchanged.)

- [ ] **Step 8: Remove the dead diff-coloring code**

In `internal/ui/panels.go`, delete:
- the `hunkStyle` line from the `var (...)` block (line 29) — it is now unused,
- the entire block from `// diffKind classifies...` (line 252) through the end of `colorizeDiff` (line 311): the `diffKind` type, the `kind*` consts, `diffMetaPrefixes`, `classifyDiffLine`, and `colorizeDiff`.

In `internal/ui/panels_test.go`, delete `TestClassifyDiffLine` (lines 96-119).

- [ ] **Step 9: Verify build and tests are green**

Run: `go build ./...`
Expected: clean (no "declared and not used", no "undefined").

Run: `go test ./internal/ui/...`
Expected: **PASS** — the two new wiring tests pass; `TestMainContentIncludesHeading`, `TestViewRendersFocusMode`, and the rest stay green.

---

# Phase 2 — The visual upgrade (no new dependencies)

## Task 5: Line gutter + full-row add/del backgrounds

Rewrite `renderLine` to lay out a right-aligned old/new line-number gutter, truncate the content to the remaining width, and paint add/del rows with a full-width background.

**Files:**
- Modify: `internal/ui/diffview.go`
- Modify: `internal/ui/diffview_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/ui/diffview_test.go`:

```go
func TestRenderLineGutterAndFullWidth(t *testing.T) {
	const width = 40
	add := renderLine(git.DiffLine{Kind: git.LineAdd, NewNo: 11, Text: "added"}, width)
	del := renderLine(git.DiffLine{Kind: git.LineDel, OldNo: 7, Text: "removed"}, width)

	// add/del rows fill the full width so the background band spans the row
	if w := lipgloss.Width(add); w != width {
		t.Errorf("add row width = %d, want %d", w, width)
	}
	if w := lipgloss.Width(del); w != width {
		t.Errorf("del row width = %d, want %d", w, width)
	}
	// gutter shows the line number on the applicable side
	if !strings.Contains(ansi.Strip(add), "11") {
		t.Errorf("add row missing new line number: %q", ansi.Strip(add))
	}
	if !strings.Contains(ansi.Strip(del), "7") {
		t.Errorf("del row missing old line number: %q", ansi.Strip(del))
	}
	// content is still present
	if !strings.Contains(ansi.Strip(add), "added") {
		t.Errorf("add row missing content: %q", ansi.Strip(add))
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/ui/ -run TestRenderLineGutterAndFullWidth -v`
Expected: **FAIL** — add/del row width is not 40 (the plain `renderLine` does not pad), and the gutter numbers are absent.

- [ ] **Step 3: Implement gutter + backgrounds**

In `internal/ui/diffview.go`, add `"strings"` and `"github.com/charmbracelet/lipgloss"` to the imports, then add this const/var block below the imports:

```go
const (
	diffNumWidth   = 4
	diffGutterCols = diffNumWidth + 1 + diffNumWidth + 1 + 1 + 1 // "NNNN NNNN │ " = 12 cells
)

var (
	diffGutterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	addRowStyle     = lipgloss.NewStyle().Background(lipgloss.Color("22")) // dim green
	delRowStyle     = lipgloss.NewStyle().Background(lipgloss.Color("52")) // dim red
)
```

Replace `renderLine` with:

```go
func renderLine(ln git.DiffLine, width int) string {
	contentWidth := width - diffGutterCols
	if contentWidth < 0 {
		contentWidth = 0
	}
	row := gutter(ln) + ansi.Truncate(ln.Text, contentWidth, "…")
	switch ln.Kind {
	case git.LineAdd:
		return addRowStyle.Width(width).Render(row)
	case git.LineDel:
		return delRowStyle.Width(width).Render(row)
	default:
		return row // context rows: no background, no padding
	}
}

func gutter(ln git.DiffLine) string {
	return diffGutterStyle.Render(fmt.Sprintf("%s %s │ ", numCell(ln.OldNo), numCell(ln.NewNo)))
}

func numCell(n int) string {
	if n == 0 {
		return strings.Repeat(" ", diffNumWidth)
	}
	return fmt.Sprintf("%*d", diffNumWidth, n)
}
```

- [ ] **Step 4: Run to verify green**

Run: `go test ./internal/ui/ -run 'TestRenderLine|TestRenderDiff' -v`
Expected: **PASS**. (`TestRenderDiffTruncatesLongLines` still holds: a context line is unpadded but still truncated to ≤ width.)

> Note: real Go diffs contain tabs; tab expansion (Task 11) makes the width math exact for tab-indented code. Tests here use space-only content.

---

## Task 6: Inline file-header band with change-bar

Replace the plain file-header line with `path  ▰▰▰▱▱  +a -b`, a fixed 5-cell add/del proportion bar, painted full width.

**Files:**
- Modify: `internal/ui/diffview.go`
- Modify: `internal/ui/diffview_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/ui/diffview_test.go`:

```go
func TestChangeBarWidth(t *testing.T) {
	if w := lipgloss.Width(changeBar(12, 3)); w != changeBarWidth {
		t.Errorf("changeBar width = %d, want %d", w, changeBarWidth)
	}
	if w := lipgloss.Width(changeBar(0, 0)); w != changeBarWidth {
		t.Errorf("empty changeBar width = %d, want %d", w, changeBarWidth)
	}
}

func TestRenderFileHeaderBand(t *testing.T) {
	const width = 50
	h := renderFileHeader(git.FileDiff{Path: "x.go", Lang: "go", Adds: 12, Dels: 3}, width)
	if w := lipgloss.Width(h); w != width {
		t.Errorf("file header width = %d, want %d", w, width)
	}
	plain := ansi.Strip(h)
	for _, want := range []string{"x.go", "+12 -3"} {
		if !strings.Contains(plain, want) {
			t.Errorf("file header missing %q: %q", want, plain)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/ui/ -run 'TestChangeBar|TestRenderFileHeaderBand' -v`
Expected: **FAIL** — `undefined: changeBar`, `undefined: changeBarWidth`, and the header is not padded to width.

- [ ] **Step 3: Implement**

In `internal/ui/diffview.go`, add `changeBarWidth = 5` to the `const` block and `fileHeaderStyle = lipgloss.NewStyle().Bold(true)` to the `var` block. Then replace `renderFileHeader` and add `changeBar` (reusing `addStyle`/`delStyle` from `panels.go`):

```go
func renderFileHeader(f git.FileDiff, width int) string {
	head := fmt.Sprintf("%s  %s  +%d -%d", f.Path, changeBar(f.Adds, f.Dels), f.Adds, f.Dels)
	return fileHeaderStyle.Width(width).Render(ansi.Truncate(head, width, "…"))
}

// changeBar is a fixed-width bar whose green/red split is proportional to the
// add/del ratio. All cells are empty when there are no changes.
func changeBar(adds, dels int) string {
	total := adds + dels
	if total == 0 {
		return strings.Repeat("▱", changeBarWidth)
	}
	green := (adds*changeBarWidth + total/2) / total // rounded
	if green > changeBarWidth {
		green = changeBarWidth
	}
	return addStyle.Render(strings.Repeat("▰", green)) + delStyle.Render(strings.Repeat("▰", changeBarWidth-green))
}
```

- [ ] **Step 4: Run to verify green**

Run: `go test ./internal/ui/ -run 'TestChangeBar|TestRenderFileHeaderBand|TestRenderDiff' -v`
Expected: **PASS** (`TestRenderDiffProjectsModel` still finds `+1 -1` because the format is unchanged for the counts).

---

## Task 7: Hunk band styling

Paint the hunk band full width with the accent background so each function-context block stands out.

**Files:**
- Modify: `internal/ui/diffview.go`
- Modify: `internal/ui/diffview_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/ui/diffview_test.go`:

```go
func TestRenderHunkBandFullWidth(t *testing.T) {
	const width = 40
	band := renderHunkBand(git.Hunk{Header: "func View()"}, width)
	if w := lipgloss.Width(band); w != width {
		t.Errorf("hunk band width = %d, want %d", w, width)
	}
	if !strings.Contains(ansi.Strip(band), "func View()") {
		t.Errorf("hunk band missing header: %q", ansi.Strip(band))
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/ui/ -run TestRenderHunkBandFullWidth -v`
Expected: **FAIL** — band width is the header length, not 40.

- [ ] **Step 3: Implement**

In `internal/ui/diffview.go`, add to the `var` block:

```go
	hunkBandStyle = lipgloss.NewStyle().Background(accentColor).Foreground(lipgloss.Color("0"))
```

Replace `renderHunkBand`:

```go
func renderHunkBand(h git.Hunk, width int) string {
	s := "@@"
	if h.Header != "" {
		s = "@@ " + h.Header
	}
	return hunkBandStyle.Width(width).Render(ansi.Truncate(s, width, "…"))
}
```

- [ ] **Step 4: Run to verify green**

Run: `go test ./internal/ui/ -run 'TestRenderHunkBandFullWidth|TestRenderDiff' -v`
Expected: **PASS**.

---

## Task 8: Scroll indicator

Reserve one column for a vertical scroll track and render a thumb sized from the scroll position. The track is shown for both diff and branch-log panes.

**Files:**
- Modify: `internal/ui/view.go` (add `scrollIndicatorWidth` const; join the track in `mainContent`)
- Modify: `internal/ui/model.go` (`mainViewportSize` subtracts the reserved column)
- Modify: `internal/ui/diffview.go` (add `scrollIndicator`)
- Modify: `internal/ui/diffview_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/ui/diffview_test.go`:

```go
func TestScrollIndicatorThumbMoves(t *testing.T) {
	const height = 10
	// total content far exceeds height → a thumb that tracks the offset
	top := scrollIndicator(0, 100, height)
	bottom := scrollIndicator(90, 100, height)
	if len(top) != height || len(bottom) != height {
		t.Fatalf("indicator height = %d/%d, want %d", len(top), len(bottom), height)
	}
	// each cell is one column wide
	for _, c := range top {
		if w := lipgloss.Width(c); w != 1 {
			t.Fatalf("indicator cell width = %d, want 1", w)
		}
	}
	// the thumb glyph appears near the top when offset is 0 and near the
	// bottom when offset is high
	if ansi.Strip(top[0]) != "█" {
		t.Errorf("expected thumb at the top, got %q", ansi.Strip(top[0]))
	}
	if ansi.Strip(bottom[height-1]) != "█" {
		t.Errorf("expected thumb at the bottom, got %q", ansi.Strip(bottom[height-1]))
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/ui/ -run TestScrollIndicatorThumbMoves -v`
Expected: **FAIL** — `undefined: scrollIndicator`.

- [ ] **Step 3: Implement the indicator**

In `internal/ui/diffview.go`, add:

```go
// scrollIndicator returns `height` one-column cells: a faint track with a
// highlighted thumb whose size and position reflect offset within total.
func scrollIndicator(offset, total, height int) []string {
	if height <= 0 {
		return nil
	}
	track := make([]string, height)
	railStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	thumbStyle := lipgloss.NewStyle().Foreground(accentColor)
	if total <= height {
		for i := range track {
			track[i] = railStyle.Render("│")
		}
		return track
	}
	thumbSize := max(1, height*height/total)
	pos := 0
	if maxOffset := total - height; maxOffset > 0 {
		pos = offset * (height - thumbSize) / maxOffset
	}
	for i := 0; i < height; i++ {
		if i >= pos && i < pos+thumbSize {
			track[i] = thumbStyle.Render("█")
		} else {
			track[i] = railStyle.Render("│")
		}
	}
	return track
}
```

- [ ] **Step 4: Run the indicator test (green), then wire it in**

Run: `go test ./internal/ui/ -run TestScrollIndicatorThumbMoves -v`
Expected: **PASS**.

In `internal/ui/view.go`, add `scrollIndicatorWidth = 1` to the `const (...)` block (lines 12-18, alongside `topBarHeight`).

In `internal/ui/model.go`, in `mainViewportSize`, subtract the reserved column from the width calc:

```go
	w = mainOuter - borderBlur.GetHorizontalFrameSize() - scrollIndicatorWidth
```

In `internal/ui/view.go`, update `mainContent` so the non-empty branch joins the track to the right of the viewport (the `ModeCommitting` early return and the empty-body substitution stay as in Task 4):

```go
	title := m.mainTitle()
	body := m.viewport.View()
	if strings.TrimSpace(ansi.Strip(body)) == "" {
		body = m.emptyMainBody()
	} else {
		track := strings.Join(scrollIndicator(m.viewport.YOffset, m.viewport.TotalLineCount(), m.viewport.Height), "\n")
		body = lipgloss.JoinHorizontal(lipgloss.Top, body, track)
	}
	return title + "\n" + mutedStyle.Render(strings.Repeat("─", lipgloss.Width(title))) + "\n\n" + body
```

- [ ] **Step 5: Run the full UI suite**

Run: `go test ./internal/ui/...`
Expected: **PASS** — `TestViewFitsTerminalWidth`/`Height` still hold (viewport is now 1 column narrower; the track restores the column, so the pane width is unchanged).

---

# Phase 3 — Word-level highlighting

## Task 9: Word diff (LCS) and replace-run pairing

Add a pure rune-level LCS word diff, then pair each deletion in a replace-run with the corresponding addition and highlight only the differing segments.

**Files:**
- Modify: `internal/ui/diffview.go`
- Modify: `internal/ui/diffview_test.go`

- [ ] **Step 1: Write the failing test (word diff)**

Add to `internal/ui/diffview_test.go`:

```go
func TestWordDiffSubstitution(t *testing.T) {
	aSegs, bSegs := wordDiff("foo bar", "foo baz")
	wantA := []seg{{Text: "foo ba", Changed: false}, {Text: "r", Changed: true}}
	wantB := []seg{{Text: "foo ba", Changed: false}, {Text: "z", Changed: true}}
	if !segsEqual(aSegs, wantA) {
		t.Errorf("aSegs = %+v, want %+v", aSegs, wantA)
	}
	if !segsEqual(bSegs, wantB) {
		t.Errorf("bSegs = %+v, want %+v", bSegs, wantB)
	}
}

func TestWordDiffInsertion(t *testing.T) {
	aSegs, bSegs := wordDiff("foo", "foo bar")
	if !segsEqual(aSegs, []seg{{Text: "foo", Changed: false}}) {
		t.Errorf("aSegs = %+v", aSegs)
	}
	if !segsEqual(bSegs, []seg{{Text: "foo", Changed: false}, {Text: " bar", Changed: true}}) {
		t.Errorf("bSegs = %+v", bSegs)
	}
}

func TestWordDiffOneSideEmpty(t *testing.T) {
	aSegs, bSegs := wordDiff("", "abc")
	if len(aSegs) != 0 {
		t.Errorf("aSegs = %+v, want empty", aSegs)
	}
	if !segsEqual(bSegs, []seg{{Text: "abc", Changed: true}}) {
		t.Errorf("bSegs = %+v", bSegs)
	}
}

func segsEqual(a, b []seg) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/ui/ -run TestWordDiff -v`
Expected: **FAIL** — `undefined: wordDiff`, `undefined: seg`.

- [ ] **Step 3: Implement `wordDiff`**

In `internal/ui/diffview.go`, add:

```go
type seg struct {
	Text    string
	Changed bool
}

// wordDiff computes a rune-level LCS of a and b and returns each side split
// into unchanged/changed segments. Pure.
func wordDiff(a, b string) (aSegs, bSegs []seg) {
	ar, br := []rune(a), []rune(b)
	n, m := len(ar), len(br)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if ar[i] == br[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	aChanged := make([]bool, n)
	bChanged := make([]bool, m)
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case ar[i] == br[j]:
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			aChanged[i] = true
			i++
		default:
			bChanged[j] = true
			j++
		}
	}
	for ; i < n; i++ {
		aChanged[i] = true
	}
	for ; j < m; j++ {
		bChanged[j] = true
	}
	return groupSegs(ar, aChanged), groupSegs(br, bChanged)
}

func groupSegs(rs []rune, changed []bool) []seg {
	var out []seg
	for i := 0; i < len(rs); {
		j := i
		for j < len(rs) && changed[j] == changed[i] {
			j++
		}
		out = append(out, seg{Text: string(rs[i:j]), Changed: changed[i]})
		i = j
	}
	return out
}
```

- [ ] **Step 4: Run word-diff tests (green)**

Run: `go test ./internal/ui/ -run TestWordDiff -v`
Expected: **PASS**.

- [ ] **Step 5: Write the failing integration test**

Add to `internal/ui/diffview_test.go`:

```go
func TestRenderDiffWordHighlightPreservesProjectionAndWidth(t *testing.T) {
	const width = 50
	d := git.Diff{Files: []git.FileDiff{{
		Path: "x.go", Lang: "go", Adds: 1, Dels: 1,
		Hunks: []git.Hunk{{Lines: []git.DiffLine{
			{Kind: git.LineDel, OldNo: 1, Text: "value := foo"},
			{Kind: git.LineAdd, NewNo: 1, Text: "value := bar"},
		}}},
	}}}
	lines, _ := renderDiff(d, width)
	for _, l := range lines {
		plain := ansi.Strip(l)
		if strings.Contains(plain, "foo") {
			if !strings.Contains(plain, "value := foo") {
				t.Errorf("del projection altered: %q", plain)
			}
			if w := lipgloss.Width(l); w != width {
				t.Errorf("del row width = %d, want %d", w, width)
			}
		}
		if strings.Contains(plain, "bar") {
			if !strings.Contains(plain, "value := bar") {
				t.Errorf("add projection altered: %q", plain)
			}
		}
	}
}
```

- [ ] **Step 6: Run to verify it fails**

Run: `go test ./internal/ui/ -run TestRenderDiffWordHighlightPreservesProjectionAndWidth -v`
Expected: **PASS or FAIL depending on current width** — it likely passes already (plain renderLine), so this test mainly guards the next step. If it passes now, proceed; the implementation must keep it passing.

- [ ] **Step 7: Wire replace-run pairing into `renderDiff`**

In `internal/ui/diffview.go`, add to the `var` block:

```go
	addWordStyle = lipgloss.NewStyle().Background(lipgloss.Color("28")) // brighter green
	delWordStyle = lipgloss.NewStyle().Background(lipgloss.Color("88")) // brighter red
```

Change the inner loop of `renderDiff` from the per-line loop to:

```go
			lines = append(lines, renderHunkLines(h.Lines, width)...)
```

(Remove the `for _, ln := range h.Lines { ... renderLine(ln, width) }` loop; keep the `hunkRows`/`renderHunkBand` lines above it.)

Add:

```go
// renderHunkLines renders a hunk's lines, detecting replace runs (consecutive
// deletions immediately followed by additions) so paired lines get word-level
// highlighting; everything else uses the plain row renderer.
func renderHunkLines(lines []git.DiffLine, width int) []string {
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); {
		if lines[i].Kind == git.LineDel {
			j := i
			for j < len(lines) && lines[j].Kind == git.LineDel {
				j++
			}
			k := j
			for k < len(lines) && lines[k].Kind == git.LineAdd {
				k++
			}
			if k > j { // a real replace run (dels followed by adds)
				out = append(out, renderReplaceRun(lines[i:j], lines[j:k], width)...)
				i = k
				continue
			}
		}
		out = append(out, renderLine(lines[i], width))
		i++
	}
	return out
}

func renderReplaceRun(dels, adds []git.DiffLine, width int) []string {
	pairs := min(len(dels), len(adds))
	out := make([]string, 0, len(dels)+len(adds))
	for x, d := range dels {
		if x < pairs {
			dSegs, _ := wordDiff(d.Text, adds[x].Text)
			out = append(out, renderSegLine(d, dSegs, delRowStyle, delWordStyle, width))
		} else {
			out = append(out, renderLine(d, width))
		}
	}
	for x, a := range adds {
		if x < pairs {
			_, aSegs := wordDiff(dels[x].Text, a.Text)
			out = append(out, renderSegLine(a, aSegs, addRowStyle, addWordStyle, width))
		} else {
			out = append(out, renderLine(a, width))
		}
	}
	return out
}

func renderSegLine(ln git.DiffLine, segs []seg, rowStyle, wordStyle lipgloss.Style, width int) string {
	contentWidth := width - diffGutterCols
	if contentWidth < 0 {
		contentWidth = 0
	}
	var b strings.Builder
	for _, s := range segs {
		if s.Changed {
			b.WriteString(wordStyle.Render(s.Text))
		} else {
			b.WriteString(s.Text)
		}
	}
	return rowStyle.Width(width).Render(gutter(ln) + ansi.Truncate(b.String(), contentWidth, "…"))
}
```

- [ ] **Step 8: Run to verify green**

Run: `go test ./internal/ui/ -run 'TestRenderDiff|TestWordDiff' -v`
Expected: **PASS** — projection unchanged, paired rows still fill width.

---

# Phase 4 — Small independent adds

## Task 10: Untracked files render as all-additions

Read an untracked file's working-tree content and synthesize an all-add `Diff`; show a one-line model for binary or non-regular files.

**Files:**
- Modify: `internal/git/diff.go` (`SynthesizeUntracked`, `MessageDiff`, `isBinary`)
- Modify: `internal/git/diff_test.go`
- Modify: `internal/ui/commands.go` (`loadDiff` reads untracked files)
- Modify: `internal/ui/update.go` (`loadMainForSelection` passes the untracked flag)

- [ ] **Step 1: Write the failing test**

Add to `internal/git/diff_test.go`:

```go
func TestSynthesizeUntrackedTextFile(t *testing.T) {
	d := SynthesizeUntracked("notes.txt", []byte("alpha\nbeta\n"))
	f := d.Files[0]
	if f.Path != "notes.txt" || f.Adds != 2 || f.Dels != 0 {
		t.Fatalf("synthesized = %+v", f)
	}
	lines := f.Hunks[0].Lines
	if lines[0] != (DiffLine{Kind: LineAdd, NewNo: 1, Text: "alpha"}) {
		t.Errorf("line[0] = %+v", lines[0])
	}
	if lines[1] != (DiffLine{Kind: LineAdd, NewNo: 2, Text: "beta"}) {
		t.Errorf("line[1] = %+v", lines[1])
	}
}

func TestSynthesizeUntrackedBinary(t *testing.T) {
	d := SynthesizeUntracked("img.png", []byte{0x89, 0x50, 0x00, 0x4e})
	line := d.Files[0].Hunks[0].Lines[0]
	if line.Kind != LineContext || line.Text != "Binary file" {
		t.Fatalf("binary model = %+v", line)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/git/ -run TestSynthesizeUntracked -v`
Expected: **FAIL** — `undefined: SynthesizeUntracked`.

- [ ] **Step 3: Implement synthesis**

In `internal/git/diff.go`, add `"bytes"` to the imports, then add:

```go
// MessageDiff builds a one-line informational Diff (binary, unreadable, etc.).
func MessageDiff(path, message string) Diff {
	return Diff{Files: []FileDiff{{
		Path: path, Lang: langFromPath(path),
		Hunks: []Hunk{{Lines: []DiffLine{{Kind: LineContext, Text: message}}}},
	}}}
}

// SynthesizeUntracked builds an all-additions Diff for an untracked file's
// working-tree content. NUL in the scanned prefix yields a "Binary file" model.
func SynthesizeUntracked(path string, content []byte) Diff {
	if isBinary(content) {
		return MessageDiff(path, "Binary file")
	}
	text := strings.TrimSuffix(string(content), "\n")
	var lines []DiffLine
	if text != "" {
		for i, l := range strings.Split(text, "\n") {
			lines = append(lines, DiffLine{Kind: LineAdd, NewNo: i + 1, Text: l})
		}
	}
	return Diff{Files: []FileDiff{{
		Path: path, Lang: langFromPath(path),
		Adds:  len(lines),
		Hunks: []Hunk{{Lines: lines}},
	}}}
}

func isBinary(content []byte) bool {
	n := len(content)
	if n > 8000 {
		n = 8000
	}
	return bytes.IndexByte(content[:n], 0) >= 0
}
```

- [ ] **Step 4: Run synthesis tests (green)**

Run: `go test ./internal/git/ -run TestSynthesizeUntracked -v`
Expected: **PASS**.

- [ ] **Step 5: Wire the untracked read into `commands.go`**

In `internal/ui/commands.go`, add `"os"` and `"path/filepath"` to the imports, then rewrite `loadDiff` to take an `untracked` flag and read the file when set:

```go
func loadDiff(ctx context.Context, repo *git.Repo, path string, staged, untracked bool) tea.Cmd {
	return func() tea.Msg {
		if untracked {
			full := filepath.Join(repo.Root(), path)
			info, err := os.Stat(full)
			if err != nil {
				return diffLoadedMsg{diff: git.MessageDiff(path, "Cannot read file")}
			}
			if !info.Mode().IsRegular() {
				return diffLoadedMsg{diff: git.MessageDiff(path, "Not a regular file")}
			}
			content, err := os.ReadFile(full)
			if err != nil {
				return diffLoadedMsg{diff: git.MessageDiff(path, "Cannot read file")}
			}
			return diffLoadedMsg{diff: git.SynthesizeUntracked(path, content)}
		}
		raw, err := repo.Diff(ctx, path, staged)
		if err != nil {
			return errMsg{err}
		}
		return diffLoadedMsg{diff: git.ParseDiff(raw)}
	}
}
```

- [ ] **Step 6: Pass the untracked flag at the call site**

In `internal/ui/update.go`, in `loadMainForSelection`, update the `PanelFiles` case:

```go
	case PanelFiles:
		if i := m.cursor[PanelFiles]; i < len(m.files) {
			f := m.files[i]
			return loadDiff(m.ctx, m.repo, f.Path, f.IsStaged(), f.Untracked)
		}
```

- [ ] **Step 7: Verify build and tests**

Run: `go build ./...`
Expected: clean.

Run: `go test ./...`
Expected: **PASS** (all packages).

---

## Task 11: Tab expansion and trailing-whitespace markers

Expand tabs to a fixed stop before width math, and render trailing spaces as a muted middot run.

**Files:**
- Modify: `internal/ui/diffview.go`
- Modify: `internal/ui/diffview_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/ui/diffview_test.go`:

```go
func TestExpandTabs(t *testing.T) {
	cases := map[string]string{
		"a\tb":  "a   b",  // 'a' at col 0, tab fills to col 4
		"\tx":   "    x",  // leading tab → 4 spaces
		"ab\tc": "ab  c",  // 'ab' at col 2, tab fills to col 4
		"plain": "plain",
	}
	for in, want := range cases {
		if got := expandTabs(in, diffTabWidth); got != want {
			t.Errorf("expandTabs(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMarkTrailingWhitespaceInProjection(t *testing.T) {
	const width = 40
	line := renderLine(git.DiffLine{Kind: git.LineContext, OldNo: 1, NewNo: 1, Text: "code  "}, width)
	if !strings.Contains(ansi.Strip(line), "code··") {
		t.Errorf("trailing spaces not shown as middots: %q", ansi.Strip(line))
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/ui/ -run 'TestExpandTabs|TestMarkTrailingWhitespaceInProjection' -v`
Expected: **FAIL** — `undefined: expandTabs`, and trailing spaces are not yet marked.

- [ ] **Step 3: Implement the helpers and a shared content formatter**

In `internal/ui/diffview.go`, add `diffTabWidth = 4` to the `const` block, then add:

```go
// diffContent prepares a line's content for display: tabs expanded to a fixed
// stop and trailing whitespace shown as muted middots.
func diffContent(text string) string {
	exp := expandTabs(text, diffTabWidth)
	body, trail := splitTrailingWS(exp)
	return body + mutedDots(trail)
}

func expandTabs(s string, tabWidth int) string {
	if !strings.ContainsRune(s, '\t') {
		return s
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			n := tabWidth - col%tabWidth
			b.WriteString(strings.Repeat(" ", n))
			col += n
			continue
		}
		b.WriteRune(r)
		col++
	}
	return b.String()
}

func splitTrailingWS(s string) (body, trail string) {
	body = strings.TrimRight(s, " ")
	return body, s[len(body):]
}

func mutedDots(trail string) string {
	if trail == "" {
		return ""
	}
	return mutedStyle.Render(strings.Repeat("·", len(trail)))
}
```

Use `diffContent` in `renderLine` — change its content expression:

```go
	row := gutter(ln) + ansi.Truncate(diffContent(ln.Text), contentWidth, "…")
```

In `renderSegLine`, expand tabs per segment so word-highlighted code lines measure correctly (trailing-whitespace marking stays on the plain `renderLine` path):

```go
	for _, s := range segs {
		txt := expandTabs(s.Text, diffTabWidth)
		if s.Changed {
			b.WriteString(wordStyle.Render(txt))
		} else {
			b.WriteString(txt)
		}
	}
```

- [ ] **Step 4: Run to verify green**

Run: `go test ./internal/ui/ -run 'TestExpandTabs|TestMarkTrailingWhitespace|TestRenderDiff|TestRenderLine' -v`
Expected: **PASS**.

---

## Task 12: Hunk navigation (`n` / `N`)

Bind `n`/`N` to jump the viewport to the next/previous hunk band, guarded so the keys are inert unless a diff is shown.

**Files:**
- Modify: `internal/ui/keys.go`
- Modify: `internal/ui/update.go` (key handling + offset helpers)
- Modify: `internal/ui/update_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/ui/update_test.go`:

```go
func TestUpdate_NAndNJumpBetweenHunks(t *testing.T) {
	m := newTestModel()
	m.mainDiff = &git.Diff{} // non-nil: hunk nav is enabled
	m.hunkRows = []int{2, 8}
	m.viewport.Height = 4
	m.viewport.SetContent(strings.Repeat("x\n", 30))

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if off := next.(Model).viewport.YOffset; off != 2 {
		t.Fatalf("after n: YOffset = %d, want 2", off)
	}

	mid := next.(Model)
	next2, _ := mid.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if off := next2.(Model).viewport.YOffset; off != 8 {
		t.Fatalf("after second n: YOffset = %d, want 8", off)
	}

	prev, _ := next2.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	if off := prev.(Model).viewport.YOffset; off != 2 {
		t.Fatalf("after N: YOffset = %d, want 2", off)
	}
}

func TestNextPrevOffsetClamp(t *testing.T) {
	rows := []int{1, 10, 20}
	if got := nextOffset(rows, 20); got != 20 {
		t.Errorf("nextOffset past last = %d, want 20", got)
	}
	if got := prevOffset(rows, 1); got != 1 {
		t.Errorf("prevOffset before first = %d, want 1", got)
	}
}
```

> `tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")}.String()` returns `"N"`, which matches `keyHunkPrev`. (There is no `tea.KeyShiftN` in bubbletea v1.3.10.)

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/ui/ -run 'TestUpdate_NAndNJumpBetweenHunks|TestNextPrevOffsetClamp' -v`
Expected: **FAIL** — `undefined: nextOffset`/`prevOffset`, and `n`/`N` are unhandled (offset stays 0).

- [ ] **Step 3: Add keys**

In `internal/ui/keys.go`, add to the const block:

```go
	keyHunkNext = "n"
	keyHunkPrev = "N"
```

- [ ] **Step 4: Handle the keys and add offset helpers**

In `internal/ui/update.go`, add two cases to the normal-mode `switch msg.String()` (e.g. after the `keyEnter` case, before the closing `}`):

```go
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
```

Add the helpers at the end of `update.go`:

```go
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
```

- [ ] **Step 5: Run to verify green**

Run: `go test ./internal/ui/ -run 'TestUpdate_NAndNJump|TestNextPrevOffsetClamp' -v`
Expected: **PASS** (`SetYOffset` clamps to the content's max offset internally).

Optionally surface the keys in help (`view.go` `helpOverlay`) — out of scope for tests; mention it in the final commit if added.

---

# Phase 5 — Syntax highlighting

## Task 13: `chroma/v2` syntax colors with a plain fallback

Lex each line's content by language and map token types to a small foreground palette, composing under the row/word backgrounds. Any lexer error or unknown extension falls back to plain text, so the plain-text projection is never altered.

**Files:**
- Modify: `go.mod` / `go.sum` (add the dependency)
- Modify: `internal/ui/diffview.go` (`highlightLine`, `syntaxColor`, thread `lang`)
- Modify: `internal/ui/diffview_test.go`

- [ ] **Step 1: Add the dependency**

Run: `go get github.com/alecthomas/chroma/v2@latest`
Then: `go mod tidy`
Expected: `go.mod` gains `github.com/alecthomas/chroma/v2` as a direct require; `go.sum` updates. (Requires network access.)

- [ ] **Step 2: Write the failing test**

Add to `internal/ui/diffview_test.go`:

```go
func TestHighlightLinePreservesPlainText(t *testing.T) {
	const code = "func main() { return }"
	if got := ansi.Strip(highlightLine(code, "go")); got != code {
		t.Errorf("highlighted projection = %q, want %q", got, code)
	}
}

func TestHighlightLineUnknownLangFallsBack(t *testing.T) {
	const code = "some text"
	if got := highlightLine(code, ""); got != code {
		t.Errorf("empty lang should be returned verbatim, got %q", got)
	}
	if got := highlightLine(code, "no-such-lang"); ansi.Strip(got) != code {
		t.Errorf("unknown lang projection = %q, want %q", ansi.Strip(got), code)
	}
}

func TestRenderDiffSyntaxKeepsProjection(t *testing.T) {
	const width = 60
	d := git.Diff{Files: []git.FileDiff{{
		Path: "x.go", Lang: "go", Adds: 1,
		Hunks: []git.Hunk{{Lines: []git.DiffLine{
			{Kind: git.LineAdd, NewNo: 1, Text: "func main() {}"},
		}}},
	}}}
	lines, _ := renderDiff(d, width)
	joined := ansi.Strip(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "func main() {}") {
		t.Errorf("syntax highlighting altered the projection:\n%s", joined)
	}
}
```

- [ ] **Step 3: Run to verify it fails**

Run: `go test ./internal/ui/ -run 'TestHighlightLine|TestRenderDiffSyntaxKeepsProjection' -v`
Expected: **FAIL** — `undefined: highlightLine`.

- [ ] **Step 4: Implement `highlightLine` and thread `lang`**

In `internal/ui/diffview.go`, add the chroma imports:

```go
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
```

Add:

```go
// highlightLine applies syntax foreground colors to a single line of code.
// Foreground (syntax) and background (row/word) are independent ANSI
// attributes, so they compose. Any error or unknown language returns the input
// unchanged, keeping the plain-text projection intact.
func highlightLine(text, lang string) string {
	if lang == "" {
		return text
	}
	lexer := lexers.Get(lang)
	if lexer == nil {
		return text
	}
	it, err := lexer.Tokenise(nil, text)
	if err != nil {
		return text
	}
	var b strings.Builder
	for _, tok := range it.Tokens() {
		if c, ok := syntaxColor(tok.Type); ok {
			b.WriteString(lipgloss.NewStyle().Foreground(c).Render(tok.Value))
		} else {
			b.WriteString(tok.Value)
		}
	}
	out := b.String()
	if !strings.HasSuffix(text, "\n") {
		out = strings.TrimSuffix(out, "\n") // lexers may append a trailing newline
	}
	return out
}

// syntaxColor maps a chroma token category to a fixed foreground color.
func syntaxColor(t chroma.TokenType) (lipgloss.Color, bool) {
	switch {
	case t.InCategory(chroma.Comment):
		return lipgloss.Color("245"), true
	case t.InCategory(chroma.Keyword):
		return lipgloss.Color("13"), true
	case t.InCategory(chroma.LiteralString):
		return lipgloss.Color("10"), true
	case t.InCategory(chroma.LiteralNumber):
		return lipgloss.Color("11"), true
	case t.InCategory(chroma.Name):
		return lipgloss.Color("14"), true
	}
	return "", false
}
```

> If the chroma token API differs in the resolved version, confirm with `go doc github.com/alecthomas/chroma/v2 TokenType` and `go doc github.com/alecthomas/chroma/v2 Iterator`. The expected shape: `lexer.Tokenise(nil, text) (chroma.Iterator, error)`, `iterator.Tokens() []chroma.Token`, `token.Type chroma.TokenType`, `token.Value string`, `TokenType.InCategory(chroma.Keyword) bool`.

Now thread `lang` through the renderers. Change `diffContent`:

```go
func diffContent(text, lang string) string {
	exp := expandTabs(text, diffTabWidth)
	body, trail := splitTrailingWS(exp)
	return highlightLine(body, lang) + mutedDots(trail)
}
```

Add `lang string` to the signatures of `renderLine`, `renderHunkLines`, `renderReplaceRun`, and `renderSegLine`, and pass it through:

- `renderDiff` inner loop: `lines = append(lines, renderHunkLines(h.Lines, f.Lang, width)...)`
- `renderLine(ln git.DiffLine, lang string, width int)`: content becomes `ansi.Truncate(diffContent(ln.Text, lang), contentWidth, "…")`.
- `renderHunkLines(lines []git.DiffLine, lang string, width int)`: pass `lang` to `renderReplaceRun(lines[i:j], lines[j:k], lang, width)` and `renderLine(lines[i], lang, width)`.
- `renderReplaceRun(dels, adds []git.DiffLine, lang string, width int)`: pass `lang` to `renderSegLine(..., lang, ...)` and `renderLine(..., lang, width)`.
- `renderSegLine(ln git.DiffLine, segs []seg, rowStyle, wordStyle lipgloss.Style, lang string, width int)`: in the segment loop, highlight each segment's expanded text: `txt := highlightLine(expandTabs(s.Text, diffTabWidth), lang)`.

- [ ] **Step 5: Run to verify green**

Run: `go test ./internal/ui/...`
Expected: **PASS** — all projection/width tests hold; syntax colors are invisible to `ansi.Strip`.

- [ ] **Step 6: Full build and test**

Run: `go build ./...` then `go test ./...`
Expected: clean build, all packages green.

- [ ] **Step 7: Manual smoke check (optional but recommended)**

Run `./loom.exe` (or `go run .`) in this repo, select a modified `.go` file, and confirm: gutter line numbers, green/red row bands, word-level highlights on an edited line, syntax colors, the file-header bar, hunk bands, `n`/`N` navigation, and the scroll track. Try an untracked file (all-add) and a binary file ("Binary file").

---

## Task 14: Final commit (approval-gated)

Per the repo owner's git workflow, this is the single commit for the whole feature. Do **not** commit earlier and do **not** add a co-author.

- [ ] **Step 1: Self-review the diff**

Run: `git status` and `git diff --stat`
Confirm: no debug prints, no commented-out code, no leftover `colorizeDiff`/`classifyDiffLine` references (`grep -rn colorizeDiff internal/` returns nothing), and `go test ./...` is green.

- [ ] **Step 2: Present the change set for approval**

Show the user the files to commit (M/A) with one-line summaries and this proposed message, then ask: "Awaiting approval. Proceed? (yes/no)".

Proposed message:

```
feat(ui): render diffs from a parsed model with editor-style highlighting

Replace line-by-line colorizeDiff with git.ParseDiff + renderDiff: gutter
line numbers, full-row add/del backgrounds, word-level highlighting, syntax
colors (chroma), trailing-whitespace markers, hunk nav (n/N), a scroll
indicator, and untracked-as-all-adds. Fixes the wrapped-line color bug by
truncating instead of wrapping, and unifies main-pane sizing.
```

- [ ] **Step 3: Commit only after explicit approval**

```bash
git add internal/git/diff.go internal/git/diff_test.go \
        internal/ui/diffview.go internal/ui/diffview_test.go \
        internal/ui/messages.go internal/ui/model.go internal/ui/update.go \
        internal/ui/update_test.go internal/ui/commands.go internal/ui/view.go \
        internal/ui/panels.go internal/ui/panels_test.go internal/ui/keys.go \
        go.mod go.sum docs/plans/2026-06-15-diff-panel.md
git commit -m "feat(ui): render diffs from a parsed model with editor-style highlighting" -m "..."
```

---

## Self-Review (author's checklist against the spec)

**Spec coverage**
- §4 model + `ParseDiff` → Task 1. §4 untracked synthesis → Task 10.
- §5 inline file header + change-bar → Task 6; hunk band → Task 7; gutter → Task 5; row backgrounds → Task 5; word highlight → Task 9; truncation → Task 2/5; trailing whitespace + tab expansion → Task 11; syntax → Task 13.
- §6 `diffLoadedMsg{diff}` + `logLoadedMsg` → Task 4; `mainDiff`/`mainText`/`hunkRows` + `refreshViewport` → Task 4; resize re-render → Task 4; `n`/`N` keys → Task 12; `mainContent` drops `colorizeDiff` → Task 4; scroll indicator → Task 8.
- §7 testing strategy → parser table tests (Task 1), word-diff boundaries (Task 9), renderer width/projection (Tasks 2/5/6/7/9), hunk nav (Task 12), untracked (Task 10), syntax projection/fallback (Task 13). The three `==` functions (`commandStateText`, `footerActions`, `panelTitle`) are untouched; `TestClassifyDiffLine` is intentionally removed with its dead code.
- §9 phasing preserved (each phase independently shippable and tested).

**Deviations** are listed in "Decisions & Deviations" above (preamble suppression, inline vs sticky header, ASCII `-`, `mainViewportSize` refactor, lang threading in Task 13, always-on scroll track).

**Type consistency:** `seg`, `DiffLine`, `Hunk`, `FileDiff`, `Diff`, `renderDiff`/`renderLine`/`renderHunkLines`/`renderReplaceRun`/`renderSegLine` signatures are consistent across tasks (the `lang` parameter is added uniformly in Task 13). `diffGutterCols`/`diffNumWidth`/`changeBarWidth`/`diffTabWidth`/`scrollIndicatorWidth` are each defined once.
