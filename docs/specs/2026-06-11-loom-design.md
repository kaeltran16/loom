# loom — Design Spec

**Date:** 2026-06-11
**Status:** Approved (architecture), v1 implementation underway
**Location:** `C:\Users\kael02\IdeaProjects\loom`

## 1. What loom is

`loom` is a terminal UI (TUI) git client — a keyboard-driven daily driver that replaces
reaching for the `git` CLI for the common loop: see status → stage → commit → switch
branches → sync with the remote. The name reflects the metaphor: a loom weaves separate
threads (branches, commits) into one fabric (history).

It is **not** a reimplementation of git. loom shells out to the real `git` binary, so it
inherits the user's existing credentials, hooks, and config for free and always behaves
exactly like git.

## 2. Decisions (and why)

| Decision | Choice | Rationale |
|---|---|---|
| Purpose | A tool to actually use (not a learning exercise) | Drives "wrap real git, spend effort on UX," not "implement git internals." |
| Interface | TUI | Lowest-friction daily driver; proven pattern (lazygit, gitui, tig). |
| Language / framework | **Go + Bubble Tea** (+ Lipgloss + Bubbles) | Fastest path to a usable tool; goroutines make non-blocking git trivial; single static `.exe` on Windows; lazygit is Go (reference implementation in the same language). |
| Git access | **Shell out** to the `git` binary for all of v1 | Exact git behavior, free credentials/hooks/config. `go-git` is a *possible later* optimization for fast reads — **not** a v1 dependency. |
| v1 scope | Standard daily driver | Smallest set that fully replaces the CLI day-to-day; long-tail features added later as isolated views. |

## 3. Architecture — two layers, one seam

A single hard boundary separates *running git* from *the UI*:

```
┌─────────────────────────────────────────────┐
│  UI layer  (internal/ui)  — Bubble Tea MVU   │
│  model + update + view. Never builds a git    │
│  command string; calls git via tea.Cmd.       │
└───────────────▲──────────────────┬───────────┘
                │ typed msgs         │ method calls
                │ (async results)    ▼   (Status, Stage, Commit…)
┌───────────────┴──────────────────────────────┐
│  git layer (internal/git) — runs git, parses  │
│  output → typed structs. No UI knowledge.     │
│  Single Runner interface = the testable seam. │
└───────────────────────────────────────────────┘
```

**The `Runner` seam.** The git layer reaches the outside world through one interface:

```go
type Runner interface {
    Run(ctx context.Context, stdin io.Reader, args ...string) (stdout, stderr []byte, err error)
}
```

The production implementation uses `os/exec`. Tests inject a fake that returns canned
output. `stdin` is nil for most commands; it carries the commit message for `git commit -F -`.

This single seam plus **golden fixtures** (real git output captured into `testdata/`) makes
the entire git layer testable with no real repository and no subprocess. The UI layer is
testable for free because Bubble Tea's `Update(msg, model) → (model, cmd)` is a pure
function — call it directly and assert the resulting state.

## 4. Git access layer (`internal/git`)

High-level typed methods on a `Repo`. The UI calls these; it never sees a command string.

| Method | git invocation | Returns |
|---|---|---|
| `Status(ctx)` | `git status --porcelain=v2 --branch -z` | `[]FileStatus` + `BranchInfo{name, upstream, ahead, behind}` |
| `Diff(ctx, path, staged)` | `git diff [--cached] -- <path>` | raw unified diff text |
| `Stage(ctx, path)` | `git add -- <path>` | error |
| `Unstage(ctx, path)` | `git restore --staged -- <path>` | error |
| `Discard(ctx, path)` | `git restore -- <path>` | error (destructive → UI confirms) |
| `Commit(ctx, msg)` | `git commit -F -` (msg via stdin → multi-line safe) | error |
| `Branches(ctx)` | `git for-each-ref --format='%(refname:short)%00%(upstream:short)%00%(HEAD)' refs/heads` | `[]Branch{name, upstream, current}` |
| `Switch(ctx, branch)` | `git switch <branch>` | error |
| `Log(ctx, ref, n)` | `git log --format='%H%x00%s%x00%an%x00%ar' -n <n> [ref]` | `[]Commit{hash, subject, author, relTime}` |
| `Show(ctx, hash)` | `git show <hash>` | commit message + diff text |
| `Fetch/Pull/Push(ctx)` | `git fetch` / `git pull` / `git push` | combined output + error (long-running) |

