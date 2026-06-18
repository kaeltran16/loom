# Stash Shelf Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a fourth Loom workflow panel for browsing, previewing, saving, applying, popping, and dropping Git stashes.

**Architecture:** Extend the existing `internal/git` repo API with typed stash methods and a pure stash-list parser. Extend the existing Bubble Tea MVU UI by adding `PanelStashes`, a small stash-message mode, and stash commands that reuse current panel sizing, viewport rendering, status rail, footer, confirmation, and command-log patterns.

**Tech Stack:** Go 1.22+, Bubble Tea, Bubbles `textinput`, Lipgloss, real `git` through `internal/git.Runner`.

---

## Scope Check

This plan implements one subsystem: Loom's stash shelf workflow. It does not implement partial stash, hunk stash, stash rename, branch-from-stash, stash search, a new diff renderer, or custom stash layout.

The current worktree may contain unrelated staged or modified files. Do not reset or clean them. When committing implementation steps, use pathspec commits that include only files touched by the task.

## File Structure

- Create `internal/git/stash.go`: stash data type, parser, and small branch/index helpers.
- Create `internal/git/stash_test.go`: parser tests for normal and unusual stash subjects.
- Modify `internal/git/repo.go`: typed stash repo methods that shell out through `Runner`.
- Modify `internal/git/repo_test.go`: repo command-argument and output tests for stash methods.
- Modify `internal/ui/model.go`: add `PanelStashes`, `ModeStashing`, stash state, and stash message input.
- Modify `internal/ui/messages.go`: add `stashesLoadedMsg`.
- Modify `internal/ui/commands.go`: add stash loading, preview, and mutation commands.
- Modify `internal/ui/keys.go`: add stash action key constants.
- Modify `internal/ui/update.go`: wire stash load results, panel navigation, stash message mode, and stash actions.
- Modify `internal/ui/panels.go`: add stash rows through existing `panelRows`/`renderPanel`.
- Modify `internal/ui/view.go`: add top-bar count, stash title, status rail selected context, footer hints, and stash message editor view through existing layout.
- Modify `internal/ui/update_test.go`: reducer tests for Stashes focus, message mode, apply/pop/drop, and refresh behavior.
- Modify `internal/ui/commands_test.go`: command tests for stash load/show and mutation output.
- Modify `internal/ui/panels_test.go`: stash row rendering tests.
- Modify `internal/ui/view_test.go`: stash title/footer/status rail/editor tests.
- Modify `README.md`: document Stashes key bindings.

---

### Task 1: Git Stash Model And Parser

**Files:**
- Create: `internal/git/stash.go`
- Create: `internal/git/stash_test.go`

- [ ] **Step 1: Write failing stash parser tests**

Create `internal/git/stash_test.go`:

```go
package git

import "testing"

func TestParseStashes(t *testing.T) {
	in := []byte("stash@{0}\x00WIP on main: abc1234 parser cleanup\x0012 minutes ago\nstash@{1}\x00On feature/login: before auth refactor\x00yesterday\n")

	got := parseStashes(in)

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2: %#v", len(got), got)
	}
	if got[0] != (Stash{
		Ref:     "stash@{0}",
		Index:   0,
		Message: "WIP on main: abc1234 parser cleanup",
		Branch:  "main",
		Age:     "12 minutes ago",
	}) {
		t.Fatalf("stash[0] = %#v", got[0])
	}
	if got[1] != (Stash{
		Ref:     "stash@{1}",
		Index:   1,
		Message: "On feature/login: before auth refactor",
		Branch:  "feature/login",
		Age:     "yesterday",
	}) {
		t.Fatalf("stash[1] = %#v", got[1])
	}
}

func TestParseStashesSkipsMalformedRows(t *testing.T) {
	in := []byte("stash@{0}\x00On main: good\x001 hour ago\nbad row without separators\nstash@{abc}\x00On main: bad index\x00now\n")

	got := parseStashes(in)

	if len(got) != 1 {
		t.Fatalf("len = %d, want 1: %#v", len(got), got)
	}
	if got[0].Ref != "stash@{0}" || got[0].Index != 0 || got[0].Branch != "main" {
		t.Fatalf("parsed stash = %#v", got[0])
	}
}

func TestStashBranchFromSubjectIsBestEffort(t *testing.T) {
	cases := map[string]string{
		"WIP on main: abc1234 work":        "main",
		"On feature/ui: save before pull":  "feature/ui",
		"custom hand-written stash title":  "",
		"WIP on release/v1.2: deadbee fix": "release/v1.2",
	}

	for subject, want := range cases {
		if got := stashBranchFromSubject(subject); got != want {
			t.Errorf("stashBranchFromSubject(%q) = %q, want %q", subject, got, want)
		}
	}
}
```

- [ ] **Step 2: Run parser tests and verify they fail**

Run:

```bash
go test ./internal/git -run 'TestParseStashes|TestStashBranchFromSubject' -count=1
```

Expected: FAIL with errors such as `undefined: parseStashes`, `undefined: Stash`, and `undefined: stashBranchFromSubject`.

- [ ] **Step 3: Implement the stash type and parser**

Create `internal/git/stash.go`:

```go
package git

import (
	"strconv"
	"strings"
)

// Stash is one entry from `git stash list`.
type Stash struct {
	Ref     string // stash@{0}
	Index   int
	Message string
	Branch  string
	Age     string
}

func parseStashes(out []byte) []Stash {
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	stashes := make([]Stash, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\x00")
		if len(parts) != 3 {
			continue
		}
		idx, ok := stashIndex(parts[0])
		if !ok {
			continue
		}
		msg := parts[1]
		stashes = append(stashes, Stash{
			Ref:     parts[0],
			Index:   idx,
			Message: msg,
			Branch:  stashBranchFromSubject(msg),
			Age:     parts[2],
		})
	}
	return stashes
}

func stashIndex(ref string) (int, bool) {
	if !strings.HasPrefix(ref, "stash@{") || !strings.HasSuffix(ref, "}") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(ref, "stash@{"), "}"))
	return n, err == nil
}

func stashBranchFromSubject(subject string) string {
	for _, prefix := range []string{"WIP on ", "On "} {
		if rest, ok := strings.CutPrefix(subject, prefix); ok {
			branch, _, ok := strings.Cut(rest, ":")
			if ok {
				return strings.TrimSpace(branch)
			}
		}
	}
	return ""
}
```

