# Merge-Conflict Handling (Editor Delegation) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When a merge conflicts, stop stranding the user — surface the merging state, hand each conflicted file to the user's editor, and let them finish or abort the merge, all by wrapping real git.

**Architecture:** Two small git-layer additions land first (capture the conflict-kind code, add `MergeAbort`/`Merging`/`EditorCommand`), each testable in isolation. The UI then tracks merge state from the status load, renders a conflict-aware banner/footer/labels, and wires two new keys: `A` (abort, through the existing confirm gate) and `e` (open the file in the editor via Bubble Tea's `tea.ExecProcess`). Actual conflict editing is delegated to the editor; loom never parses conflict regions.

**Tech Stack:** Go 1.26, Bubble Tea (`tea`, incl. `tea.ExecProcess`), Bubbles, Lipgloss. Standard `go test` table-driven tests.

**Spec:** `docs/superpowers/specs/2026-06-16-merge-conflict-editor-delegation-design.md`

**Note on a spec refinement:** the spec sketched `EditorArgv(ctx) ([]string, error)`. This plan implements `EditorCommand(ctx) string` instead (no error — a failed `git config` read just falls through to the VS Code default), and moves the platform shell-wrapping into the UI's `editorExecCmd`. The git layer stays free of process-launch concerns; the UI owns `os/exec`.

---

## File Structure

- **Modify** `internal/git/status.go` — `FileStatus` gains `Conflict string`; set it from the `u`-line code. Keep the existing `Unmerged bool` (used widely) to minimize churn.
- **Modify** `internal/git/repo.go` — add `MergeAbort` and `Merging`.
- **Create** `internal/git/editor.go` — `chooseEditor` (pure) + `EditorCommand` (Repo method). Editor concern lives in one focused file.
- **Modify** `internal/ui/messages.go` — `statusLoadedMsg.merging`; new `editorDoneMsg`.
- **Modify** `internal/ui/model.go` — `merging bool` state.
- **Modify** `internal/ui/commands.go` — `loadStatus` also reads merge state; new `mergeAbort`, `openEditor`, `editorExecCmd`.
- **Modify** `internal/ui/update.go` — set `merging` from status; `A` abort flow; `e` edit flow; `editorDoneMsg` refresh.
- **Modify** `internal/ui/keys.go` — `keyAbortMerge = "A"`, `keyEditConflict = "e"`.
- **Modify** `internal/ui/view.go` — merge banner in top bar; conflict footer hints; conflict-kind in selected-context; help-overlay rows.
- **Modify** `internal/ui/panels.go` — `conflictLabel(code)`.
- **Modify** `README.md` — keys table rows for `e` and `A`.
- **Modify** test files alongside each: `status_test.go`, `repo_test.go`, `editor_test.go` (new), `update_test.go`, `view_test.go`, `panels_test.go`, `commands_test.go`.

---

## Task 1: Capture conflict kind from porcelain status

**Files:**
- Modify: `internal/git/status.go:10-17` (struct) and `internal/git/status.go:66-71` (`u`-line parse)
- Test: `internal/git/status_test.go:37-40`

The `u` line is `u <XY> <sub> … <path>`; `strings.Fields` puts the two-char code (`UU`, `AA`, …) at `f[1]`. The current parser already splits this line and uses `f[10:]` for the path — we just also keep `f[1]`.

- [ ] **Step 1: Update the failing assertion**

In `internal/git/status_test.go`, replace the unmerged block (lines 37-40):

```go
	// unmerged: both modified (code UU in the fixture)
	if files[3].Path != "conflict.txt" || !files[3].Unmerged || files[3].Conflict != "UU" {
		t.Errorf("file[3] wrong: %+v", files[3])
	}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/git/ -run TestParseStatus -v`
Expected: FAIL — `files[3].Conflict undefined` (compile error).

- [ ] **Step 3: Add the field and capture the code**

In `internal/git/status.go`, replace the `FileStatus` struct (lines 10-17):

```go
// FileStatus is one changed path in the working tree.
type FileStatus struct {
	Path      string
	Staged    rune   // index status char (X); '.' means none
	Worktree  rune   // worktree status char (Y); '.' means none
	Untracked bool
	Unmerged  bool
	Conflict  string // porcelain v2 unmerged code (e.g. "UU"); "" when not a conflict
}
```

Replace the `u`-line case (lines 66-71):

```go
		case strings.HasPrefix(line, "u "):
			// u <xy> <sub> <m1> <m2> <m3> <mW> <h1> <h2> <h3> <path>
			f := strings.Fields(line)
			if len(f) >= 11 {
				files = append(files, FileStatus{Path: strings.Join(f[10:], " "), Unmerged: true, Conflict: f[1]})
			}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/git/ -run TestParseStatus -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/git/status.go internal/git/status_test.go
git commit -m "feat(git): capture conflict kind from porcelain status"
```

---

## Task 2: Merge abort + in-merge detection

**Files:**
- Modify: `internal/git/repo.go` (add two methods after `Discard`/`Switch`, e.g. near line 97)
- Test: `internal/git/repo_test.go`

`Merging` uses `git rev-parse --verify --quiet MERGE_HEAD`, which exits 0 when a merge is in progress and non-zero (no output, due to `--quiet`) when not. A non-zero exit is read as "not merging," never propagated as an error. The `fakeRunner` returns the same canned `err` for every call, so its presence/absence drives the boolean.

- [ ] **Step 1: Write the failing tests**

Append to `internal/git/repo_test.go`:

```go
func TestRepo_MergeAbort_args(t *testing.T) {
	fr := &fakeRunner{}
	if err := (&Repo{runner: fr}).MergeAbort(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{"merge", "--abort"}
	if !reflect.DeepEqual(fr.gotArgs, want) {
		t.Errorf("args = %v, want %v", fr.gotArgs, want)
	}
}

func TestRepo_Merging_trueWhenRevParseSucceeds(t *testing.T) {
	fr := &fakeRunner{}
	got, err := (&Repo{runner: fr}).Merging(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected merging=true when rev-parse succeeds")
	}
	want := []string{"rev-parse", "--verify", "--quiet", "MERGE_HEAD"}
	if !reflect.DeepEqual(fr.gotArgs, want) {
		t.Errorf("args = %v, want %v", fr.gotArgs, want)
	}
}

func TestRepo_Merging_falseWhenRevParseFails(t *testing.T) {
	fr := &fakeRunner{err: errors.New("no MERGE_HEAD")}
	got, err := (&Repo{runner: fr}).Merging(context.Background())
	if err != nil {
		t.Fatalf("Merging should swallow the absent-ref error, got %v", err)
	}
	if got {
		t.Error("expected merging=false when rev-parse fails")
	}
}
```

(`errors` and `reflect` are already imported in `repo_test.go`.)

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/git/ -run 'TestRepo_MergeAbort|TestRepo_Merging' -v`
Expected: FAIL — `r.MergeAbort undefined`, `r.Merging undefined`.

- [ ] **Step 3: Implement the methods**

In `internal/git/repo.go`, add after `Switch` (line 97):

```go
// MergeAbort cancels an in-progress merge, restoring the pre-merge state.
func (r *Repo) MergeAbort(ctx context.Context) error {
	return r.mutate(ctx, nil, "merge", "--abort")
}

// Merging reports whether a merge is in progress (a MERGE_HEAD ref exists).
// rev-parse exits non-zero when the ref is absent, which we read as "not
// merging" rather than a failure.
func (r *Repo) Merging(ctx context.Context) (bool, error) {
	_, _, err := r.runner.Run(ctx, nil, "rev-parse", "--verify", "--quiet", "MERGE_HEAD")
	return err == nil, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/git/ -run 'TestRepo_MergeAbort|TestRepo_Merging' -v`
Expected: PASS (all three).

- [ ] **Step 5: Commit**

```bash
git add internal/git/repo.go internal/git/repo_test.go
git commit -m "feat(git): add merge abort and in-merge detection"
```

---

## Task 3: Resolve the editor command

**Files:**
- Create: `internal/git/editor.go`
- Test: `internal/git/editor_test.go`

Precedence mirrors git's own (`$GIT_EDITOR` → `core.editor` → `$VISUAL` → `$EDITOR`); when all are empty, fall back to `code --wait`. The pure `chooseEditor` is fully unit-tested; the thin `EditorCommand` method reads env + git config.

- [ ] **Step 1: Write the failing tests**

Create `internal/git/editor_test.go`:

```go
package git

import (
	"context"
	"reflect"
	"testing"
)

func TestChooseEditor(t *testing.T) {
	cases := []struct {
		name                            string
		gitEditor, core, visual, editor string
		want                            string
	}{
		{"git editor wins", "vim", "nano", "emacs", "vi", "vim"},
		{"core next", "", "nano", "emacs", "vi", "nano"},
		{"visual next", "", "", "emacs", "vi", "emacs"},
		{"editor last", "", "", "", "vi", "vi"},
		{"all empty falls back", "", "", "", "", "code --wait"},
		{"whitespace ignored", "  ", "", "", "", "code --wait"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := chooseEditor(c.gitEditor, c.core, c.visual, c.editor); got != c.want {
				t.Errorf("chooseEditor = %q, want %q", got, c.want)
			}
		})
	}
}

func TestRepo_EditorCommand_fallsBackToVSCode(t *testing.T) {
	t.Setenv("GIT_EDITOR", "")
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	fr := &fakeRunner{} // core.editor unset → empty stdout
	got := (&Repo{runner: fr}).EditorCommand(context.Background())
	if got != "code --wait" {
		t.Errorf("EditorCommand = %q, want code --wait", got)
	}
	want := []string{"config", "--get", "core.editor"}
	if !reflect.DeepEqual(fr.gotArgs, want) {
		t.Errorf("args = %v, want %v", fr.gotArgs, want)
	}
}

func TestRepo_EditorCommand_prefersGitEditorEnv(t *testing.T) {
	t.Setenv("GIT_EDITOR", "vim")
	got := (&Repo{runner: &fakeRunner{}}).EditorCommand(context.Background())
	if got != "vim" {
		t.Errorf("EditorCommand = %q, want vim", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/git/ -run 'ChooseEditor|EditorCommand' -v`
Expected: FAIL — `undefined: chooseEditor`, `r.EditorCommand undefined`.

- [ ] **Step 3: Implement**

Create `internal/git/editor.go`:

```go
package git

import (
	"context"
	"os"
	"strings"
)

// defaultEditor is loom's fallback when neither git nor the environment names an
// editor. VS Code's --wait blocks until the file is closed, which loom relies on
// so it does not resume before the edit is finished.
const defaultEditor = "code --wait"

// chooseEditor returns the editor command to use, mirroring git's precedence
// (GIT_EDITOR, core.editor, VISUAL, EDITOR) and falling back to VS Code.
func chooseEditor(gitEditor, coreEditor, visual, editor string) string {
	for _, c := range []string{gitEditor, coreEditor, visual, editor} {
		if s := strings.TrimSpace(c); s != "" {
			return s
		}
	}
	return defaultEditor
}

// EditorCommand resolves the editor command git itself would use for this repo,
// falling back to VS Code when nothing is configured. The result is a shell
// command string (e.g. "code --wait" or "vim"); the caller appends the file.
func (r *Repo) EditorCommand(ctx context.Context) string {
	core, _, _ := r.runner.Run(ctx, nil, "config", "--get", "core.editor")
	return chooseEditor(
		os.Getenv("GIT_EDITOR"),
		strings.TrimSpace(string(core)),
		os.Getenv("VISUAL"),
		os.Getenv("EDITOR"),
	)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/git/ -run 'ChooseEditor|EditorCommand' -v`
Expected: PASS.

- [ ] **Step 5: Run the whole git package**

Run: `go test ./internal/git/ -v`
Expected: PASS — the git layer is green.

- [ ] **Step 6: Commit**

```bash
git add internal/git/editor.go internal/git/editor_test.go
git commit -m "feat(git): resolve the editor command with a VS Code default"
```

---

## Task 4: Track in-merge state from the status load

**Files:**
- Modify: `internal/ui/messages.go:5-8`
- Modify: `internal/ui/model.go:76` (add field near `busy`)
- Modify: `internal/ui/commands.go:13-21` (`loadStatus`)
- Modify: `internal/ui/update.go:22-25` (`statusLoadedMsg` case)
- Test: `internal/ui/update_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/ui/update_test.go`:

```go
func TestUpdate_StatusLoadedSetsMerging(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(statusLoadedMsg{merging: true})
	if !updated.(Model).merging {
		t.Error("expected merging=true after statusLoadedMsg{merging:true}")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/ui/ -run TestUpdate_StatusLoadedSetsMerging -v`
Expected: FAIL — `unknown field 'merging' in struct literal` and `m.merging undefined`.

- [ ] **Step 3: Add the message field**

In `internal/ui/messages.go`, replace the `statusLoadedMsg` struct (lines 5-8):

```go
type statusLoadedMsg struct {
	files   []git.FileStatus
	branch  git.BranchInfo
	merging bool
}
```

- [ ] **Step 4: Add the model field**

In `internal/ui/model.go`, add to the `Model` struct directly after the `busy bool` line (line 76):

```go
	merging     bool // a merge is in progress (MERGE_HEAD exists)
```

- [ ] **Step 5: Read merge state in `loadStatus`**

In `internal/ui/commands.go`, replace `loadStatus` (lines 13-21):

```go
func loadStatus(ctx context.Context, repo *git.Repo) tea.Cmd {
	return func() tea.Msg {
		files, branch, err := repo.Status(ctx)
		if err != nil {
			return errMsg{err}
		}
		merging, _ := repo.Merging(ctx)
		return statusLoadedMsg{files: files, branch: branch, merging: merging}
	}
}
```

- [ ] **Step 6: Store it in the reducer**

In `internal/ui/update.go`, replace the `statusLoadedMsg` case (lines 22-25):

```go
	case statusLoadedMsg:
		m.files, m.err = msg.files, nil
		m.branch = msg.branch
		m.merging = msg.merging
		return m.reloadMain()
```

- [ ] **Step 7: Run the test to verify it passes**

Run: `go test ./internal/ui/ -run TestUpdate_StatusLoadedSetsMerging -v`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/ui/messages.go internal/ui/model.go internal/ui/commands.go internal/ui/update.go internal/ui/update_test.go
git commit -m "feat(ui): track in-merge state from status"
```

---

## Task 5: Conflict-aware banner, footer, and labels

**Files:**
- Modify: `internal/ui/panels.go` (add `conflictLabel`)
- Modify: `internal/ui/view.go` (`topBar`, add `mergeBanner`/`unmergedCount`, `selectedContextLines`, `footerHints`)
- Test: `internal/ui/panels_test.go`, `internal/ui/view_test.go`

This is presentation only and reads `m.merging` (Task 4) and `FileStatus.Conflict` (Task 1).

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/panels_test.go`:

```go
func TestConflictLabel(t *testing.T) {
	cases := map[string]string{
		"UU": "both modified",
		"AA": "both added",
		"DD": "both deleted",
		"AU": "added by us",
		"UD": "deleted by them",
		"UA": "added by them",
		"DU": "deleted by us",
		"":   "unmerged",
		"ZZ": "unmerged",
	}
	for code, want := range cases {
		if got := conflictLabel(code); got != want {
			t.Errorf("conflictLabel(%q) = %q, want %q", code, got, want)
		}
	}
}
```

Append to `internal/ui/view_test.go`:

```go
func TestMergeBanner(t *testing.T) {
	m := newTestModel()
	if got := m.mergeBanner(); got != "" {
		t.Errorf("not merging: banner = %q, want empty", got)
	}
	m.merging = true
	m.files = []git.FileStatus{{Path: "a", Unmerged: true}}
	if got := m.mergeBanner(); got != "MERGING — 1 conflict" {
		t.Errorf("banner = %q, want 'MERGING — 1 conflict'", got)
	}
	m.files = []git.FileStatus{{Path: "a", Unmerged: true}, {Path: "b", Unmerged: true}}
	if got := m.mergeBanner(); got != "MERGING — 2 conflicts" {
		t.Errorf("banner = %q, want 'MERGING — 2 conflicts'", got)
	}
	m.files = nil
	if got := m.mergeBanner(); got != "MERGING — ready to commit" {
		t.Errorf("banner = %q, want 'MERGING — ready to commit'", got)
	}
}

func TestFooterConflictHints(t *testing.T) {
	m := newTestModel()
	m.merging = true
	m.focus = PanelFiles
	if got := m.footerActions(); got != "Conflict: e edit · space resolve · A abort · c commit" {
		t.Errorf("footer = %q", got)
	}
}

func TestSelectedContextShowsConflictKind(t *testing.T) {
	m := newTestModel()
	m.focus = PanelFiles
	m.files = []git.FileStatus{{Path: "a.go", Unmerged: true, Conflict: "UU"}}
	joined := strings.Join(m.selectedContextLines(), "\n")
	if !strings.Contains(joined, "conflict: both modified") {
		t.Errorf("selected context = %q, want conflict kind", joined)
	}
}
```

(`git` and `strings` are already imported in `view_test.go`.)

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/ui/ -run 'TestConflictLabel|TestMergeBanner|TestFooterConflictHints|TestSelectedContextShowsConflictKind' -v`
Expected: FAIL — `undefined: conflictLabel`, `m.mergeBanner undefined`, footer mismatch.

- [ ] **Step 3: Add `conflictLabel`**

In `internal/ui/panels.go`, add after `markerColor` (after line 58):

```go
// conflictLabel maps a porcelain v2 unmerged code to a human description.
func conflictLabel(code string) string {
	switch code {
	case "UU":
		return "both modified"
	case "AA":
		return "both added"
	case "DD":
		return "both deleted"
	case "AU":
		return "added by us"
	case "UD":
		return "deleted by them"
	case "UA":
		return "added by them"
	case "DU":
		return "deleted by us"
	default:
		return "unmerged"
	}
}
```

- [ ] **Step 4: Add the merge banner and wire it into the top bar**

In `internal/ui/view.go`, replace `topBar` (lines 125-133):

```go
func (m Model) topBar() string {
	stateText, stateStyle := m.commandState()
	parts := []string{
		m.branchSummary(),
		m.workflowTabs(),
		stateStyle.Render(stateText),
	}
	if banner := m.mergeBanner(); banner != "" {
		parts = append(parts, warnStyle.Render(banner))
	}
	return strings.Join(parts, mutedStyle.Render(" | "))
}

// mergeBanner is the top-bar merge cue: the remaining-conflict count while any
// remain, then "ready to commit" once all are resolved but the merge commit has
// not landed. Empty when not merging.
func (m Model) mergeBanner() string {
	if !m.merging {
		return ""
	}
	switch n := m.unmergedCount(); n {
	case 0:
		return "MERGING — ready to commit"
	case 1:
		return "MERGING — 1 conflict"
	default:
		return fmt.Sprintf("MERGING — %d conflicts", n)
	}
}

func (m Model) unmergedCount() int {
	n := 0
	for _, f := range m.files {
		if f.Unmerged {
			n++
		}
	}
	return n
}
```

- [ ] **Step 5: Show the conflict kind in the selected-context lines**

In `internal/ui/view.go`, replace the unmerged block in `selectedContextLines` (lines 213-216):

```go
			if f.Unmerged {
				state = "conflict: " + conflictLabel(f.Conflict)
				actions = "actions: e edit · space resolve · A abort"
			}
```

- [ ] **Step 6: Add conflict footer hints**

In `internal/ui/view.go`, replace the `PanelFiles` case in `footerHints` (lines 501-502):

```go
	case PanelFiles:
		if m.merging {
			return "Conflict", []keyHint{{"e", "edit"}, {"space", "resolve"}, {"A", "abort"}, {"c", "commit"}}
		}
		return "Files", []keyHint{{"space", "stage"}, {"d", "discard"}, {"c", "commit"}, {"?", "help"}, {"q", "quit"}}
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test ./internal/ui/ -run 'TestConflictLabel|TestMergeBanner|TestFooterConflictHints|TestSelectedContextShowsConflictKind' -v`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/ui/panels.go internal/ui/view.go internal/ui/panels_test.go internal/ui/view_test.go
git commit -m "feat(ui): conflict-aware banner, footer, and labels"
```

---

## Task 6: Abort an in-progress merge

**Files:**
- Modify: `internal/ui/keys.go` (add `keyAbortMerge`)
- Modify: `internal/ui/commands.go` (add `mergeAbort`)
- Modify: `internal/ui/update.go` (`handleKey` case)
- Test: `internal/ui/update_test.go`, `internal/ui/commands_test.go`

Abort reuses the existing `ModeConfirming` gate — the same one `d`/discard uses.

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/update_test.go`:

```go
func TestUpdate_AbortMergeEntersConfirm(t *testing.T) {
	m := newTestModel()
	m.merging = true
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("A")})
	got := updated.(Model)
	if got.mode != ModeConfirming {
		t.Fatalf("mode = %v, want ModeConfirming", got.mode)
	}
	if got.confirm.action == nil {
		t.Error("expected an abort action stored on the confirm request")
	}
	if cmd != nil {
		t.Error("abort should not dispatch until y is pressed")
	}
}

func TestUpdate_AbortMergeIgnoredWhenNotMerging(t *testing.T) {
	m := newTestModel() // merging is false
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("A")})
	if updated.(Model).mode != ModeNormal {
		t.Error("A should be a no-op when not merging")
	}
}
```

Append to `internal/ui/commands_test.go`:

```go
func TestMergeAbortCmd_label(t *testing.T) {
	repo := git.NewTestRepo(&git.StubRunner{})
	msg := mergeAbort(context.Background(), repo)().(gitDoneMsg)
	if msg.err != nil {
		t.Fatalf("unexpected error: %v", msg.err)
	}
	if msg.cmd != "git merge --abort" {
		t.Errorf("cmd = %q, want git merge --abort", msg.cmd)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/ui/ -run 'AbortMerge|MergeAbortCmd' -v`
Expected: FAIL — `undefined: mergeAbort`, `keyAbortMerge` not handled (so `A` stays in `ModeNormal`).

- [ ] **Step 3: Add the key**

In `internal/ui/keys.go`, add to the const block (after `keyBottom` line 17):

```go
	keyAbortMerge   = "A"
```

- [ ] **Step 4: Add the command**

In `internal/ui/commands.go`, add after `switchBranch` (after line 103):

```go
func mergeAbort(ctx context.Context, repo *git.Repo) tea.Cmd {
	return mutation("git merge --abort", func() error { return repo.MergeAbort(ctx) })
}
```

- [ ] **Step 5: Handle the key in the reducer**

In `internal/ui/update.go`, add a case in `handleKey`'s normal-mode switch, directly after the `keyDiscard` case (after line 199):

```go
	case keyAbortMerge:
		if !m.merging {
			return m, nil
		}
		m.mode = ModeConfirming
		m.confirm = confirmReq{
			prompt: "Abort the merge? Conflict resolutions will be discarded. [y/n]",
			action: mergeAbort(m.ctx, m.repo),
		}
		return m, nil
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/ui/ -run 'AbortMerge|MergeAbortCmd' -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/keys.go internal/ui/commands.go internal/ui/update.go internal/ui/update_test.go internal/ui/commands_test.go
git commit -m "feat(ui): abort an in-progress merge with a confirm"
```

---

## Task 7: Open a conflicted file in the editor

**Files:**
- Modify: `internal/ui/keys.go` (add `keyEditConflict`)
- Modify: `internal/ui/messages.go` (add `editorDoneMsg`)
- Modify: `internal/ui/commands.go` (add `openEditor`, `editorExecCmd`; new imports)
- Modify: `internal/ui/update.go` (`editorDoneMsg` case + `e` key + `editConflict`)
- Test: `internal/ui/update_test.go`, `internal/ui/commands_test.go`

`tea.ExecProcess` suspends the TUI, runs the editor attached to the real terminal, and delivers `editorDoneMsg` on exit. Tests only check the *dispatch* decision (a non-nil Cmd, the right message) and that `editorExecCmd` embeds the editor and file — they never run the returned process.

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/update_test.go`:

```go
func TestUpdate_EditConflictDispatchesWhenUnmerged(t *testing.T) {
	m := newTestModel()
	m.files = []git.FileStatus{{Path: "a.go", Unmerged: true}}
	m.focus = PanelFiles
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if cmd == nil {
		t.Fatal("expected an open-editor cmd for an unmerged file")
	}
}

func TestUpdate_EditConflictNoopWhenNotUnmerged(t *testing.T) {
	m := newTestModel()
	m.files = []git.FileStatus{{Path: "a.go", Worktree: 'M'}}
	m.focus = PanelFiles
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if cmd != nil {
		t.Error("e on a non-conflicted file should be a no-op")
	}
}

func TestUpdate_EditorDoneRefreshes(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(editorDoneMsg{})
	if cmd == nil {
		t.Error("expected a status refresh after editorDoneMsg")
	}
}

func TestUpdate_EditorDoneErrorSurfaces(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(editorDoneMsg{err: errFake("boom")})
	if updated.(Model).err == nil {
		t.Error("expected err set on editor failure")
	}
}
```

(`errFake` is the existing UI-test error helper used by the amend tests.)

Append to `internal/ui/commands_test.go`:

```go
func TestEditorExecCmd_embedsEditorAndFile(t *testing.T) {
	c := editorExecCmd("code --wait", "/repo/a.go")
	joined := strings.Join(c.Args, " ")
	if !strings.Contains(joined, "code --wait") || !strings.Contains(joined, "a.go") {
		t.Errorf("args = %v, want editor and file embedded", c.Args)
	}
}
```

Add `"strings"` to the imports of `internal/ui/commands_test.go` if not already present.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/ui/ -run 'EditConflict|EditorDone|EditorExecCmd' -v`
Expected: FAIL — `undefined: editorDoneMsg`, `undefined: editorExecCmd`, `e` not handled.

- [ ] **Step 3: Add the key and message**

In `internal/ui/keys.go`, add to the const block (after `keyDiscard` line 19):

```go
	keyEditConflict = "e"
```

In `internal/ui/messages.go`, add at the end of the file:

```go
// editorDoneMsg reports that the external editor launched for a conflicted file
// has exited.
type editorDoneMsg struct{ err error }
```

- [ ] **Step 4: Add the command and its shell wrapper**

In `internal/ui/commands.go`, update the import block to add `os/exec` and `runtime`:

```go
import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kael02/loom/internal/git"
)
```

Add after `mergeAbort` (from Task 6):

```go
// openEditor suspends the TUI and opens the conflicted file in the user's editor
// (the one git would use, else VS Code). On exit it yields editorDoneMsg.
func openEditor(ctx context.Context, repo *git.Repo, path string) tea.Cmd {
	editor := repo.EditorCommand(ctx)
	full := filepath.Join(repo.Root(), path)
	return tea.ExecProcess(editorExecCmd(editor, full), func(err error) tea.Msg {
		return editorDoneMsg{err: err}
	})
}

// editorExecCmd builds the command that opens file in editor, run through the
// platform shell so a multi-word editor string ("code --wait") launches the same
// way git launches core.editor.
func editorExecCmd(editor, file string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/c", editor+` "`+file+`"`)
	}
	return exec.Command("sh", "-c", editor+` "$0"`, file)
}
```

- [ ] **Step 5: Handle the key and the done message in the reducer**

In `internal/ui/update.go`, add a case to `Update`'s type switch, directly after the `errMsg` case (after line 63):

```go
	case editorDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		return m, loadStatus(m.ctx, m.repo)
```

Add a case in `handleKey`'s normal-mode switch, directly after the `keyAbortMerge` case (from Task 6):

```go
	case keyEditConflict:
		return m.editConflict()
```

Add the helper after `discardSelected` (after line 311):

```go
// editConflict opens the selected file in the editor, but only when it is an
// unmerged conflict in the Files panel.
func (m Model) editConflict() (tea.Model, tea.Cmd) {
	if m.focus != PanelFiles {
		return m, nil
	}
	i := m.cursor[PanelFiles]
	if i >= len(m.files) || !m.files[i].Unmerged {
		return m, nil
	}
	return m, openEditor(m.ctx, m.repo, m.files[i].Path)
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/ui/ -run 'EditConflict|EditorDone|EditorExecCmd' -v`
Expected: PASS.

- [ ] **Step 7: Build and run the whole suite**

Run: `go build ./... && go test ./... -v`
Expected: PASS — entire project green.

- [ ] **Step 8: Commit**

```bash
git add internal/ui/keys.go internal/ui/messages.go internal/ui/commands.go internal/ui/update.go internal/ui/update_test.go internal/ui/commands_test.go
git commit -m "feat(ui): open a conflicted file in the editor"
```

---

## Task 8: Document the conflict keys

**Files:**
- Modify: `internal/ui/view.go` (`helpOverlay`, lines 85-105)
- Modify: `README.md:26-37` (Keys table)

No new behavior; docs only. No test step.

- [ ] **Step 1: Add help-overlay rows**

In `internal/ui/view.go`, in the `helpOverlay` string slice, add `"e …"` directly after the `"d               discard (confirm y)",` line:

```go
		"e               edit a conflicted file in your editor",
```

and add `"A …"` directly after the `"C               amend last commit (edit message, Ctrl-D send, Esc cancel)",` line:

```go
		"A               abort an in-progress merge (confirm y)",
```

- [ ] **Step 2: Add README key rows**

In `README.md`, in the Keys table, add two rows directly after the `| \`C\` | amend last commit (edit message, \`Ctrl-D\` to confirm) |` row:

```
| `e` | edit a conflicted file in your editor |
| `A` | abort an in-progress merge (confirm `y`) |
```

- [ ] **Step 3: Build (sanity) and confirm the suite still passes**

Run: `go build ./... && go test ./... `
Expected: PASS (docs change does not affect tests; this guards against a typo in the Go string slice).

- [ ] **Step 4: Commit**

```bash
git add internal/ui/view.go README.md
git commit -m "docs: document conflict keys (e edit, A abort)"
```

---

## Self-Review

**1. Spec coverage:**

| Spec requirement | Task |
|---|---|
| Capture the `u`-line conflict code | Task 1 (`FileStatus.Conflict`) |
| `MergeAbort` → `git merge --abort` | Task 2 |
| Merge detection via `MERGE_HEAD` | Task 2 (`Merging`) |
| Editor precedence + VS Code (`code --wait`) fallback | Task 3 (`chooseEditor`, `EditorCommand`) |
| Pure, testable editor chooser | Task 3 (`TestChooseEditor`) |
| Track merge state in the model from status | Task 4 |
| `MERGING — N conflicts` / `ready to commit` top-bar cue | Task 5 (`mergeBanner`, `topBar`) |
| Conflict footer hints on the Files panel while merging | Task 5 (`footerHints`) |
| Conflict-kind label in selected-context | Task 5 (`conflictLabel`, `selectedContextLines`) |
| `A` abort through the existing confirm gate | Task 6 |
| `e` open in editor via `tea.ExecProcess` | Task 7 (`openEditor`, `editConflict`) |
| `--wait` only on the built-in default; configured editors verbatim | Task 3 (`chooseEditor` returns the configured string as-is; `defaultEditor` carries `--wait`) |
| Editor launched through the platform shell, file appended | Task 7 (`editorExecCmd`) |
| Editor error → footer, refresh anyway; abort error via `gitDoneMsg` | Task 7 (`editorDoneMsg` case), Task 6 (`mergeAbort` via `mutation`) |
| `space` marks resolved (existing) | No change needed — unmerged files parse with `IsStaged()==false`, so `space` dispatches `git add`. |
| `c` finishes the merge (existing) | No change needed — existing commit flow; `loadStatus` then clears `merging`. |
| README/help reflect `e` and `A` | Task 8 |

No gaps. Explicit non-goals (no hunk editing, no 3-way view, no merge initiation, no rebase/cherry-pick, no `mergetool`, no loom editor config) are not implemented, as intended.

**2. Placeholder scan:** No TBD/TODO. Every code step shows complete code; every test step shows full assertions; every run step states the expected result.

**3. Type consistency:** `FileStatus.Conflict` (string) is set in Task 1 and read in Task 5 (`conflictLabel(f.Conflict)`). `Merging`/`MergeAbort`/`EditorCommand` signatures defined in Tasks 2-3 match their callers in Tasks 4, 6, 7. `statusLoadedMsg.merging` (Task 4) matches `loadStatus` and the reducer. `editorDoneMsg{err error}` (Task 7) matches `openEditor` and the reducer case. `mergeAbort(ctx, repo)` and `openEditor(ctx, repo, path)` signatures match their `update.go` call sites. Key constants `keyAbortMerge="A"` / `keyEditConflict="e"` match the `handleKey` cases. `mergeBanner`/`unmergedCount`/`editConflict` are each defined once and called consistently.
