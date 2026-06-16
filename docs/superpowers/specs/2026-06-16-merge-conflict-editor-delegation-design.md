# loom Merge-Conflict Handling (Editor Delegation) Design

**Date:** 2026-06-16
**Status:** Approved for planning
**Scope:** Stop stranding the user when a merge conflicts. Surface the conflicted state, hand each conflicted file to the user's editor, and let them finish or abort the merge — all by wrapping real git. No in-app conflict editing.

## Context

Conflicts can already arise in loom today: `p` (pull) runs `git pull`, which merges, and a `switch` can conflict too. When that happens loom shows the files in red (`panels.go` `markerColor`, `view.go:213` `"unmerged file"`) but offers **no way forward** — the user must quit to the CLI. This is a dead end.

The status parser already detects unmerged files (`status.go:70` sets `FileStatus.Unmerged`), but it throws away the `u`-line's two-character conflict code (`UU`, `AA`, `DU`, …), so loom can't tell "both modified" from "deleted by them."

This pass closes the loop with the smallest coherent slice, consistent with loom's founding principle: **loom wraps the real `git` binary; it does not reimplement git.** Actual conflict resolution is delegated to the user's editor (the same way loom already inherits the user's credentials, hooks, and config). loom's only jobs are to make the state obvious, launch the editor, and complete or abort the merge.

## Core Principle

**Delegate the judgement, wrap the mechanics.** Choosing how to merge two changes is human work that an editor (VS Code's merge editor, vim's fugitive, etc.) already does well. loom does not parse conflict regions or write merged files. It wraps the git mechanics around that edit: detect the merge, open the file, mark resolved (`git add`), finish (`git commit`), or abort (`git merge --abort`).

## Behavior

### Entering a conflicted state

Conflicts arrive through existing operations (`pull`, `switch`). loom does **not** add any command that *starts* a merge or rebase in this pass.

loom is "in a merge" when `MERGE_HEAD` exists. This is detected with `git rev-parse --verify --quiet MERGE_HEAD` (exit 0 = merging), folded into the normal status refresh. This is tracked separately from unmerged-file count because the merge is still in progress — and still abortable — after every file is resolved but before the merge commit lands.

### The conflict loop

While merging, the user can:

| Key | Action | git |
|---|---|---|
| `e` | open the selected conflicted file in the editor | (suspend TUI, run editor) |
| `space` | mark the selected file resolved | `git add -- <path>` (existing `Stage`) |
| `c` | finish the merge | existing commit flow (`git commit`) |
| `A` | abort the whole merge | `git merge --abort` (confirm gate) |

Typical flow: `p` pulls and conflicts → loom shows `MERGING — N conflicts` → user selects a file, presses `e`, resolves it in VS Code's merge editor, saves and closes → loom refreshes (the file is still unmerged until staged) → `space` marks it resolved → repeat → `c` writes the merge commit. At any point before the commit, `A` aborts.

`e` is a no-op unless the selected Files-panel row is an unmerged file. `A` is only offered while merging.

### Editor resolution and launch

loom opens the file in the editor **git itself would use**, with one changed default. Precedence:

1. `$GIT_EDITOR`
2. `git config core.editor`
3. `$VISUAL`
4. `$EDITOR`
5. **loom default: `code --wait`** (instead of git's `vi` fallback)

The resolved command is run through the platform shell with the conflicted file path appended, mirroring how git launches `core.editor`. The TUI is suspended for the duration via Bubble Tea's `tea.ExecProcess`, which releases the real terminal to the editor and delivers a completion message on exit; loom then refreshes status.

Two rules make the default robust:

- **`--wait` is appended only to loom's built-in VS Code default.** A user-configured editor is launched verbatim — the user owns its blocking flags, exactly as git requires. Without `--wait`, the `code` launcher returns immediately and loom would resume before the edit is finished.
- The editor command is **resolved through a pure function** (given the four candidate strings, return the chosen command), so the precedence is unit-testable without touching the real environment.

### State cue

While merging, loom shows a top-bar cue alongside the existing command-state slot: `MERGING — N conflicts` while N unmerged files remain, and `MERGING — ready to commit` once every file is resolved but the merge commit has not yet landed (this is why merge state is tracked separately from the unmerged count). When focus is on the Files panel during a merge, the footer shows the conflict actions (`e edit · space resolve · A abort · c commit`) in place of the normal Files hints. The selected-context rail line for an unmerged file shows its conflict kind (see below).

### Conflict-kind labels

The two-char `u`-line code maps to a human label, used in the selected-context rail line:

| Code | Label |
|---|---|
| `UU` | both modified |
| `AA` | both added |
| `DD` | both deleted |
| `AU` | added by us |
| `UD` | deleted by them |
| `UA` | added by them |
| `DU` | deleted by us |

The mapping is presentation text and lives in the UI layer (git layer stays free of UI strings). All seven cases still resolve through the same edit/stage/commit/abort loop; the label is informational.

## Component Boundaries

| File | Change |
|---|---|
| `internal/git/status.go` | `FileStatus` gains `Conflict string` (the raw `u`-line code, e.g. `"UU"`; empty when not a conflict). Set it at the `u`-line parse alongside the existing `Unmerged` bool. |
| `internal/git/repo.go` | `MergeAbort(ctx)` → `git merge --abort`. `Merging(ctx) (bool, error)` → `git rev-parse --verify --quiet MERGE_HEAD`. `EditorArgv(ctx) ([]string, error)` resolving the editor (config via `git config --get core.editor`, env via `os`, VS Code fallback) — backed by a pure chooser helper. |
| `internal/ui/commands.go` | `openEditor(...)` returns a `tea.ExecProcess` Cmd that runs the resolved editor on the file and yields `editorDoneMsg`. `mergeAbort(...)` mutation Cmd. Status refresh extended to also carry the merging flag. |
| `internal/ui/messages.go` | `editorDoneMsg{err error}`; merging flag added to the status-loaded message. |
| `internal/ui/model.go` | `merging bool` presentation state. |
| `internal/ui/update.go` | `e` dispatches `openEditor` for an unmerged Files row; `A` enters the existing `ModeConfirming` gate with a `mergeAbort` action; `editorDoneMsg` refreshes status; store the merging flag from the status message. |
| `internal/ui/view.go` | `MERGING — N conflicts` top-bar cue; conflict footer hints while merging; conflict-kind label in the selected-context line; help-overlay rows for `e` / `A`. |
| `internal/ui/panels.go` | `conflictLabel(code string) string` mapping. |
| `README.md` | Keys table rows for `e` (open conflict in editor) and `A` (abort merge). |

The two-layer architecture and the `Runner` seam are unchanged.

## Error Handling

- **Editor not found or non-zero exit** → `editorDoneMsg.err` → footer error, status refreshed anyway so the user can retry or pick another file. Never crashes the loop.
- **`git merge --abort` failure** → flows through the existing `gitDoneMsg.err` → footer, like every other mutation.
- **Not actually merging** when `A` or `e` somehow fire → no-op (guarded on `merging` / unmerged selection).
- All git stderr stays wrapped with context by the git layer, as today.

## Testing

Behavior-level, matching the repo's pure-function + reducer style. No subprocess in tests.

- **Parser:** extend the existing `parseStatus` test so the unmerged fixture line asserts `Conflict == "UU"`.
- **Repo:** `MergeAbort` issues `["merge","--abort"]`; `Merging` issues `["rev-parse","--verify","--quiet","MERGE_HEAD"]` and maps exit 0/non-zero to `true`/`false`.
- **Editor chooser (pure):** table over the four candidate strings → expected command; all-empty → `code --wait`; `--wait` appended only to the built-in default.
- **Conflict labels (pure):** each code → expected label.
- **Reducer:** `e` on an unmerged Files row returns a non-nil Cmd; `e` on a clean/non-Files selection is a no-op; `A` while merging enters `ModeConfirming` with an abort action stored; `editorDoneMsg` triggers a status refresh; the merging flag from the status message lands on the model.
- **View:** `MERGING — N conflicts` appears when merging; conflict footer hints render while merging on the Files panel; the conflict-kind label appears in the selected-context line.

The `tea.ExecProcess` launch itself is the one integration seam — tests assert the dispatch decision (Cmd produced, right message on completion), not the spawned process.

## Non-Goals

- No in-app conflict-region editing (keep-ours / keep-theirs / per-hunk picking). That is the editor's job, and it overlaps with the deferred hunk-level staging feature.
- No 3-way merge view rendered inside loom.
- No command to *start* a merge, rebase, or cherry-pick from loom.
- No rebase / cherry-pick conflict support — `ours`/`theirs` semantics invert under rebase, so labels and actions can't be shared. Merge-only this pass.
- No `git mergetool` integration (possible later add; `e` covers the need).
- No loom-specific editor configuration — override the VS Code default via standard git config / env, the same knobs git reads.