- [ ] **Step 4: Run parser tests and verify they pass**

Run:

```bash
go test ./internal/git -run 'TestParseStashes|TestStashBranchFromSubject' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit the parser**

Run:

```bash
git add internal/git/stash.go internal/git/stash_test.go
git commit -m "feat(git): parse stash list entries" -- internal/git/stash.go internal/git/stash_test.go
```

Expected: commit succeeds and includes only those two files.

---

### Task 2: Git Repo Stash Methods

**Files:**
- Modify: `internal/git/repo.go`
- Modify: `internal/git/repo_test.go`

- [ ] **Step 1: Write failing repo method tests**

Append these tests to `internal/git/repo_test.go`:

```go
func TestRepo_Stashes_callsArgsAndParses(t *testing.T) {
	fr := &fakeRunner{stdout: []byte("stash@{0}\x00On main: save point\x003 minutes ago\n")}
	repo := &Repo{runner: fr}

	got, err := repo.Stashes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"stash", "list", "--format=%gd%x00%gs%x00%cr"}
	if !reflect.DeepEqual(fr.gotArgs, wantArgs) {
		t.Errorf("args = %v, want %v", fr.gotArgs, wantArgs)
	}
	if len(got) != 1 || got[0].Ref != "stash@{0}" || got[0].Message != "On main: save point" {
		t.Errorf("stashes = %#v", got)
	}
}

func TestRepo_StashShow_args(t *testing.T) {
	fr := &fakeRunner{stdout: []byte("diff --git a/a.go b/a.go\n")}
	got, err := (&Repo{runner: fr}).StashShow(context.Background(), "stash@{2}")
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"stash", "show", "--patch", "--stat", "stash@{2}"}
	if !reflect.DeepEqual(fr.gotArgs, wantArgs) {
		t.Errorf("args = %v, want %v", fr.gotArgs, wantArgs)
	}
	if got != "diff --git a/a.go b/a.go\n" {
		t.Errorf("show = %q", got)
	}
}

func TestRepo_StashWriteMethodArgsAndOutput(t *testing.T) {
	cases := []struct {
		name string
		call func(*Repo) (string, error)
		want []string
	}{
		{"push", func(r *Repo) (string, error) { return r.StashPush(context.Background(), "save point") },
			[]string{"stash", "push", "-u", "-m", "save point"}},
		{"apply", func(r *Repo) (string, error) { return r.StashApply(context.Background(), "stash@{1}") },
			[]string{"stash", "apply", "stash@{1}"}},
		{"pop", func(r *Repo) (string, error) { return r.StashPop(context.Background(), "stash@{1}") },
			[]string{"stash", "pop", "stash@{1}"}},
		{"drop", func(r *Repo) (string, error) { return r.StashDrop(context.Background(), "stash@{1}") },
			[]string{"stash", "drop", "stash@{1}"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fr := &fakeRunner{stdout: []byte("stdout"), stderr: []byte("stderr")}
			out, err := c.call(&Repo{runner: fr})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(fr.gotArgs, c.want) {
				t.Errorf("args = %v, want %v", fr.gotArgs, c.want)
			}
			if out != "stdout\nstderr" {
				t.Errorf("output = %q, want combined stdout/stderr", out)
			}
		})
	}
}

func TestRepo_StashApplyFailureReturnsOutputAndError(t *testing.T) {
	fr := &fakeRunner{stdout: []byte("Auto-merging a.go\n"), stderr: []byte("CONFLICT (content): Merge conflict in a.go\n"), err: errors.New("exit status 1")}
	out, err := (&Repo{runner: fr}).StashApply(context.Background(), "stash@{0}")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(out, "Auto-merging") || !strings.Contains(out, "CONFLICT") {
		t.Fatalf("output = %q, want stdout and stderr", out)
	}
	if !strings.Contains(err.Error(), "git stash apply") {
		t.Fatalf("error = %v, want command context", err)
	}
}
```

Add `strings` to the existing import list in `repo_test.go`.

- [ ] **Step 2: Run repo stash method tests and verify they fail**

Run:

```bash
go test ./internal/git -run 'TestRepo_Stash' -count=1
```

Expected: FAIL with undefined repo methods.

- [ ] **Step 3: Implement stash methods in `repo.go`**

Add these methods near the other high-level repo methods in `internal/git/repo.go`:

```go
func (r *Repo) Stashes(ctx context.Context) ([]Stash, error) {
	out, errb, err := r.runner.Run(ctx, nil, "stash", "list", "--format=%gd%x00%gs%x00%cr")
	if err != nil {
		return nil, fmt.Errorf("git stash list: %w: %s", err, strings.TrimSpace(string(errb)))
	}
	return parseStashes(out), nil
}

func (r *Repo) StashShow(ctx context.Context, ref string) (string, error) {
	out, errb, err := r.runner.Run(ctx, nil, "stash", "show", "--patch", "--stat", ref)
	if err != nil {
		return "", fmt.Errorf("git stash show: %w: %s", err, strings.TrimSpace(string(errb)))
	}
	return string(out), nil
}

func (r *Repo) StashPush(ctx context.Context, message string) (string, error) {
	return r.stashOutput(ctx, "push", "-u", "-m", message)
}

func (r *Repo) StashApply(ctx context.Context, ref string) (string, error) {
	return r.stashOutput(ctx, "apply", ref)
}

func (r *Repo) StashPop(ctx context.Context, ref string) (string, error) {
	return r.stashOutput(ctx, "pop", ref)
}

func (r *Repo) StashDrop(ctx context.Context, ref string) (string, error) {
	return r.stashOutput(ctx, "drop", ref)
}

func (r *Repo) stashOutput(ctx context.Context, args ...string) (string, error) {
	out, errb, err := r.runner.Run(ctx, nil, append([]string{"stash"}, args...)...)
	combined := strings.TrimSpace(string(out) + "\n" + string(errb))
	if err != nil {
		return combined, fmt.Errorf("git stash %s: %w", args[0], err)
	}
	return combined, nil
}
```

- [ ] **Step 4: Run git package tests**

Run:

```bash
go test ./internal/git -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit repo stash methods**

