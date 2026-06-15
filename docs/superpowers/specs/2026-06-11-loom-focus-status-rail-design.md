# loom Focus Mode Status Rail Design

**Date:** 2026-06-11
**Status:** Approved for planning
**Scope:** Replace the stacked panel presentation with Focus Mode plus a read-only status rail.

## Goal

Make loom feel more like a confident daily git client by giving the active workflow more space while keeping command feedback and cross-workflow orientation visible.

This builds on the existing UI polish pass. The earlier pass improved the current stacked layout with panel counts, main pane headings, contextual footer copy, and clearer state messages. This pass changes the layout model: Files, Branches, and Commits become focused workflows instead of three always-visible stacked panels.

## Chosen Direction

Use **Focus Mode plus Status Rail**:

- a top bar shows the current branch, ahead/behind counts, active workflow, and workflow counts;
- the body shows the active workflow list on the left, preview content in the center, and a read-only status rail on the right;
- the footer stays compact and only shows immediate key hints;
- command feedback moves out of the footer and into the status rail.

The status rail is informational. It must not become a second command palette or an alternate command execution surface.

## Layout

### Top Bar

The top bar provides global orientation:

```text
main ↑2 ↓0 | [1 Files 3] 2 Branches 4 3 Commits 50 | Ready
```

It should show:

- current branch name, with `(no branch)` fallback;
- ahead and behind counts;
- workflow tabs and counts for Files, Branches, and Commits;
- a short command state such as `Ready`, `Fetching...`, `Pushing...`, or `Error`.

### Body

The body has three regions:

1. Active workflow list.
2. Preview pane.
3. Status rail.

The active workflow list changes with focus:

- Files: changed files, with existing portable ASCII markers.
- Branches: local branches, with the current branch marked.
- Commits: recent commit list.

The preview pane keeps the current behavior:

- Files focus shows the selected file diff.
- Branches focus shows the selected branch log.
- Commits focus shows the selected commit detail.

The preview pane should remain the largest reading area.

### Status Rail

The rail has stable sections:

```text
Status Rail

Workflow
Files: 3 changed
Branches: 4 local
Commits: 50 loaded

Command
Ready
Last: git fetch OK

Recent
10:21 git add README.md
10:20 git fetch

Selected
unstaged file
actions: stage, discard
```

Section content can be compact, but the order should remain stable so the user can build muscle memory.

## Behavior

Navigation remains familiar:

- `1`, `2`, `3` jump to Files, Branches, or Commits.
- `Tab` cycles workflows.
- `j` / `k` and arrow keys move selection.
- Existing command keys keep their current behavior.

The major behavior change is presentation: only the active workflow list is shown in full. The other workflows remain visible as counts in the top bar and rail.

The rail should derive most content from existing model state:

- workflow counts from `files`, `branches`, and `commits`;
- active workflow from `focus`;
- selected context from `focus` and `cursor`;
- last command and recent command list from `cmdLog`;
- errors from `err`;
- busy state from `busy`.

A small presentation field may be added if implementation needs clearer command labels, for example `fetching...`, `pushing...`, or `switching branch...`. Do not add broad command lifecycle machinery for this pass.

## Command Feedback

The command section should answer:

- Is loom ready or currently running a command?
- What was the last command?
- Did the last command fail?
- What are the most recent commands?

Errors should remain visible in the rail until replaced by a successful command or a newer error. On narrow terminals, the footer may still show a short error summary so failures are not hidden.

Remote command output can stay summarized for this pass. A full output viewer is out of scope.

## Responsive Fallback

Focus Mode must behave reasonably on smaller terminals.

Fallback priority:

1. Keep the active workflow list visible.
2. Keep the preview pane usable.
3. Collapse or hide the status rail.
4. Keep the footer within the terminal height.

Acceptable fallback options:

- hide the rail below a width threshold and keep command state in the top bar/footer;
- compress the rail to command state plus workflow counts;
- add a simple rail toggle only if needed for usability.

Do not make small-screen support depend on mouse interaction.

## Component Boundaries

Keep the work inside `internal/ui`:

- `view.go`: top-level Focus Mode layout, top bar, footer, and responsive fallback.
- `panels.go`: active workflow list rendering, selected row rendering, shared styles, and rail helper rendering if that matches the local structure.
- `model.go`: only add minimal presentation state if required for command labels or a rail toggle.
- `update.go`: preserve existing key behavior; only change if minimal presentation state must be maintained.

The `internal/git` package should not change for this pass.

## Testing

Add focused tests around deterministic rendering and state-derived helpers:

- workflow tabs and counts render correctly;
- active workflow list renders for Files, Branches, and Commits;
- the old stacked panel competition is removed from the main Focus Mode body;
- status rail renders workflow summary, ready state, busy state, error state, and recent commands;
- selected context changes for Files, Branches, and Commits;
- narrow-width fallback does not exceed terminal height and keeps footer content usable;
- existing update tests continue passing.

Avoid brittle full-screen golden snapshots unless a small smoke snapshot proves useful. Prefer testing helper output and key strings.

## Non-Goals

- No new git commands.
- No hunk or line staging.
- No command palette.
- No mouse support.
- No theming or config system.
- No rich commit graph.
- No full command output viewer.
- No git-layer parser or runner changes.