**Parsers are pure functions** (`status.go`, `branch.go`, `log.go`): `[]byte → struct`. They
are what the golden fixtures test. NUL (`-z` / `%x00`) separation is used everywhere paths or
free-text fields appear, so spaces and unusual characters never break parsing.

The status parser handles porcelain v2 line types: `1` (changed), `2` (renamed/copied),
`u` (unmerged), `?` (untracked), plus the `# branch.head` and `# branch.ab` header lines.

## 5. UI layer (`internal/ui`) — Model-View-Update

**Panels (v1):** a column of context panels — **Files · Branches · Commits** — drives a
large **main pane**, with a **status/footer bar** beneath. Number keys (`1/2/3`) jump panels,
`Tab` cycles, the focused panel's border highlights.

```
┌ Files ─────────┐┌ Diff: src/app.go (unstaged) ───────────────┐
│  M src/app.go  ││ @@ -12,7 +12,9 @@ func run() {               │
│  M README.md   ││ -  p := tea.NewProgram(model)               │
│ ?? notes.txt   ││ +  p := tea.NewProgram(model, tea.WithAlt…)  │
├ Branches ──────┤│ +  // run without blocking                  │
│ *main          ││    if _, err := p.Run(); err != nil {       │
│  feat/login    ││                                             │
├ Commits ───────┤│                                             │
│  a1b2c3 fix nav││                                             │
│  d4e5f6 init   ││                                             │
└────────────────┘└─────────────────────────────────────────────┘
 main ↑2 ↓0   [space] stage  [c] commit  [P] push  [?] help   ⠹ fetching…
```

**Main-pane content depends on the focused panel** (all reuse one scrollable viewport):
- Files focused → `Diff` of the selected file (staged or unstaged, by section).
- Commits focused → `Show` of the selected commit (message + diff).
- Branches focused → `Log` of the selected branch.

### Model

```go
type Model struct {
    repo     *git.Repo
    files    []git.FileStatus
    branches []git.Branch
    commits  []git.Commit
    focus    Panel              // Files | Branches | Commits
    cursor   map[Panel]int      // selection index per panel
    viewport viewport.Model     // bubbles: scrollable main pane
    input    textarea.Model     // bubbles: commit message editor
    spinner  spinner.Model      // bubbles: busy indicator
    mode     Mode               // Normal | Committing | Confirming
    confirm  confirmReq         // pending destructive action + prompt
    busy     bool
    cmdLog   []string           // the git commands we ran (transparency)
    err      error              // last error, shown in the footer
    w, h     int
}
```

### Messages (delivered back into `Update`)

`statusLoadedMsg{files, branch}` · `branchesLoadedMsg{branches}` · `commitsLoadedMsg{commits}`
· `diffLoadedMsg{text}` · `gitDoneMsg{cmd, output, err}` (generic mutation result) · `errMsg{err}`
· plus `spinner.TickMsg` and `tea.WindowSizeMsg`.

### Commands (`commands.go`)

Each git method is wrapped in a `tea.Cmd` that Bubble Tea runs in its own goroutine and that
returns one of the messages above: `loadStatus`, `loadBranches`, `loadCommits`, `loadMain`
(focus-aware diff/show/log), `stageFile`, `unstageFile`, `discardFile`, `commit`, `switchBranch`,
`fetch`, `pull`, `push`. This is what keeps the UI non-blocking.

### Keymap (context-sensitive per focused panel)

| Key | Files | Branches | Commits |
|---|---|---|---|
| `↑/↓` or `j/k` | move cursor | move cursor | move cursor |
| `space` | stage / unstage toggle | — | — |
| `d` | discard (confirm) | — | — |
| `enter` | — | switch to branch | — |
| `c` | commit (any panel) | commit | commit |
| `f` / `p` / `P` | fetch / pull / push (any panel) | | |
| `Tab`, `1/2/3` | change focus | | |
| `?` | help overlay | | |
| `q` / `Ctrl-C` | quit | | |

Commit flow: `c` → `mode = Committing` → `input` textarea takes over the main pane → type
message → `Ctrl-D` confirms (`commit` Cmd), `Esc` cancels. On success: refresh status + log,
clear input, return to `Normal`.