Run:

```bash
git add internal/git/repo.go internal/git/repo_test.go
git commit -m "feat(git): add stash repo methods" -- internal/git/repo.go internal/git/repo_test.go
```

Expected: commit succeeds and includes only those two files.

---

### Task 3: UI Stashes Panel Skeleton

**Files:**
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/messages.go`
- Modify: `internal/ui/keys.go`
- Modify: `internal/ui/panels.go`
- Modify: `internal/ui/view.go`
- Modify: `internal/ui/update.go`
- Modify: `internal/ui/panels_test.go`
- Modify: `internal/ui/view_test.go`
- Modify: `internal/ui/update_test.go`

- [ ] **Step 1: Write failing panel/navigation/view tests**

Append to `internal/ui/panels_test.go`:

```go
func TestPanelLinesRenderStashes(t *testing.T) {
	m := newTestModel()
	m.stashes = []git.Stash{{Ref: "stash@{0}", Message: "On main: parser cleanup", Branch: "main", Age: "12 minutes ago"}}

	got := strings.Join(m.panelLines(PanelStashes), "\n")

	for _, want := range []string{"stash@{0}", "On main: parser cleanup", "12 minutes ago"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stash panel missing %q: %q", want, got)
		}
	}
}
```

Append to `internal/ui/view_test.go`:

```go
func TestTopBarIncludesStashesWorkflow(t *testing.T) {
	m := newTestModel()
	m.branch = git.BranchInfo{Name: "main"}
	m.stashes = []git.Stash{{Ref: "stash@{0}", Message: "On main: save point"}}
	m.focus = PanelStashes

	got := m.topBar()

	for _, want := range []string{"1 Files 0", "2 Branches 0", "3 Commits 0", "[4 Stashes 1]"} {
		if !strings.Contains(got, want) {
			t.Fatalf("topBar missing %q: %q", want, got)
		}
	}
}

func TestMainTitleForStashSelection(t *testing.T) {
	m := newTestModel()
	m.focus = PanelStashes
	m.stashes = []git.Stash{{Ref: "stash@{0}", Message: "On main: save point", Age: "3 minutes ago"}}

	if got := m.mainTitle(); got != "stash@{0} | On main: save point | 1 of 1" {
		t.Fatalf("mainTitle = %q", got)
	}
}
```

Append to `internal/ui/update_test.go`:

```go
func TestUpdate_FourFocusesStashes(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	if updated.(Model).focus != PanelStashes {
		t.Fatalf("focus = %v, want PanelStashes", updated.(Model).focus)
	}
}

func TestUpdate_TabCyclesThroughStashes(t *testing.T) {
	m := newTestModel()
	for _, want := range []Panel{PanelBranches, PanelCommits, PanelStashes, PanelFiles} {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = updated.(Model)
		if m.focus != want {
			t.Fatalf("focus = %v, want %v", m.focus, want)
		}
	}
}

func TestUpdate_StashesLoadedPopulatesModel(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(stashesLoadedMsg{stashes: []git.Stash{{Ref: "stash@{0}", Message: "On main: save"}}})
	got := updated.(Model)
	if len(got.stashes) != 1 || got.stashes[0].Ref != "stash@{0}" {
		t.Fatalf("stashes = %#v", got.stashes)
	}
}
```

- [ ] **Step 2: Run skeleton tests and verify they fail**

Run:

```bash
go test ./internal/ui -run 'TestPanelLinesRenderStashes|TestTopBarIncludesStashesWorkflow|TestMainTitleForStashSelection|TestUpdate_FourFocusesStashes|TestUpdate_TabCyclesThroughStashes|TestUpdate_StashesLoadedPopulatesModel' -count=1
```

Expected: FAIL with `undefined: PanelStashes`, `undefined: stashesLoadedMsg`, or missing stash rendering.

- [ ] **Step 3: Add `PanelStashes`, stash state, message, and key**

In `internal/ui/model.go`, update panel constants:

```go
const (
	PanelFiles Panel = iota
	PanelBranches
	PanelCommits
	PanelStashes
	panelCount
)
```

Add `stashes []git.Stash` to `Model` near `commits []git.Commit`.

In `internal/ui/messages.go`, add:

```go
type stashesLoadedMsg struct{ stashes []git.Stash }
```

In `internal/ui/keys.go`, add:

```go
	keyStashes      = "4"
```

- [ ] **Step 4: Wire panel rows and names**

In `internal/ui/panels.go`, update `emptyPanelLine`:

```go
	case PanelStashes:
		return "No stashes"
```

Update `panelName`:

```go
	case PanelStashes:
		return "Stashes"
```

Add:

```go
func stashLine(s git.Stash) string {
	label := s.Ref
	if s.Age != "" {
		label += " " + s.Age
	}
	if s.Message != "" {
		label += "  " + s.Message
	}
	return label
}
```

Update `panelRows`:

```go
	case PanelStashes:
		if len(m.stashes) == 0 {
			return []panelRow{{text: emptyPanelLine(p), kind: panelRowEmpty, itemIndex: -1}}
		}
		rows := make([]panelRow, len(m.stashes))
		for i, s := range m.stashes {
			rows[i] = panelRow{text: stashLine(s), kind: panelRowItem, itemIndex: i}
		}
		return rows
```

- [ ] **Step 5: Wire top bar, title, selected context, footer, and focus key**

In `internal/ui/view.go`, update `workflowTabs`:

```go
	tabs := []string{
		m.workflowTab(PanelFiles, "1", "Files", len(m.files)),
		m.workflowTab(PanelBranches, "2", "Branches", len(m.branches)),
		m.workflowTab(PanelCommits, "3", "Commits", len(m.commits)),
		m.workflowTab(PanelStashes, "4", "Stashes", len(m.stashes)),
	}
```

Add helper:

```go
func (m Model) selectedStash() (git.Stash, bool) {
	i := m.cursor[PanelStashes]
	if i < 0 || i >= len(m.stashes) {
		return git.Stash{}, false
	}
	return m.stashes[i], true
}
```

Update `selectedContextLines` with:

```go
	case PanelStashes:
		s, ok := m.selectedStash()
		if !ok {
			return []string{"No stash selected"}
		}
		lines := []string{s.Ref, s.Message}
		if s.Branch != "" || s.Age != "" {
			lines = append(lines, stashMeta(s))
		}
		return append(lines, "actions: s save, a apply, o pop, d drop")
