# Loom Stash Shelf Design

Date: 2026-06-18
Status: approved design sections, pending user review of written spec
Scope: add a first-class stash workflow to Loom using the existing panel layout and diff viewport.

## Goal

Make Loom more complete as a daily-driver Git TUI by adding an IntelliJ-like stash shelf.

The user should be able to:

- save current work as a named stash;
- browse multiple saved stash versions;
- preview a stash before applying it;
- apply, pop, or drop a stash with clear safety feedback.

This is a first-class workflow, not only a shortcut for the latest stash.

## Current Context

Loom is a keyboard-driven terminal Git client with existing workflows for Files, Branches, and Commits. Recent work already improved commit UX, merge-conflict handling, command logging, status rail context, and changed-file review polish.

The stash workflow should fit that product shape. It should extend the current `internal/git` and `internal/ui` boundaries rather than introduce a separate stash-specific screen architecture.

## Product Shape

Add a fourth top-level workflow panel:

`1 Files | 2 Branches | 3 Commits | 4 Stashes`

When `Stashes` is focused:

- the left panel lists stash entries from newest to oldest;
- the main pane previews the selected stash diff;
- the status rail, when visible, describes the selected stash and available actions;
- the footer shows stash-specific keys.

Footer actions:

`s save | a apply | o pop | d drop | ? help | q quit`

The v1 purpose is multiple named work snapshots that are visible, inspectable, and recoverable.

## Non-Goals

- No partial-file stash.
- No hunk-level stash.
- No stash rename.
- No branch-from-stash.
- No stash search or filtering.
- No new diff renderer.
- No parsed diff model, syntax highlighting, or hunk navigation.
- No custom layout just for stashes.

## Git Layer

Add typed stash support in `internal/git`. The UI must call repo methods and never build raw Git commands itself.

New type:

```go
type Stash struct {
    Ref     string // stash@{0}
    Index   int
    Message string
    Branch  string
    Age     string
}
```

New repo methods:

```go
Stashes(ctx) ([]Stash, error)
StashShow(ctx, ref string) (string, error)
StashPush(ctx, message string) (output string, err error)
StashApply(ctx, ref string) (output string, err error)
StashPop(ctx, ref string) (output string, err error)
StashDrop(ctx, ref string) (output string, err error)
```

Expected Git commands:

- `git stash list --format=%gd%x00%gs%x00%cr`
- `git stash show --patch --stat <ref>`
- `git stash push -u -m <message>`
- `git stash apply <ref>`
- `git stash pop <ref>`
- `git stash drop <ref>`

`stash push` includes untracked files by default through `-u`. This matches the shelf-style workflow where newly created files are commonly part of a saved work version. Ignored files remain out of scope.

The stash list parser should use NUL-delimited fields. `Ref` comes from `%gd`, `Message` from `%gs`, and `Age` from `%cr`. `Branch` is best-effort metadata parsed from standard stash subjects such as `WIP on main: ...` or `On main: ...`; it may be empty for unusual stash messages.

Push, apply, pop, and drop should return Git output so no-change messages, failures, conflicts, and warnings can be preserved in the command log.

## UI Workflow

Add `PanelStashes` as a peer to `PanelFiles`, `PanelBranches`, and `PanelCommits`.

Model addition:

```go
stashes []git.Stash
```

New messages:

```go
stashesLoadedMsg{stashes []git.Stash}
stashShowLoadedMsg{text string, seq int}
```

New commands:

```go
loadStashes
loadStashShow
stashPush
stashApply
stashPop
stashDrop
```

Startup should load status, branches, commits, and stashes. `Tab` cycles through all four panels, and `4` focuses Stashes directly.

Selecting a stash loads `git stash show --patch --stat <ref>` into the existing main viewport. The preview uses Loom's current raw diff rendering path: the output is plain text in the viewport, and existing diff line coloring handles additions, deletions, hunk headers, and metadata.

## Save Flow

Pressing `s` in the Stashes workflow opens a small stash-message input mode.

Required behavior:

- `Ctrl-D` saves the stash with the entered message;
- `Esc` cancels;
- an empty message does not dispatch a stash command;
- on success, refresh Files and Stashes;
- on failure, show a concise error and preserve Git output where available.

The message editor should reuse existing input components where practical, but it should remain a separate mode from commit editing so commit state and stash state do not become coupled. A single-field message editor is enough for v1.

## Apply, Pop, And Drop Flow

Actions on the selected stash:

- `a` applies the stash and keeps it in the stash list;
- `o` pops the stash and removes it if Git succeeds;
- `d` asks for confirmation before dropping the stash.

After save, apply, pop, or drop, Loom should refresh status and stashes. After apply or pop, Loom should also keep merge/conflict state current through the existing status refresh path, so the user can switch back to Files to resolve conflicts.

If apply or pop creates conflicts or returns warnings, Loom should not hide the details. The footer can show a concise failure message, while the full Git output belongs in the command log.

## Layout Constraints

Stashes must follow Loom's existing panel sizing and rendering patterns:

- reuse the current left list pane width calculation;
- reuse the existing main viewport sizing;
- reuse the status rail only when the existing wide-terminal threshold shows it;
- reuse `renderPanel`, `panelRows`, `mainContent`, `mainTitle`, `footerHints`, and current scroll behavior where possible;
- keep stash rows compact, similar in density to branch and commit rows;
- keep stash preview in the same diff viewport used by file diffs and commit details.

The Stashes workflow should feel like a normal Loom panel, not a custom screen.

## Error Handling

The stash workflow should use existing error and command-log conventions:

- Git-layer methods wrap stderr with command context.
- UI failures set `m.err` so the footer or status rail can surface them.
- Commands with useful output append that output to `cmdLog`.
- `drop` is destructive and must use confirmation.
- `apply` and `pop` are allowed to leave the working tree conflicted; Loom should refresh status and let the existing Files conflict workflow take over.

## Testing

Add focused tests around the new Git parser, command dispatch, reducer state, and rendering helpers:

- parse `git stash list` output into typed `git.Stash` values;
- stash repo methods dispatch the expected Git args;
- startup loads stashes along with status, branches, and commits;
- `4` focuses Stashes;
- `Tab` cycles through four panels;
- selected stash loads a preview into the current main viewport;
- `s`, `a`, `o`, and `d` only act in valid contexts;
- drop asks for confirmation;
- save, apply, pop, and drop refresh status and stash list;
- apply and pop failures preserve output in command log and show a concise error;
- Stashes reuses normal panel layout helpers instead of a custom layout path.

Prefer pure parser and reducer tests where possible. Avoid brittle full-screen ANSI snapshots.

## Acceptance Criteria

- Stashes appears as a fourth top-level workflow.
- The stash list shows multiple named saved versions from newest to oldest.
- Selecting a stash previews it with the existing diff viewport and line coloring.
- The user can save current work with a message, including untracked files.
- The user can apply, pop, and drop selected stashes.
- Drop requires confirmation.
- Apply/pop conflicts surface clearly and leave the user in a resolvable Files state.
- Existing Files, Branches, Commits, commit, remote, and conflict workflows continue to behave as before.
- Existing tests pass, with new tests covering stash parsing, command dispatch, navigation, actions, and failure handling.
