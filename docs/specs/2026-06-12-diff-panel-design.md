# loom — Diff Panel Design Spec

**Date:** 2026-06-12
**Status:** Approved (design), implementation pending
**Location:** `C:\Users\kael02\IdeaProjects\loom`
**Scope:** new `internal/git/diff.go`, new `internal/ui/diffview.go`; edits to `internal/ui/{messages,model,update,view,panels,commands}.go`; new `chroma/v2` dependency.

## 1. Problem

The main pane renders diffs as flat, foreground-colored text. `mainContent()` (`view.go:311`) calls `colorizeDiff(m.viewport.View())`, which classifies **already-wrapped viewport text** line by line. This caps how readable the diff can be and carries a latent bug:

- The `@@ -a,b +c,d @@` ranges are discarded, so there are **no line numbers**.
- `-`/`+` lines are never **paired**, so there is no intra-line (word-level) highlighting.
- A long `+`/`-` line that soft-wraps **loses its color** on the wrapped continuation row.
- Changes are signalled only by *foreground* color on black; blocks of change don't stand out, and the code keeps no syntax color.

The target is the editor / Antigravity look: the diff signal lives in a **full-row background**, the exact changed token is highlighted, and code keeps its **syntax colors** — all driven by a parsed model rather than line-by-line guessing.

## 2. Goals / Non-goals

**Goals**
- Parse diff/`git show` output into a structured model that is the single source of truth.
- Render: file header (path + change-bar + `+adds −dels` + state), function-context hunk bands, old/new line-number gutter, full-row add/del backgrounds, precise word-level highlighting, syntax highlighting, visible trailing whitespace, a scroll indicator, and hunk-to-hunk navigation (`n`/`N`).
- Render untracked files as all-additions instead of an empty pane.
- Fix the wrapped-line color bug by switching diffs from wrap to truncate.