```

Add:

```go
func stashMeta(s git.Stash) string {
	switch {
	case s.Branch != "" && s.Age != "":
		return s.Branch + " · " + s.Age
	case s.Branch != "":
		return s.Branch
	default:
		return s.Age
	}
}
```

Update `mainTitle`:

```go
	case PanelStashes:
		s, ok := m.selectedStash()
		if !ok {
			return "No stashes"
		}
		pos, total := m.selectedStashPosition()
		return fmt.Sprintf("%s | %s | %d of %d", s.Ref, s.Message, pos, total)
```

Add:

```go
func (m Model) selectedStashPosition() (int, int) {
	if len(m.stashes) == 0 {
		return 0, 0
	}
	i := m.cursor[PanelStashes]
	if i < 0 {
		i = 0
	}
	if i >= len(m.stashes) {
		i = len(m.stashes) - 1
	}
	return i + 1, len(m.stashes)
}
```

Update `emptyMainBody`:

```go
	case PanelStashes:
		return "No stash preview to show."
```

Update `footerHints`:

```go
	case PanelStashes:
		return "Stashes", []keyHint{{"s", "save"}, {"a", "apply"}, {"o", "pop"}, {"d", "drop"}, {"?", "help"}, {"q", "quit"}}
```

In `internal/ui/update.go`, add `stashesLoadedMsg` handling:

```go
	case stashesLoadedMsg:
		m.stashes = msg.stashes
		return m, nil
```

Add key handling:

```go
	case keyStashes:
		m.focus = PanelStashes
		m.mainFocused = false
		return m.reloadMain()
```

Update `focusLen`:

```go
	case PanelStashes:
		return len(m.stashes)
```

- [ ] **Step 6: Run skeleton tests and full UI tests**

Run:

```bash
go test ./internal/ui -run 'TestPanelLinesRenderStashes|TestTopBarIncludesStashesWorkflow|TestMainTitleForStashSelection|TestUpdate_FourFocusesStashes|TestUpdate_TabCyclesThroughStashes|TestUpdate_StashesLoadedPopulatesModel' -count=1
go test ./internal/ui -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit panel skeleton**

Run:

```bash
git add internal/ui/model.go internal/ui/messages.go internal/ui/keys.go internal/ui/panels.go internal/ui/view.go internal/ui/update.go internal/ui/panels_test.go internal/ui/view_test.go internal/ui/update_test.go
git commit -m "feat(ui): add stashes workflow panel" -- internal/ui/model.go internal/ui/messages.go internal/ui/keys.go internal/ui/panels.go internal/ui/view.go internal/ui/update.go internal/ui/panels_test.go internal/ui/view_test.go internal/ui/update_test.go
```

Expected: commit succeeds and includes only those files.

---

### Task 4: Load And Preview Stashes

**Files:**
- Modify: `internal/ui/commands.go`
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/update.go`
- Modify: `internal/ui/commands_test.go`
- Modify: `internal/ui/update_test.go`

- [ ] **Step 1: Write failing command and reducer tests**

Append to `internal/ui/commands_test.go`:

```go
func TestLoadStashesCmd_returnsStashesLoadedMsg(t *testing.T) {
	repo := git.NewTestRepo(&git.StubRunner{Stdout: []byte("stash@{0}\x00On main: save\x003 minutes ago\n")})

	msg := loadStashes(context.Background(), repo)()

	got, ok := msg.(stashesLoadedMsg)
	if !ok {
		t.Fatalf("want stashesLoadedMsg, got %T", msg)
	}
	if len(got.stashes) != 1 || got.stashes[0].Ref != "stash@{0}" {
		t.Fatalf("payload = %#v", got.stashes)
	}
}

func TestLoadStashShowCmd_returnsDiffLoadedMsg(t *testing.T) {
	repo := git.NewTestRepo(&git.StubRunner{Stdout: []byte("diff --git a/a.go b/a.go\n")})

	msg := loadStashShow(context.Background(), repo, "stash@{0}", 9)().(diffLoadedMsg)

	if msg.seq != 9 {
		t.Fatalf("seq = %d, want 9", msg.seq)
	}
	if msg.text != "diff --git a/a.go b/a.go\n" {
		t.Fatalf("text = %q", msg.text)
	}
}
```

Append to `internal/ui/update_test.go`:

```go
func TestInitLoadsStashes(t *testing.T) {
	m := newTestModel()
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init should return a batch command")
	}
	// Bubble Tea batches are opaque; this test protects the intended contract by
	// checking loadStashes itself and by keeping Init non-nil.
}

func TestLoadMainForSelectionLoadsStashPreview(t *testing.T) {
	m := newTestModel()
	m.focus = PanelStashes
	m.stashes = []git.Stash{{Ref: "stash@{0}", Message: "On main: save"}}
	m.reqSeq = 3

	cmd := m.loadMainForSelection()

	if cmd == nil {
		t.Fatal("expected stash preview load command")
	}
	msg, ok := cmd().(diffLoadedMsg)
	if !ok {
		t.Fatalf("expected diffLoadedMsg, got %T", cmd())
	}
	if msg.seq != 3 {
		t.Fatalf("seq = %d, want 3", msg.seq)
	}
}
```

- [ ] **Step 2: Run targeted tests and verify they fail**

Run:

```bash
go test ./internal/ui -run 'TestLoadStashesCmd|TestLoadStashShowCmd|TestLoadMainForSelectionLoadsStashPreview' -count=1
```

Expected: FAIL with undefined `loadStashes` and `loadStashShow`, or nil stash preview command.

- [ ] **Step 3: Implement stash load commands**

In `internal/ui/commands.go`, add:

```go
func loadStashes(ctx context.Context, repo *git.Repo) tea.Cmd {
	return func() tea.Msg {
		stashes, err := repo.Stashes(ctx)
		if err != nil {
			return errMsg{err}
		}
		return stashesLoadedMsg{stashes: stashes}
	}
}

