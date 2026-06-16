# Commit message UX — design

Date: 2026-06-16
Status: approved (design); pending implementation plan

## Goal

Replace loom's single free-form commit textarea with a structured commit editor:
a one-line subject plus a multiline body, a warn-only 50/72 subject-length
counter, a muted conventional-commit hint, `git commit --amend` with a
push-safety confirm, and a transient success notice after a commit lands.

This is the first of two planned UI/UX cycles. The second (a batch of seven
quick-win fixes) is out of scope here.

## Scope

In scope:

- Two-field commit editor: subject (`textinput`) + body (`textarea`), `Tab` toggles focus.
- Subject-length counter `NN/50`, warn-only: green ≤50, yellow ≤72, red >72.
- Muted conventional-commit hint near the subject field (no pre-fill, no enforcement).
- `--amend`: triggered by `C`, pre-filled with HEAD's message; a y/n confirm when
  the commit is already pushed.
- Transient success notice `Committed <shorthash> <subject>` after commit/amend,
  shown in the command-state slot, cleared on the next action.

Out of scope (later cycles):

- Push / remote-op success feedback (the push half of the original feedback item).
- The seven quick-win items: untracked-file preview, main-pane loading state,
  cursor re-clamp on reload, theming, README/help sync, busy-state gating.

## UX / layout

Commit mode renders inside the existing main pane. Normal commit and amend share
one layout; amend changes the title and footer verb and adds a conditional
warning row.

```
╭─ Commit ────────────────────────────────────╮
│ Commit message                               │
│ Committing 2 staged files                    │
│                                              │
│ Subject   type(scope): … · imperative, ≤50   │   ← label + muted conventional hint
│ feat(ui): add commit body support     23/50  │   ← textinput (focused), counter right-aligned
│ ──────────────────────────────────────────── │
│ Body                                         │
│ Explain why, not what. Wraps here.           │   ← textarea
│ █                                            │
│ Ctrl-D commit · Tab body · Esc cancel        │
╰──────────────────────────────────────────────╯
```

Counter colors: green ≤50, yellow ≤72, red >72. Warn-only — never blocks submit.

Amend differences:

- Title `Amend commit`; footer verb `Ctrl-D amend`.
- When the commit is already pushed, insert a red row above the fields:
  `⚠ already pushed — amend needs a force-push`.

Focus: `Tab` toggles subject↔body; the unfocused field dims.

Scope-hint line (the line under the title):

- Normal commit: existing `commitScopeHint` behavior (staged count, or
  "Nothing staged — committing all changes").
- Amend with staged changes: `Amending HEAD + N staged file(s)`.
- Amend with nothing staged: `Amending HEAD (message only)`.

## Behavior

Key bindings:

- `c` → open the commit editor (empty fields).
- `C` (new, `keyAmend`) → open the amend editor pre-filled with HEAD's message,
  `amending = true`.

In commit mode (`update.go` `handleKey`, `ModeCommitting` branch):

- `Tab` → toggle focused field (blur one, focus the other).
- Any other key → routed to the focused field's `Update`.
- `Ctrl-D` (submit):
  - Empty subject → no-op (stay in the editor).
  - Otherwise assemble the message: `subject` when body is blank, else
    `subject + "\n\n" + body`. Then:
    - amend **and** already pushed (`branch.Ahead == 0 && branch.Upstream != ""`)
      → enter `ModeConfirming` with the amend command as its `action`, prompt
      `Amend pushed commit? Needs a force-push. [y/n]`.
    - amend otherwise → run the amend command directly.
    - normal commit → commit the staged index if anything is staged, else
      stage-all + commit (existing logic, unchanged).
- `Esc` → reset both fields and `amending`, return to `ModeNormal`.

Success notice:

- On a successful commit/amend, the command sets a notice string
  `Committed <shorthash> <subject>`; the reducer stores it in `m.notice`.
- `m.notice` clears at the start of the next key handling (clear-on-next-action;
  no timer).

Rationale — amend confirm at submit time: loom's `ModeConfirming.action` is a
`tea.Cmd` (`update.go`), so the confirm flow can only dispatch commands, not open
an editor. Gating at submit keeps the amend command as the confirmed action. The
in-editor "already pushed" warning provides early awareness so the confirm is not
a surprise after typing.

## Git layer (`internal/git/repo.go`)

- `Commit(ctx, msg) (string, error)` — change signature: after a successful
  `commit -F -`, run `rev-parse --short HEAD` and return the short hash.
- `CommitAll(ctx, msg) (string, error)` — same, after `add -A` + commit.
- `CommitAmend(ctx, msg) (string, error)` (new) — `commit --amend -F -` then
  `rev-parse --short HEAD`. Does **not** auto-stage; amend folds in only the
  already-staged changes plus the new message.
- `HeadMessage(ctx) (string, error)` (new) — `git log -1 --pretty=%B HEAD`.
  The UI splits it: first line = subject; remainder (after a leading blank line)
  = body.

Determinism note: deriving the hash via `rev-parse` rather than parsing
`git commit` stdout avoids version/locale-dependent output.

## State & components (`internal/ui`)

`model.go`:

- Replace `input textarea.Model` with `subject textinput.Model` +
  `body textarea.Model`. `textinput` ships in the already-vendored
  `github.com/charmbracelet/bubbles` module — no new external dependency.
- Add `commitField` (enum: subject | body) for focus tracking.
- Add `amending bool`.
- Add `notice string` for the transient success line.

`view.go`:

- `commandState` (`view.go:173`) returns `notice` (green) when set and not
  busy/errored — that is how the success line surfaces; reusable for push
  feedback later.
- New rendering for the two-field editor, counter, conventional hint, and the
  conditional amend warning. Recompute the commit-mode header height
  (`commitHeaderHeight`) for the new layout; the body textarea takes the
  remaining height.

`commands.go`:

- Commit/amend commands capture the hash from the git layer and set the success
  notice on `gitDoneMsg`.

`keys.go`:

- Add `keyAmend = "C"`.

## Two implementation choices (decided)

- Success-hash transport: extend `gitDoneMsg` with a `notice string` field set by
  the commit command (one message type) — chosen over a separate
  `commitDoneMsg`.
- Amend-confirm timing: submit-time confirm + in-editor warning — chosen over a
  trigger-time confirm (which would require generalizing the confirm flow to
  perform state transitions).

## Testing

Mirror the existing `_test.go` style.

Pure / unit:

- Counter threshold → color mapping (≤50, ≤72, >72).
- Message assembly (`subject`, vs `subject + "\n\n" + body`; body omitted when blank).
- `HeadMessage` split into subject/body (single-line, multi-line with blank,
  trailing whitespace).
- Pushed-detection predicate (`Ahead == 0 && Upstream != ""`).

View:

- Commit mode renders both fields, counter, conventional hint.
- Amend mode renders the title/verb change and the warning row only when pushed.

Update:

- `Tab` toggles field focus.
- Empty-subject `Ctrl-D` is a no-op.
- Non-empty submit assembles the message and dispatches the right command
  (commit / commitAll / amend).
- Amend on a pushed branch routes through `ModeConfirming`.
- Success notice is set on `gitDoneMsg` success and cleared on the next key.

## Risks / notes

- `Commit`/`CommitAll` signature change touches `commands.go` and their tests;
  contained to the package.
- Amend on a pushed branch is intentionally gated but still permitted — loom does
  not perform the force-push itself; it only rewrites the local commit.
- `HeadMessage` on an empty repository (no HEAD) errors; the `C` handler should
  no-op when `HeadMessage` fails rather than entering the editor.