## 6. Data flow — a stage action, end to end

```
[space] on Files panel
   → Update: focus=Files, mode=Normal  ⇒ return stageFile(path), set busy=true
   → Bubble Tea runs the Cmd in a goroutine        ← UI stays responsive, spinner ticks
   → git add runs → returns gitDoneMsg{cmd:"git add …", err:nil}
   → Update: busy=false, append to cmdLog  ⇒ return loadStatus() (refresh)
   → statusLoadedMsg ⇒ Update updates files ⇒ View re-renders
```

Every feature is this same shape — *keypress → dispatch Cmd → git runs async → result msg →
state update → chained refresh → re-render.* Commit, push, and switch are copies with
different methods; they add views, not new architecture.

## 7. Error handling

- **git layer** wraps stderr with context: `fmt.Errorf("git status: %w: %s", err, stderr)`.
  Errors are never swallowed.
- **UI layer**: `errMsg` → `model.err` → rendered in the footer error bar. A git failure never
  crashes the loop.
- **Destructive ops** (discard, force-push if added later): `mode = Confirming` renders a
  `[y/n]` prompt before the Cmd is dispatched.
- **Startup**: if the working directory is not inside a git repo, print a friendly message and
  exit cleanly (no panic, no stack trace).

## 8. Testing strategy

- **git layer (the bulk):** `Runner` mocked for dispatch; parsers tested with golden fixtures
  (`testdata/*.txt` captured from real git output), table-driven. This is where the real risk
  lives (porcelain v2 parsing) and where coverage is concentrated.
- **UI layer:** call `Update(msg)` directly and assert model state — e.g. "`space` on Files
  issues a stage Cmd and sets `busy`"; "`Committing` + `Ctrl-D` issues a commit Cmd"; "`d` on
  Files enters `Confirming`." No terminal required. Optional `teatest` golden-frame tests for
  the rendered `View` can be added later.

## 9. Project structure

```
loom/
├── main.go                 # find repo (walk up for .git), build Repo, start tea.Program
├── go.mod
├── internal/
│   ├── git/
│   │   ├── runner.go       # Runner interface + os/exec implementation
│   │   ├── repo.go         # Repo: Status/Stage/Commit/Push/… (the methods table)
│   │   ├── status.go       # porcelain v2 parser
│   │   ├── branch.go       # for-each-ref parser
│   │   ├── log.go          # log parser
│   │   ├── *_test.go
│   │   └── testdata/       # golden fixtures
│   └── ui/
│       ├── model.go        # Model, Init
│       ├── update.go       # Update + keymap dispatch
│       ├── view.go         # View + Lipgloss layout (+ basic +/- diff coloring)
│       ├── commands.go     # tea.Cmd wrappers around git methods
│       ├── keys.go         # keybinding definitions
│       ├── panels.go       # panel render helpers
│       └── *_test.go
└── README.md
```

## 10. v1 scope

**In:** status view; file-level stage / unstage; discard (confirmed); commit (multi-line);
branch list + switch; focus-aware main pane (file diff / commit show / branch log) with basic
`+`/`-` coloring; scrollable diff viewport; fetch / pull / push; context-sensitive keymap +
help overlay; command log (transparency); friendly not-a-repo handling.

**Out (future, each an isolated addition):** commit-graph / DAG visualization; hunk- and
line-level staging; interactive rebase; merge-conflict resolution; stash management; reflog /
undo view; submodules, worktrees, bisect; syntax-highlighted diffs; mouse support; config /
theming.

## 11. Platform / UX notes (Windows 11)

- Target **Windows Terminal**, not legacy `conhost` (truecolor, Unicode, box-drawing). Document
  this requirement in the README.
- Glyphs (branch icons, arrows) need a capable font; provide a **plain-ASCII fallback** so loom
  degrades gracefully in arbitrary shells / over SSH. Prefer simple box-drawing for structural UI.
- Avoid double-width emoji/CJK in structural elements to keep alignment stable.

## 12. Open questions (resolve during planning)

- Exact `go.mod` module path (e.g. `github.com/<user>/loom` vs a local path) — needed before
  `go mod init`.
- Whether the command log is always-visible (a thin bottom strip) or toggled (`x`) — leaning
  toggled to save vertical space; decide during implementation.