func loadStashShow(ctx context.Context, repo *git.Repo, ref string, seq int) tea.Cmd {
	return func() tea.Msg {
		text, err := repo.StashShow(ctx, ref)
		if err != nil {
			return errMsg{err}
		}
		return diffLoadedMsg{text: text, seq: seq}
	}
}
```

In `internal/ui/model.go`, update `Init`:

```go
	return tea.Batch(
		loadStatus(m.ctx, m.repo),
		loadBranches(m.ctx, m.repo),
		loadCommits(m.ctx, m.repo),
		loadStashes(m.ctx, m.repo),
		m.spinner.Tick,
	)
```

In `internal/ui/update.go`, update `loadMainForSelection`:

```go
	case PanelStashes:
		if i := m.cursor[PanelStashes]; i < len(m.stashes) {
			return loadStashShow(m.ctx, m.repo, m.stashes[i].Ref, m.reqSeq)
		}
```

- [ ] **Step 4: Run targeted and full UI tests**

Run:

```bash
go test ./internal/ui -run 'TestLoadStashesCmd|TestLoadStashShowCmd|TestLoadMainForSelectionLoadsStashPreview' -count=1
go test ./internal/ui -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit stash loading**

Run:

```bash
git add internal/ui/commands.go internal/ui/model.go internal/ui/update.go internal/ui/commands_test.go internal/ui/update_test.go
git commit -m "feat(ui): load and preview stashes" -- internal/ui/commands.go internal/ui/model.go internal/ui/update.go internal/ui/commands_test.go internal/ui/update_test.go
```

Expected: commit succeeds and includes only those files.

---

### Task 5: Stash Save Message Mode

**Files:**
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/commands.go`
- Modify: `internal/ui/update.go`
- Modify: `internal/ui/view.go`
- Modify: `internal/ui/commands_test.go`
- Modify: `internal/ui/update_test.go`
- Modify: `internal/ui/view_test.go`

- [ ] **Step 1: Write failing save-mode tests**

Append to `internal/ui/commands_test.go`:

```go
func TestStashPushCmd_returnsOutputAndNotice(t *testing.T) {
	repo := git.NewTestRepo(&git.StubRunner{Stdout: []byte("Saved working directory and index state On main: save\n")})

	msg := stashPush(context.Background(), repo, "save")().(gitDoneMsg)

	if msg.cmd != "git stash push" {
		t.Fatalf("cmd = %q", msg.cmd)
	}
	if msg.output == "" {
		t.Fatal("expected stash output in command log")
	}
	if msg.notice != "Stashed save" {
		t.Fatalf("notice = %q", msg.notice)
	}
	if msg.err != nil {
		t.Fatalf("unexpected error: %v", msg.err)
	}
}
```

Append to `internal/ui/update_test.go`:

```go
func TestUpdate_SInStashesEntersStashingMode(t *testing.T) {
	m := newTestModel()
	m.focus = PanelStashes

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	got := updated.(Model)

	if got.mode != ModeStashing {
		t.Fatalf("mode = %v, want ModeStashing", got.mode)
	}
	if cmd != nil {
		t.Fatal("opening stash message mode should not dispatch a command")
	}
}

func TestUpdate_StashCtrlDDispatchesPush(t *testing.T) {
	m := newTestModel()
	m.mode = ModeStashing
	m.stashMessage.SetValue("save point")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	got := updated.(Model)

	if got.mode != ModeNormal {
		t.Fatalf("mode = %v, want ModeNormal", got.mode)
	}
	if !got.busy {
		t.Fatal("expected busy=true after stash submit")
	}
	if cmd == nil {
		t.Fatal("expected stash push command")
	}
	done := cmd().(gitDoneMsg)
	if done.cmd != "git stash push" {
		t.Fatalf("cmd = %q", done.cmd)
	}
}

func TestUpdate_StashEmptyMessageIsNoop(t *testing.T) {
	m := newTestModel()
	m.mode = ModeStashing

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})

	if updated.(Model).mode != ModeStashing {
		t.Fatal("empty stash message should stay in stash editor")
	}
	if cmd != nil {
		t.Fatal("empty stash message should not dispatch")
	}
}

func TestUpdate_StashEscCancels(t *testing.T) {
	m := newTestModel()
	m.mode = ModeStashing
	m.stashMessage.SetValue("save point")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)

	if got.mode != ModeNormal {
		t.Fatalf("mode = %v, want ModeNormal", got.mode)
	}
	if got.stashMessage.Value() != "" {
		t.Fatalf("stash message = %q, want reset", got.stashMessage.Value())
	}
	if cmd != nil {
		t.Fatal("cancel should not dispatch")
	}
}
```

Append to `internal/ui/view_test.go`:

```go
func TestStashEditorView(t *testing.T) {
	m := newTestModel()
	m.w, m.h = 120, 40
	m.layout()
	m.mode = ModeStashing
	m.stashMessage.SetValue("save point")

	got := m.View()

	for _, want := range []string{"Save stash", "Message", "save point", "Ctrl-D save", "Esc cancel"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stash editor missing %q:\n%s", want, got)
		}
	}
}
```

- [ ] **Step 2: Run save-mode tests and verify they fail**

Run:

```bash
go test ./internal/ui -run 'TestStashPushCmd|TestUpdate_SInStashesEntersStashingMode|TestUpdate_StashCtrlDDispatchesPush|TestUpdate_StashEmptyMessageIsNoop|TestUpdate_StashEscCancels|TestStashEditorView' -count=1
```

Expected: FAIL with undefined mode, field, command, or editor view.

- [ ] **Step 3: Add stash message model state**

In `internal/ui/model.go`, add mode:

```go
	ModeStashing
```

Add field to `Model`:

```go
	stashMessage textinput.Model
```

In `NewModel`, initialize it:

```go
	stashMsg := textinput.New()
	stashMsg.Placeholder = "Stash message..."
```

Return it:

```go
		stashMessage: stashMsg,
