# Loom Review Flow Polish Design

Date: 2026-06-17
Status: approved design sections, pending user review of written spec
Scope: polish existing Files review workflow in `internal/ui` without adding Git capabilities.

## Goal

Make Loom's existing changed-file review loop easier to scan and safer to act on.

The user should be able to answer three questions quickly:

- What kind of file am I looking at: conflicted, staged, unstaged, or untracked?
- What will the next key press do to this selected file?
- Where am I in the set of changed files?

This is a polish pass over existing state and behavior. It does not add hunk staging, mouse support, new Git operations, a parsed diff renderer, or a theme system.

## Current Context

Loom is a keyboard-driven terminal Git client. The current UI has:

- a top bar with branch, workflow tabs, command state, and merge status;
- a left list pane that switches between Files, Branches, and Commits;
- a main pane that shows a diff, branch log, commit detail, or commit editor;
- an optional status rail on wide terminals;
- a contextual footer with key hints.

Recent work already improved commit UX and merge-conflict recovery. This pass focuses on the everyday Files review flow and avoids overlapping with the larger rich diff-panel design.

## Design

### 1. Review Flow

The Files panel becomes a clearer review surface while keeping the same interaction model. It still selects one real file at a time. The selected file remains the target for staging, unstaging, discard confirmation, conflict edit, conflict resolve, and commit actions.

The UI should present changed files as a review queue, not as one flat mixed list. The main pane and status rail then explain the selected item in that queue.

### 2. File List Grouping

Render Files rows in visual groups using fields already present on `git.FileStatus`:

1. `Conflicts`
2. `Staged`
3. `Unstaged`
4. `Untracked`

Group headers are display-only rows. The cursor must skip them, and file actions must continue to target the underlying `m.files` entry for the selected real file.

Within each group, preserve the relative order from `git status` as much as practical. The UI should not feel like it is reordering files unpredictably beyond the useful state grouping.

Header labels should be short enough for narrow terminals:

- `Conflicts`
- `Staged`
- `Unstaged`
- `Untracked`

Existing file status markers still appear on file rows. Grouping is presentation-only and should not require a git-layer change.

### 3. Action Clarity

The footer and status rail should describe the selected file's real next action instead of showing only generic Files-panel shortcuts.

Example footer states:

- staged file: `space unstage | c commit staged`
- unstaged file: `space stage | d discard | c commit all`
- untracked file: `space stage | d discard`
- conflict: `e edit | space resolve | A abort | c commit`

The footer remains the compact control hint. The status rail remains contextual detail. In the rail, the `Selected` section should include:

- selected path;
- selected state;
- one or two concrete actions.

This does not change what keys do. It only makes existing behavior easier to predict.

### 4. Main Pane Polish

For file selections, the main title should be compact and state-aware:

`internal/ui/view.go | staged | 2 of 7`

The `2 of 7` position counts real file rows, not group headers. It helps users understand where they are in the review set without adding new navigation commands.

The existing scroll cue stays, but the copy can be clearer when content overflows:

- `diff 42%`
- `more below`

This pass should not introduce new git calls only to compute diff stats. If stats become available from a later parsed-diff model, they can be added then.

Empty and loading copy should be specific:

- no changed files: `Working tree clean`
- selected file has no diff: `No diff for this file`
- diff content is being refreshed after selection changes: `Loading diff...`

The main pane remains a single viewport. Rich diff parsing, syntax highlighting, hunk bands, word highlights, and hunk navigation remain in the separate diff-panel design.

### 5. State And Component Boundaries

Keep the implementation in `internal/ui` unless tests show a small helper belongs elsewhere. The git layer should not change.

The likely internal shape is a small visible-row helper for the Files panel:

- convert `m.files` into a rendered row list containing group headers and file rows;
- map selectable visible rows back to `m.files` indexes;
- keep cursor movement and file actions using real file indexes.

This helper should make grouping testable without spreading header-skipping logic through unrelated update paths.

Expected files:

- `internal/ui/panels.go`: grouped Files row rendering and row metadata helper;
- `internal/ui/update.go`: cursor movement over selectable file rows if grouping changes the row model;
- `internal/ui/view.go`: selected context, footer hints, main title, empty/loading copy;
- `internal/ui/model.go`: only if a small extra field is needed for loading copy or row mapping;
- `internal/ui/*_test.go`: focused tests for grouping, selection mapping, footer hints, and titles.

### 6. Error Handling

This pass does not introduce new command failure modes. Existing git errors keep flowing through current `err` and `gitDoneMsg` handling.

The design should avoid hiding errors behind polish:

- footer errors stay visible on narrow terminals;
- status rail errors stay visible on wide terminals;
- command log access stays discoverable when remote operations fail with output.

Grouped display must degrade safely. If there are no files in a group, omit that group header. If there are no changed files at all, show the current clean-tree empty state.

### 7. Testing

Add focused tests around the presentation and reducer boundaries:

- grouped Files rows render headers in `Conflicts`, `Staged`, `Unstaged`, `Untracked` order;
- empty groups are omitted;
- cursor movement skips group headers;
- selected visible rows map back to the correct `m.files` entry;
- `space`, `d`, `e`, and `c` still target the selected file semantics correctly;
- footer hints differ for staged, unstaged, untracked, and conflict selections;
- main title includes selected file state and review position;
- clean, no-diff, and loading copy render as intended.

Prefer pure helper tests where possible. Avoid brittle full-screen ANSI snapshots.

## Non-Goals

- No hunk or line staging.
- No parsed diff renderer.
- No syntax highlighting.
- No new git commands or git-layer data calls.
- No mouse support.
- No command palette.
- No theme or configuration system.
- No branch/commit workflow changes outside copy that must stay consistent with the Files polish.

## Acceptance Criteria

- Files are grouped by review state without changing Git behavior.
- The cursor never lands on a group header.
- Existing file actions still operate on the intended file.
- Footer and rail wording make the selected file's next action clear.
- The main pane title shows selected file state and review position.
- Empty/loading text is specific and calm.
- Existing tests pass, with new tests covering grouping and action clarity.