**Non-goals**
- **No interactive accept/reject** (Antigravity's per-chunk/line staging). Staging stays in the Files panel via the existing `space`/`d` keys.
- **No side-by-side** view — the center pane is too narrow (~56 cols at 120-wide). Unified only.
- **No horizontal scroll in v1** — long lines truncate with `…`; h-scroll is a later add if missed.
- **No theming/config** — single hard-coded palette, like the rest of the UI.
- The branch-log view (plain text in the same pane) is **not** a diff and stays plain text.

## 3. Guiding principle

The existing color spec's rule — *text-producing functions return plain strings; color is applied where they are rendered* — adapts here as:

> **The parser is the plain, testable single source of truth. All styling lives in the renderer.**

`git.ParseDiff` produces a model with zero styling; tests assert on the model and on the renderer's *plain-text projection* and *cell widths* (`lipgloss.Width`), never on raw ANSI. The viewport now holds a pre-styled string (full-row backgrounds require width-padded, styled lines), but nothing in the test suite inspects escape codes, so this stays verifiable.

## 4. The model (`internal/git/diff.go`, new — beside the existing status/branch/log parsers)

```go
type LineKind int
const ( LineContext LineKind = iota; LineAdd; LineDel; LineHunk; LineMeta )

type DiffLine struct {
    Kind        LineKind
    OldNo, NewNo int      // 0 when not applicable (e.g. add has no OldNo)
    Text        string    // content without the leading +/-/space
}

type Hunk struct {
    Header string      // the function context after "@@ … @@", e.g. "func (r *Repo) Log(...)"
    Lines  []DiffLine
}

type FileDiff struct {
    Path        string
    Lang        string  // derived from extension, for the syntax lexer
    Adds, Dels  int
    Hunks       []Hunk
}

type Diff struct { Files []FileDiff }

func ParseDiff(raw string) Diff  // pure; tolerant — unknown lines fold into LineContext
```

`Repo.Diff`/`Repo.Show` keep returning the raw string (single responsibility: run git, return output). The command layer (`loadDiff`/`loadShow`) calls `git.ParseDiff` on the result. `git show` and `git diff` share the same hunk grammar, so one parser covers both; `show`'s commit header lines parse as `LineMeta` and are shown muted (or suppressed — §5).

## 5. Rendering (`internal/ui/diffview.go`, new)

`renderDiff(d git.Diff, width int) (lines []string, hunkRows []int)` walks the model and returns the styled lines for the viewport plus the row indices of each hunk band (for `n`/`N`). Replaces `colorizeDiff`, `classifyDiffLine`, and `diffMetaPrefixes` in `panels.go` (removed).

Per-region rules:

- **File header (sticky, outside the viewport).** Rendered by `view.go` above the rule: `path  ▰▰▰▱▱  +12 −3  state·lang`. The change-bar is a fixed-width (~5 cell) proportion of adds/dels. Replaces the bare `mainTitle()` string for the diff case; commit/branch cases keep their current titles.
- **Hunk band.** A full-row line with the accent (cyan `14`) background showing `Hunk.Header`. This is the function context git already emits on the `@@` line.
- **Gutter.** `oldNo`/`newNo` right-aligned, muted (`245`/`238`); blank where a side doesn't apply.
- **Row background.** Add rows get a dim green background, del rows a dim red background, padded to `width` so the band fills the row. Context rows have no background.
- **Word highlight.** Within a *replace run* (consecutive `-` lines immediately followed by `+` lines), pair the i-th del with the i-th add and compute a rune-level LCS; the differing segments get a brighter add/del background. Unpaired add/del lines get the row background only. LCS helper lives in `diffview.go`, pure and unit-tested.
- **Truncation.** Each line is truncated to `width` with a trailing `…` via `ansi.Truncate` (already an indirect dep, `charmbracelet/x/ansi`; fall back to `lipgloss.MaxWidth` + manual ellipsis if the API differs). No wrapping — this is what fixes the wrapped-color bug.
- **Trailing whitespace.** Trailing spaces/tabs on a rendered line are shown as a muted middot run. Tabs elsewhere are expanded to a fixed tab stop before width math.
- **Syntax highlighting (heaviest piece).** `chroma/v2` lexes `Text` by `FileDiff.Lang`; token types map to a small fixed lipgloss foreground palette. Foreground (syntax) and background (diff row / word highlight) are independent ANSI attributes, so they compose: set token fg, then the row/segment bg. Unknown extension → no lexer, plain text. **Implement this last within v1** so the structural rendering can ship and be verified before the composition work.

## 6. Data flow changes

- **`messages.go`** — `diffLoadedMsg{ diff git.Diff }` (was `text string`); add `logLoadedMsg{ text string }` for the branch-log path.
- **`commands.go`** — `loadDiff`/`loadShow` parse via `git.ParseDiff` and send `diffLoadedMsg`. `loadBranchLog` sends `logLoadedMsg`. New: when the selected file is **untracked**, `loadDiff` reads the working-tree file (`os.ReadFile` under `repo.Root()`); a NUL byte in the first few KB → a one-line "Binary file" model; a directory/non-regular file → a one-line note; otherwise synthesize a `FileDiff` whose lines are all `LineAdd` (newNo 1..N).
- **`model.go`** — add `mainDiff *git.Diff` (nil when showing text) and `mainText string`; add `hunkRows []int`. Add `refreshViewport()` that calls `renderDiff` at the current content width and `viewport.SetContent`. 
- **`update.go`** — `diffLoadedMsg` stores the diff and calls `refreshViewport`; `logLoadedMsg` stores text and sets plain content; `WindowSizeMsg` calls `refreshViewport` after `layout()` (re-render at new width). New normal-mode keys `n`/`N` (`keyHunkNext`/`keyHunkPrev` in `keys.go`) jump the viewport `YOffset` to the next/prev entry in `hunkRows`, guarded by `mainDiff != nil`.
- **`view.go`** — `mainContent()` for the diff case renders the sticky file header + rule + `viewport.View()`; the `colorizeDiff` call is removed. A thin scroll indicator (a track with a thumb sized from `YOffset`/total) is joined to the right of the viewport when content overflows.

## 7. Testing strategy

- **Parser (`internal/git`)** — table tests over sample `git diff` and `git show` output: assert file paths, adds/dels counts, hunk headers, and per-line `Kind`/`OldNo`/`NewNo`. Include a rename, a new file, and a multi-file `show`.
- **Word-diff** — assert segment boundaries for representative del/add pairs (substitution, insertion, no-change-on-one-side).
- **Renderer** — strip styles to a plain projection and assert: gutter numbers present and aligned, `…` appears when a line exceeds width, and a full row's `lipgloss.Width == contentWidth` (mirrors the existing selection-bar width test). No ANSI assertions.
- **Hunk nav** — given a known `hunkRows`, assert `n`/`N` move `YOffset` to the expected offsets and clamp at the ends.
- **Untracked** — assert all-add synthesis for a text file and the "Binary file" path for NUL content.
- **Syntax** — assert the plain-text projection is unchanged by highlighting and that an unknown extension falls back to plain.
- Existing `Contains`-based view/panel tests stay green; the three `==` functions (`commandStateText`, `footerActions`, `panelTitle`) are untouched.

## 8. Risks

- **Syntax-color composition.** Layering chroma foreground under diff/word backgrounds with correct ANSI nesting and width accounting is the fiddliest part. Mitigation: ship structural rendering first; add syntax last behind `highlightLine`, falling back to plain on any lexer error.
- **`chroma/v2` weight.** A sizable (if single, well-maintained, pure-Go) dependency. Accepted: it is the largest readability multiplier and was explicitly chosen.
- **Width-dependent re-render.** The diff must re-render on resize; forgetting the `WindowSizeMsg` path would freeze line wrapping/truncation at the old width. Covered by routing both load and resize through `refreshViewport`.
- **`ansi.Truncate` API.** If the exact signature differs from assumption, fall back to `lipgloss.MaxWidth` plus a manual `…`. No behavioral risk, only implementation detail.

## 9. Phasing (smallest viable cut first)

1. **Model + parser** (`git/diff.go`) and wire `diffLoadedMsg`/`logLoadedMsg` — render plain from the model (parity with today, plus correct truncation). Proves the foundation.
2. **Gutter + full-row backgrounds + file header + hunk bands + scroll indicator** — the core visual upgrade, no new deps.
3. **Word-level highlighting** (LCS pairing).
4. **Untracked → all-adds**, **trailing-whitespace**, **hunk nav `n`/`N`** — small independent adds.
5. **Syntax highlighting** (`chroma/v2`) — last, isolated, with plain fallback.

Each phase is independently shippable and testable; a stall at any point still leaves the diff panel better than today.