```

- [ ] **Step 4: Add stash push command**

In `internal/ui/commands.go`, add:

```go
func stashPush(ctx context.Context, repo *git.Repo, message string) tea.Cmd {
	return func() tea.Msg {
		out, err := repo.StashPush(ctx, message)
		if err != nil {
			return gitDoneMsg{cmd: "git stash push", output: out, err: err}
		}
		return gitDoneMsg{cmd: "git stash push", output: out, notice: "Stashed " + strings.TrimSpace(message)}
	}
}
```

`commands.go` already imports `strings`; reuse it.

- [ ] **Step 5: Wire stash mode update behavior**

In `internal/ui/update.go`, handle `ModeStashing` before normal mode:

```go
	if m.mode == ModeStashing {
		switch msg.Type {
		case tea.KeyCtrlD:
			return m.submitStash()
		case tea.KeyEsc:
			m.resetStashMessage()
			return m, nil
		}
		var cmd tea.Cmd
		m.stashMessage, cmd = m.stashMessage.Update(msg)
		return m, cmd
	}
```

Add normal key handling:

```go
	case keyStashSave:
		if m.focus != PanelStashes {
			return m, nil
		}
		m.mode = ModeStashing
		m.stashMessage.Focus()
		return m, nil
```

Add helper methods:

```go
func (m *Model) resetStashMessage() {
	m.stashMessage.Reset()
	m.stashMessage.Blur()
	m.mode = ModeNormal
}

func (m Model) submitStash() (tea.Model, tea.Cmd) {
	message := strings.TrimSpace(m.stashMessage.Value())
	if message == "" {
		return m, nil
	}
	m.resetStashMessage()
	m.busy = true
	return m, stashPush(m.ctx, m.repo, message)
}
```

Ensure `update.go` already imports `strings`; it does.

In `internal/ui/keys.go`, add:

```go
	keyStashSave  = "s"
```

- [ ] **Step 6: Add stash editor view and footer**

In `internal/ui/view.go`, update `mainContent` before normal title rendering:

```go
	if m.mode == ModeStashing {
		return m.stashEditorView()
	}
```

Add:

```go
func (m Model) stashEditorView() string {
	heading := strongStyle.Render("Save stash")
	label := accentStyle.Render("Message")
	width := m.viewport.Width
	if width < 1 {
		width = m.w
	}
	vm := m
	vm.stashMessage.Width = width - 2
	if vm.stashMessage.Width < 1 {
		vm.stashMessage.Width = 1
	}
	return strings.Join([]string{
		heading,
		mutedStyle.Render("Save current tracked and untracked work"),
		"",
		label,
		vm.stashMessage.View(),
		"",
		mutedStyle.Render("Ctrl-D save · Esc cancel"),
	}, "\n")
}
```

Update `footerHints`:

```go
	case ModeStashing:
		return "Stash", []keyHint{{"ctrl+d", "save"}, {"esc", "cancel"}}
```

- [ ] **Step 7: Run save-mode tests and full UI tests**

Run:

```bash
go test ./internal/ui -run 'TestStashPushCmd|TestUpdate_SInStashesEntersStashingMode|TestUpdate_StashCtrlDDispatchesPush|TestUpdate_StashEmptyMessageIsNoop|TestUpdate_StashEscCancels|TestStashEditorView' -count=1
go test ./internal/ui -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit stash save mode**

Run:

```bash
git add internal/ui/model.go internal/ui/commands.go internal/ui/update.go internal/ui/view.go internal/ui/commands_test.go internal/ui/update_test.go internal/ui/view_test.go
git commit -m "feat(ui): save named stashes" -- internal/ui/model.go internal/ui/commands.go internal/ui/update.go internal/ui/view.go internal/ui/commands_test.go internal/ui/update_test.go internal/ui/view_test.go
```

Expected: commit succeeds and includes only those files.

---

### Task 6: Apply, Pop, Drop, And Refresh Semantics

**Files:**
- Modify: `internal/ui/commands.go`
- Modify: `internal/ui/update.go`
- Modify: `internal/ui/keys.go`
- Modify: `internal/ui/commands_test.go`
- Modify: `internal/ui/update_test.go`

- [ ] **Step 1: Write failing stash action tests**

Append to `internal/ui/commands_test.go`:

```go
func TestStashActionCommandsKeepOutput(t *testing.T) {
	cases := []struct {
		name string
		cmd  tea.Cmd
		want string
	}{
		{"apply", stashApply(context.Background(), git.NewTestRepo(&git.StubRunner{Stdout: []byte("applied\n")}), "stash@{0}"), "git stash apply stash@{0}"},
		{"pop", stashPop(context.Background(), git.NewTestRepo(&git.StubRunner{Stdout: []byte("popped\n")}), "stash@{0}"), "git stash pop stash@{0}"},
		{"drop", stashDrop(context.Background(), git.NewTestRepo(&git.StubRunner{Stdout: []byte("dropped\n")}), "stash@{0}"), "git stash drop stash@{0}"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			msg := c.cmd().(gitDoneMsg)
			if msg.cmd != c.want {
				t.Fatalf("cmd = %q, want %q", msg.cmd, c.want)
			}
			if msg.output == "" {
				t.Fatal("expected output preserved")
			}
			if msg.err != nil {
				t.Fatalf("unexpected error: %v", msg.err)
			}
		})
	}
}
```

If `commands_test.go` does not already import Bubble Tea after adding this test, add:

```go
tea "github.com/charmbracelet/bubbletea"
```

Append to `internal/ui/update_test.go`:

