# Loom Cherry-Pick Design

Date: 2026-06-19
Status: approved concept, pending user review of written spec
Scope: add multi-select cherry-pick to Loom's existing Commits workflow.

## Goal

Make the Commits workflow actionable after investigation. The user should be able to mark one
or more visible commits and cherry-pick them onto the current branch without leaving Loom for
the happy path.

The user should be able to:

- mark commits in the Commits list;
- review the selected count before mutating repository state;
- confirm cherry-pick explicitly;
- apply selected commits in the same order they appear in the current Commits list;
- see success, dirty-worktree, empty-selection, and conflict-stopped outcomes clearly.

## Current Context

Loom is a keyboard-driven terminal Git client with top-level workflows for Files, Branches,
Commits, and Stashes. The Commits workflow can browse recent commits and search read-only
commit results. The main pane previews the selected commit through the existing `git show`
path.

Cherry-pick belongs in the Commits workflow because the selected objects are commits. It should
reuse the existing list, confirmation, command-log, and refresh patterns instead of introducing
a new top-level workflow.

## Product Shape

Press `space` while focused on Commits to mark or unmark the highlighted commit for
cherry-pick. Marked commits are visually highlighted in the Commits list, using the same visual
language as selected or emphasized file-change rows where possible. A lightweight ASCII marker
such as `*` may be added only if row highlighting alone is ambiguous in terminal themes.

Press `y` while focused on Commits to start cherry-pick confirmation. `y` requires at least one
marked commit. If no commits are marked, Loom does not enter confirmation and shows
`No commits selected`.

When commits are marked, `y` opens a confirmation prompt:

```text
Cherry-pick 3 commits in list order? [y/n]
```

The second `y`, inside confirmation mode, runs the actual Git command. The selected commits are
applied in the same order they appear in the currently visible Commits list. Loom does not
provide reorder controls in this pass.

Selections clear whenever the commit list is replaced or reloaded, including normal commit
loads, commit search application, commit search clearing, branch log reloads, and post-mutation
status refreshes.

## Keyboard Behavior

| Key | Action |
|---|---|
| `space` | mark or unmark the highlighted commit while focused on Commits |
| `y` | open cherry-pick confirmation when at least one commit is marked |
| `y` in confirmation | run cherry-pick |
| `n` / `Esc` in confirmation | cancel without mutating repository state |

The existing `space` behavior for Files remains unchanged. The key is contextual: Files uses it
for stage or unstage, and Commits uses it for commit marking.

## Clean-Tree Gate

Cherry-pick requires a clean worktree and clean index before it starts. If there are current
file changes, pressing `y` while commits are marked should show:

```text
Commit or stash current changes before cherry-pick
```

Loom should not auto-stash, auto-stage, or attempt to cherry-pick over dirty local work in this
version.

## Git Layer

Add a typed cherry-pick method to `internal/git`:

```go
func (r *Repo) CherryPick(ctx context.Context, hashes []string) (string, error)
```

The method should:

- reject an empty hash slice before invoking Git;
- run one command: `git cherry-pick <hash1> <hash2> ...`;
- preserve the hash order supplied by the UI;
- return combined stdout and stderr, matching the style of stash and remote commands;
- return an error that identifies `git cherry-pick` when Git exits non-zero.

The UI must not build raw Git command strings itself. It should call the typed repo method and
use a command wrapper that returns `gitDoneMsg`.

## UI State And Data Flow

Store selected commits by hash, not by list index. Indexes can become stale when the commit
list is searched, cleared, or refreshed. Rendering can then check whether each row's hash is
selected.

When the user confirms cherry-pick, the UI builds the ordered hash slice by walking
`m.commits` in visible list order and keeping hashes present in the selected set. This makes
the execution order obvious and testable.

On successful cherry-pick:

- clear selected commit hashes;
- show a notice such as `Cherry-picked 3 commits`;
- refresh status and the relevant lists through the existing `gitDoneMsg` refresh path;
- keep Git output available in the command log.

On failed cherry-pick:

- clear the busy state;
- refresh status so conflicted files or other resulting state becomes visible;
- keep Git output available in the command log;
- show a clear notice or error such as `Cherry-pick stopped; resolve conflicts outside Loom or inspect Files`.

## Conflict Behavior

If Git stops with conflicts, Loom should surface the state but not manage the rest of the
cherry-pick sequence in v1.

The expected behavior is:

- Git leaves the repository in its standard cherry-pick conflict state;
- Loom refreshes status;
- unmerged files appear in Files through existing status parsing and conflict rendering;
- the command log contains Git's conflict output;
- no `cherry-pick --continue`, `--abort`, or `--skip` controls are added in this pass.

This avoids reusing merge-specific labels or controls for cherry-pick, where the semantics and
next steps differ.

## Rendering And Feedback

The Commits panel should make marked commits persistent and visible even when the cursor moves.
Marked rows should use the same highlight style family as selected or emphasized file-change
rows. If terminal rendering makes the distinction unclear, include an ASCII marker like `*` in
the commit row.

The Commits footer should include:

```text
space mark · y cherry-pick
```

The selected context or status rail should show the selected count:

```text
Cherry-pick: 3 selected
```

The help overlay should document the contextual `space` behavior and `y` cherry-pick action.

## Error And Notice Copy

Use short, direct copy:

- Empty selection: `No commits selected`
- Dirty worktree or index: `Commit or stash current changes before cherry-pick`
- Confirmation: `Cherry-pick N commits in list order? [y/n]`
- Success: `Cherry-picked N commits`
- Conflict or other Git failure: `Cherry-pick stopped; resolve conflicts outside Loom or inspect Files`

The command log remains the place for full Git output.

## Tests

Add coverage at the same layers as existing Loom features:

- Git tests: `Repo.CherryPick` builds `git cherry-pick <hashes...>` in exact order, rejects
  empty input, and returns combined output on success and failure.
- UI command tests: the cherry-pick command wrapper returns `gitDoneMsg` with command label,
  output, notice, and error.
- Reducer tests: `space` toggles commit selection only in Commits; `y` with no selection
  reports `No commits selected`; `y` with dirty files reports the clean-tree gate;
  `y` with selected commits enters confirmation; confirmation runs hashes in visible list order.
- Reload tests: selected commits clear when commits are reloaded or replaced.
- View tests: marked commit rows use persistent selected styling; footer, help, selected
  context, and status rail mention marking and cherry-pick.
- Conflict-path test: failed cherry-pick still refreshes status so unmerged files surface.

## Non-Goals

- No single-commit fallback when nothing is selected.
- No range selection.
- No reorder UI.
- No `cherry-pick --continue`, `--abort`, or `--skip`.
- No automatic stash.
- No cherry-picking with a dirty worktree or dirty index.
- No command-language parser.
- No changes to commit search beyond allowing marked visible results to be cherry-picked.
