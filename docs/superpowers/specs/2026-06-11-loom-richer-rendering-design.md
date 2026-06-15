# loom Richer Diff & List Rendering Design

**Date:** 2026-06-11
**Status:** Approved for implementation
**Scope:** Visual polish of the diff/preview body and the workflow lists, using only `lipgloss` over data loom already has. No new dependencies; `internal/git` untouched.

## Context

v1 is feature-complete. Today the diff renderer (`colorizeDiff`) only colors `+`/`-` lines — hunk headers and file metadata are flat white. List rows show plain markers, and the selected row is marked by a subtle background (`238`) that is easy to lose. `Commit.Author` and `Commit.RelTime` are parsed but never displayed.

This pass adds four independent rendering enhancements. It is presentation-only.

## Core Principle

**Keep the text source pure; style in the styling layer.** `panelLines`/`fileLine` continue to return plain text — they are the single source for width math and are asserted verbatim by existing tests. All color is applied in the rendering layer (`styledPanelLines`, `colorizeDiff`).

This also resolves the one real hazard: **ANSI nesting.** Coloring a marker and then wrapping the row in a selection background would let the inner color's reset code punch a hole in the selection background mid-row. The rule that avoids it: **marker color applies only to non-selected rows; the selected row gets the caret + highlight instead.** Selection and state-color never overlap.

## Enhancements

### 1. Diff body styling

A pure classifier replaces the ad-hoc switch in `colorizeDiff`:

```
classifyDiffLine(line string) diffKind   // Add | Del | Hunk | Meta | Context
```

- `Hunk` (`@@` prefix) → cyan (`hunkStyle`).
- `Meta` (`diff --git`, `index`, `+++`, `---`, and `git show` headers `commit`/`Author:`/`Date:`/`Merge:`) → muted (`mutedStyle`).
- `Add` (`+`, not `+++`) / `Del` (`-`, not `---`) → existing green/red.
- `Context` → unchanged.

`colorizeDiff` becomes a thin loop: classify each line, apply the mapped style. The `git show` commit header is deliberately folded into the muted `Meta` kind for simplicity (a distinct style can be added later if it reads too dim).

### 2. File-list marker colors

A pure mapping colors the status marker by state:

```
markerColor(FileStatus) lipgloss.Color
```

staged → green · modified (worktree) → yellow · untracked → dim · unmerged → red · deleted → red.

Applied to the marker glyph in `styledPanelLines` for **non-selected** Files rows (it reads `m.files[i]` for state). Path text stays neutral.

### 3. Commit metadata (time / author)

`selectedContextLines` (commits case) appends an `author · relTime` line built from `m.commits[i]`, only when those fields are non-empty (so existing empty-field tests stay green). The narrow commit list stays compact; the preview header's author/date already come from `git show` output and are styled by enhancement 1.

### 4. Clearer selected row

`styledPanelLines` prefixes every row with a 2-column gutter: a caret (`▌`, a named constant) when selected, spaces otherwise — keeping columns aligned. `cursorStyle` gets a more visible background. Applies to all three panels.

## Component Boundaries

| File | Change |
|------|--------|
| `panels.go` | `classifyDiffLine` + `diffKind`; `colorizeDiff` thin loop; `markerColor`; gutter/caret + marker color in `styledPanelLines`; `hunkStyle`, brighter `cursorStyle`, caret constant |
| `view.go` | `selectedContextLines` commits case adds author/relTime |
| `panels_test.go`, `view_test.go` | new tests below |
| `internal/git` | untouched |

## Testing

Behavior-level, table-driven, matching the repo's pure-function style. No brittle ANSI-byte assertions — test the classifier and the color mappings, not the escape codes.

- `classifyDiffLine`: each representative prefix → expected kind.
- `markerColor`: each `FileStatus` state → expected color.
- `selectedContextLines`: commit case with author/relTime populated surfaces both.
- selected-row rendering: caret on the selected row, spaced gutter (no caret) on others, alignment preserved.
- Existing tests remain green (`panelLines`/`fileLine` unchanged).

## Non-Goals

- No `internal/git` changes, no new git commands.
- No new dependencies (no syntax highlighting / `chroma`).
- No per-branch ahead/behind glyphs (data not available without new git calls).
- No new keybindings, no layout/responsive changes, no theming/config system.