```go
func TestUpdate_AAppliesSelectedStash(t *testing.T) {
	m := newTestModel()
	m.focus = PanelStashes
	m.stashes = []git.Stash{{Ref: "stash@{0}", Message: "On main: save"}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})

	if !updated.(Model).busy {
		t.Fatal("expected busy=true on apply")
	}
	if cmd == nil {
		t.Fatal("expected apply command")
	}
	if done := cmd().(gitDoneMsg); done.cmd != "git stash apply stash@{0}" {
		t.Fatalf("cmd = %q", done.cmd)
	}
}

func TestUpdate_OPopsSelectedStash(t *testing.T) {
	m := newTestModel()
	m.focus = PanelStashes
	m.stashes = []git.Stash{{Ref: "stash@{0}", Message: "On main: save"}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})

	if !updated.(Model).busy {
		t.Fatal("expected busy=true on pop")
	}
	if cmd == nil {
		t.Fatal("expected pop command")
	}
	if done := cmd().(gitDoneMsg); done.cmd != "git stash pop stash@{0}" {
		t.Fatalf("cmd = %q", done.cmd)
	}
}

func TestUpdate_DInStashesConfirmsDrop(t *testing.T) {
	m := newTestModel()
	m.focus = PanelStashes
	m.stashes = []git.Stash{{Ref: "stash@{0}", Message: "On main: save"}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	got := updated.(Model)

	if got.mode != ModeConfirming {
		t.Fatalf("mode = %v, want ModeConfirming", got.mode)
	}
	if got.confirm.action == nil {
		t.Fatal("expected drop action stored for confirmation")
	}
	if cmd != nil {
		t.Fatal("drop should not dispatch before confirmation")
	}
}

func TestUpdate_StashActionIgnoredOutsideStashes(t *testing.T) {
	for _, key := range []string{"a", "o"} {
		m := newTestModel()
		m.focus = PanelFiles
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		if updated.(Model).busy || cmd != nil {
			t.Fatalf("key %q outside Stashes should be ignored", key)
		}
	}
}

func TestUpdate_StashActionWithNoSelectionIsNoop(t *testing.T) {
	for _, key := range []string{"a", "o", "d"} {
		m := newTestModel()
		m.focus = PanelStashes
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		if updated.(Model).busy || updated.(Model).mode != ModeNormal || cmd != nil {
			t.Fatalf("key %q with no stash should be ignored", key)
		}
	}
}
```

- [ ] **Step 2: Run action tests and verify they fail**

Run:

```bash
go test ./internal/ui -run 'TestStashActionCommandsKeepOutput|TestUpdate_AAppliesSelectedStash|TestUpdate_OPopsSelectedStash|TestUpdate_DInStashesConfirmsDrop|TestUpdate_StashAction' -count=1
```

Expected: FAIL with undefined stash action commands or no update wiring.

- [ ] **Step 3: Add keys and command helpers**

In `internal/ui/keys.go`, add:

```go
	keyStashApply = "a"
	keyStashPop   = "o"
	keyStashDrop  = "d"
```

The existing `keyDiscard = "d"` can remain. If duplicate constants feel confusing, keep `keyDiscard` for Files and use `keyStashDrop` in Stashes; both values are `"d"`.

In `internal/ui/commands.go`, add:

```go
func stashApply(ctx context.Context, repo *git.Repo, ref string) tea.Cmd {
	return stashMutation("git stash apply "+ref, func() (string, error) { return repo.StashApply(ctx, ref) })
}

func stashPop(ctx context.Context, repo *git.Repo, ref string) tea.Cmd {
	return stashMutation("git stash pop "+ref, func() (string, error) { return repo.StashPop(ctx, ref) })
}

func stashDrop(ctx context.Context, repo *git.Repo, ref string) tea.Cmd {
	return stashMutation("git stash drop "+ref, func() (string, error) { return repo.StashDrop(ctx, ref) })
}

func stashMutation(label string, fn func() (string, error)) tea.Cmd {
	return func() tea.Msg {
		out, err := fn()
		return gitDoneMsg{cmd: label, output: out, err: err}
	}
}
```

- [ ] **Step 4: Wire update actions**

In `internal/ui/update.go`, add normal key handling before generic `keyDiscard` handling, or branch inside `keyDiscard` so Files discard and Stashes drop do not conflict:

```go
	case keyStashApply:
		return m.applySelectedStash()
	case keyStashPop:
		return m.popSelectedStash()
```

Change the existing `case keyDiscard:` block to:

```go
	case keyDiscard:
		if m.focus == PanelStashes {
			return m.dropSelectedStash()
		}
		return m.discardSelected()
```

Add helpers:

```go
func (m Model) applySelectedStash() (tea.Model, tea.Cmd) {
	if m.focus != PanelStashes {
		return m, nil
	}
	s, ok := m.selectedStash()
	if !ok {
		return m, nil
	}
	m.busy = true
	return m, stashApply(m.ctx, m.repo, s.Ref)
}

func (m Model) popSelectedStash() (tea.Model, tea.Cmd) {
	if m.focus != PanelStashes {
		return m, nil
	}
	s, ok := m.selectedStash()
	if !ok {
		return m, nil
	}
	m.busy = true
	return m, stashPop(m.ctx, m.repo, s.Ref)
}

func (m Model) dropSelectedStash() (tea.Model, tea.Cmd) {
	if m.focus != PanelStashes {
		return m, nil
	}
	s, ok := m.selectedStash()
	if !ok {
		return m, nil
	}
	m.mode = ModeConfirming
	m.confirm = confirmReq{
		prompt: "Drop " + s.Ref + "? [y/n]",
		action: stashDrop(m.ctx, m.repo, s.Ref),
	}
	return m, nil
}
```

- [ ] **Step 5: Refresh stashes after successful mutations**

In `internal/ui/update.go`, update successful `gitDoneMsg` refresh batch:

```go
		return m, tea.Batch(
			loadStatus(m.ctx, m.repo),
			loadBranches(m.ctx, m.repo),
			loadCommits(m.ctx, m.repo),
			loadStashes(m.ctx, m.repo),
		)
```

This refreshes Stashes for stash actions and is harmless for existing mutations.

- [ ] **Step 6: Run action and full UI tests**

Run:

```bash
go test ./internal/ui -run 'TestStashActionCommandsKeepOutput|TestUpdate_AAppliesSelectedStash|TestUpdate_OPopsSelectedStash|TestUpdate_DInStashesConfirmsDrop|TestUpdate_StashAction' -count=1
go test ./internal/ui -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit stash actions**

Run:

```bash
git add internal/ui/commands.go internal/ui/update.go internal/ui/keys.go internal/ui/commands_test.go internal/ui/update_test.go
git commit -m "feat(ui): apply pop and drop stashes" -- internal/ui/commands.go internal/ui/update.go internal/ui/keys.go internal/ui/commands_test.go internal/ui/update_test.go
```

Expected: commit succeeds and includes only those files.

---

### Task 7: Stash UX Polish And Documentation

**Files:**
- Modify: `internal/ui/view.go`
- Modify: `internal/ui/view_test.go`
- Modify: `README.md`

- [ ] **Step 1: Write failing UX tests**

Append to `internal/ui/view_test.go`:

```go
func TestFooterStashesHints(t *testing.T) {
	m := newTestModel()
	m.focus = PanelStashes
	m.stashes = []git.Stash{{Ref: "stash@{0}", Message: "On main: save"}}

	if got := m.footerActions(); got != "Stashes: s save · a apply · o pop · d drop · ? help · q quit" {
		t.Fatalf("footerActions = %q", got)
	}
}

