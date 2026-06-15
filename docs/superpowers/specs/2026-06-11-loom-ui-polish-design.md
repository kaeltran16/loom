# loom UI Polish Design

**Date:** 2026-06-11
**Status:** Approved for planning
**Scope:** Calm readable visual polish plus clarity copy for the existing TUI layout.

## Goal

Improve loom's terminal UI so it feels calmer, more readable, and easier to orient in during daily git work. This pass keeps the current structure: stacked Files, Branches, and Commits panels on the left, one main pane on the right, and a footer at the bottom.

This is not a layout rethink and does not add new git capabilities.

## Direction

Use a **Calm Readable** treatment:

- softer borders and less aggressive focus contrast;
- consistent panel padding and spacing;
- muted secondary text for metadata and empty states;
- restrained color use for git meaning: additions, deletions, warnings, errors, and focus;
- fewer always-visible shortcut hints.

The focused panel must remain obvious, but unfocused panels should recede instead of competing with the main pane.

## UI Changes

### Panel Titles And Counts

Each panel title includes a count:

- `Files 3`
- `Branches 2`
- `Commits 50`

When a panel has no items, it renders intentional empty copy instead of a blank body:

- Files: `No changes`
- Branches: `No local branches`
- Commits: `No commits`

Panel rows keep the current simple ASCII markers for portability. The polish pass can align markers and paths more consistently, but it should not introduce icon dependencies.

### Main Pane Heading

The main pane gets a heading derived from focus and selection:

- Files focus with selected file: `Diff: internal/ui/view.go (unstaged)` or `(staged)`
- Branches focus: `Branch log: main`
- Commits focus: `Commit: 37527ee`
- No selected content: a short empty-state heading such as `Working tree clean`

The heading is presentation state, computed from the current model. It should not require a new git call.

### Contextual Footer

The footer becomes two logical parts:

1. Repo status: current branch, ahead/behind counts, busy/error/success state.
2. Contextual actions: shortcuts relevant to the focused panel and current mode.

Examples:

- Files focus: `Files: space stage · d discard · c commit · ? help · q quit`
- Branches focus: `Branches: enter switch · c commit · f fetch · p pull · P push · ? help · q quit`
- Commits focus: `Commits: c commit · f fetch · p pull · P push · ? help · q quit`
- Confirming mode: `Confirm: y yes · n no · esc cancel`
- Committing mode: `Commit: ctrl+d submit · esc cancel`

Global actions remain visible, but the footer should no longer list every shortcut on every screen.

### State Messages

The UI should render clear state messages for common non-happy paths:

- Initial size/loading state: `Loading repository...`
- Busy state: spinner plus a short action label when available, otherwise `working...`
- Clean tree: `Working tree clean`
- Empty branch or commit data: panel-specific empty text
- Git error: `error: <message>` in the footer
- Last successful command: short footer text based on `cmdLog`, such as `last: git fetch`

This is intentionally not a notification framework. It reuses existing model state (`busy`, `err`, `cmdLog`, loaded lists) unless implementation shows a tiny field is needed for a clearer busy label.

## Component Boundaries

Keep the work inside `internal/ui`:

- `view.go`: assemble page layout, main pane heading, footer text, and top-level state messages.
- `panels.go`: panel title/count rendering, row formatting, empty panel copy, and shared styles.
- `model.go`: only add a small field if needed to distinguish busy action labels or loaded empty state.
- `update.go`: only change if a small model field must be maintained after command dispatch/results.

The git layer should not change for this pass.

## Error Handling

Existing git errors continue flowing through `errMsg` and `gitDoneMsg`. The polish pass only changes how errors are displayed:

- keep the raw useful git message visible;
- avoid stack-like or noisy wrapping in the UI;
- preserve non-crashing behavior.

Confirmed destructive actions remain unchanged except for clearer prompt/footer copy.

## Testing

Add focused tests around pure formatting/rendering helpers rather than terminal snapshots:

- panel title/count formatting;
- empty panel line rendering;
- main pane heading for Files, Branches, Commits, and empty selections;
- contextual footer text by focus and mode;
- error and last-command footer text.

Existing `Update` tests should remain valid. Add reducer tests only if implementation introduces new model state.

## Non-Goals

- No hunk or line staging.
- No new top-level layout.
- No command palette.
- No mouse support.
- No theming/config system.
- No new git commands.
- No persistent notification center.