func TestSelectedContextLinesShowsStashActions(t *testing.T) {
	m := newTestModel()
	m.focus = PanelStashes
	m.stashes = []git.Stash{{Ref: "stash@{0}", Message: "On main: save", Branch: "main", Age: "3 minutes ago"}}

	got := strings.Join(m.selectedContextLines(), "\n")

	for _, want := range []string{"stash@{0}", "On main: save", "main", "3 minutes ago", "actions: s save, a apply, o pop, d drop"} {
		if !strings.Contains(got, want) {
			t.Fatalf("selected context missing %q:\n%s", want, got)
		}
	}
}

func TestViewStashesUsesNormalLayoutBounds(t *testing.T) {
	m := newTestModel()
	m.w = 120
	m.h = 40
	m.layout()
	m.branch = git.BranchInfo{Name: "main"}
	m.focus = PanelStashes
	m.stashes = []git.Stash{{Ref: "stash@{0}", Message: "On main: save", Age: "3 minutes ago"}}
	m.viewport.SetContent("diff --git a/a.go b/a.go\n+new\n")

	got := m.View()

	if height := lipgloss.Height(got); height > m.h {
		t.Fatalf("View height = %d, want <= %d", height, m.h)
	}
	if width := lipgloss.Width(got); width > m.w {
		t.Fatalf("View width = %d, want <= %d", width, m.w)
	}
	for _, want := range []string{"[4 Stashes 1]", "stash@{0}", "Status Rail", "apply"} {
		if !strings.Contains(got, want) {
			t.Fatalf("View missing %q:\n%s", want, got)
		}
	}
}
```

- [ ] **Step 2: Run UX tests and verify current gaps**

Run:

```bash
go test ./internal/ui -run 'TestFooterStashesHints|TestSelectedContextLinesShowsStashActions|TestViewStashesUsesNormalLayoutBounds' -count=1
```

Expected: FAIL only for missing or mismatched UX strings if Task 3 implemented partial wording.

- [ ] **Step 3: Tighten UI copy**

In `internal/ui/view.go`, make sure stash footer, selected context, and title are exactly:

```go
	case PanelStashes:
		return "Stashes", []keyHint{{"s", "save"}, {"a", "apply"}, {"o", "pop"}, {"d", "drop"}, {"?", "help"}, {"q", "quit"}}
```

Make sure selected context action line is:

```go
	return append(lines, "actions: s save, a apply, o pop, d drop")
```

Keep all layout inside current `View`, `renderPanel`, `mainContent`, and `renderStatusRail` helpers. Do not add a stash-specific pane size or branch around `railVisible`, `listOuter`, or `mainOuter`.

- [ ] **Step 4: Update README key table**

In `README.md`, update the key table with:

```markdown
| `4` | focus Stashes |
| `s` | save current work as a stash from Stashes |
| `a` / `o` / `d` | apply / pop / drop selected stash |
```

If the table uses combined focus keys, rewrite the focus row as:

```markdown
| `1` `2` `3` `4` / `Tab` | focus Files / Branches / Commits / Stashes |
```

- [ ] **Step 5: Run UX tests and README-free full tests**

Run:

```bash
go test ./internal/ui -run 'TestFooterStashesHints|TestSelectedContextLinesShowsStashActions|TestViewStashesUsesNormalLayoutBounds' -count=1
go test ./... -count=1
```

Expected: PASS. If unrelated dirty changes in the worktree break `go test ./...`, capture the failing package and rerun the stash-relevant packages:

```bash
go test ./internal/git ./internal/ui -count=1
```

Expected: PASS for stash-relevant packages.

- [ ] **Step 6: Commit UX polish and docs**

Run:

```bash
git add internal/ui/view.go internal/ui/view_test.go README.md
git commit -m "docs(ui): document stash workflow" -- internal/ui/view.go internal/ui/view_test.go README.md
```

Expected: commit succeeds and includes only those files.

---

### Task 8: Final Verification

**Files:**
- Read: `docs/superpowers/specs/2026-06-18-stash-shelf-design.md`
- Verify: all changed source and test files

- [ ] **Step 1: Run full automated tests**

Run:

```bash
go test ./... -count=1
```

Expected: PASS.

If unrelated pre-existing worktree changes break the full suite, do not alter them. Record the failing output and run the stash-relevant subset:

```bash
go test ./internal/git ./internal/ui -count=1
```

Expected: PASS.

- [ ] **Step 2: Build Loom**

Run:

```bash
go build -o loom.exe .
```

Expected: command exits 0 and produces `loom.exe`.

- [ ] **Step 3: Manual smoke test in a disposable repo**

Run from a temporary directory outside this repo:

```bash
mkdir C:\tmp\loom-stash-smoke
cd C:\tmp\loom-stash-smoke
git init
git config user.name "Loom Smoke"
git config user.email "loom-smoke@example.invalid"
"hello" | Set-Content README.md
git add README.md
git commit -m "init"
"change" | Add-Content README.md
"new" | Set-Content scratch.txt
C:\Users\kael02\IdeaProjects\loom\loom.exe
```

In Loom:

- press `4` to focus Stashes;
- press `s`, type `smoke save`, press `Ctrl-D`;
- confirm the stash count increases and Files becomes clean;
- select the stash and confirm the main pane shows a diff;
- press `a` and confirm Files shows changes again while the stash remains;
- press `4`, press `d`, press `y`, and confirm the stash is dropped.

Expected: no panic, layout stays within the terminal, and the stash preview uses the existing diff viewport coloring.

- [ ] **Step 4: Inspect final diff for scope**

Run:

```bash
git diff --stat HEAD
git diff --name-only HEAD
```

Expected: changes are limited to stash shelf source, tests, and README. No unrelated staged or modified files are included in stash feature commits.

- [ ] **Step 5: Final implementation summary**

Report:

- tests run and pass/fail status;
- build result;
- manual smoke result or why it was skipped;
- any unrelated pre-existing worktree changes that were intentionally left untouched.
