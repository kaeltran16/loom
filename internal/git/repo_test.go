package git

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestRepo_Status_callsArgsAndParses(t *testing.T) {
	fr := &fakeRunner{stdout: []byte("# branch.head main\n1 M. N... 100644 100644 100644 1 2 src/app.go\n")}
	repo := &Repo{runner: fr}

	files, br, err := repo.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"status", "--porcelain=v2", "--branch"}
	if !reflect.DeepEqual(fr.gotArgs, wantArgs) {
		t.Errorf("args = %v, want %v", fr.gotArgs, wantArgs)
	}
	if br.Name != "main" || len(files) != 1 || files[0].Path != "src/app.go" {
		t.Errorf("parse wiring wrong: %+v %+v", br, files)
	}
}

func TestRepo_Diff_unstagedArgs(t *testing.T) {
	fr := &fakeRunner{stdout: []byte("diff text")}
	repo := &Repo{runner: fr}
	got, err := repo.Diff(context.Background(), "src/app.go", false)
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"diff", "--", "src/app.go"}
	if !reflect.DeepEqual(fr.gotArgs, wantArgs) {
		t.Errorf("args = %v, want %v", fr.gotArgs, wantArgs)
	}
	if got != "diff text" {
		t.Errorf("diff = %q", got)
	}
}

func TestRepo_Diff_stagedArgs(t *testing.T) {
	fr := &fakeRunner{}
	repo := &Repo{runner: fr}
	_, _ = repo.Diff(context.Background(), "src/app.go", true)
	wantArgs := []string{"diff", "--cached", "--", "src/app.go"}
	if !reflect.DeepEqual(fr.gotArgs, wantArgs) {
		t.Errorf("args = %v, want %v", fr.gotArgs, wantArgs)
	}
}

func TestRepo_WriteMethodArgs(t *testing.T) {
	cases := []struct {
		name string
		call func(*Repo) error
		want []string
	}{
		{"stage", func(r *Repo) error { return r.Stage(context.Background(), "f.go") },
			[]string{"add", "--", "f.go"}},
		{"unstage", func(r *Repo) error { return r.Unstage(context.Background(), "f.go") },
			[]string{"restore", "--staged", "--", "f.go"}},
		{"discard", func(r *Repo) error { return r.Discard(context.Background(), "f.go") },
			[]string{"restore", "--", "f.go"}},
		{"switch", func(r *Repo) error { return r.Switch(context.Background(), "main") },
			[]string{"switch", "main"}},
		{"fetch", func(r *Repo) error { _, e := r.Fetch(context.Background()); return e },
			[]string{"fetch"}},
		{"pull", func(r *Repo) error { _, e := r.Pull(context.Background()); return e },
			[]string{"pull"}},
		{"push", func(r *Repo) error { _, e := r.Push(context.Background()); return e },
			[]string{"push"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fr := &fakeRunner{}
			if err := c.call(&Repo{runner: fr}); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(fr.gotArgs, c.want) {
				t.Errorf("args = %v, want %v", fr.gotArgs, c.want)
			}
		})
	}
}

func TestRepo_CommitAll_stagesCommitsThenResolvesHash(t *testing.T) {
	fr := &fakeRunner{stdout: []byte("a1b2c3d\n")}
	hash, err := (&Repo{runner: fr}).CommitAll(context.Background(), "my message")
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"add", "-A"},
		{"commit", "-F", "-"},
		{"rev-parse", "--short", "HEAD"},
	}
	if !reflect.DeepEqual(fr.calls, want) {
		t.Errorf("calls = %v, want %v", fr.calls, want)
	}
	if string(fr.gotIn) != "my message" {
		t.Errorf("stdin = %q, want %q", fr.gotIn, "my message")
	}
	if hash != "a1b2c3d" {
		t.Errorf("hash = %q, want a1b2c3d", hash)
	}
}

func TestRepo_CommitAll_doesNotCommitWhenStageFails(t *testing.T) {
	fr := &fakeRunner{err: errors.New("boom")}
	_, err := (&Repo{runner: fr}).CommitAll(context.Background(), "msg")
	if err == nil {
		t.Fatal("expected error when staging fails")
	}
	if len(fr.calls) != 1 {
		t.Errorf("expected only the failed `add` call, got %v", fr.calls)
	}
}

func TestRepo_Commit_passesMessageThenResolvesHash(t *testing.T) {
	fr := &fakeRunner{stdout: []byte("a1b2c3d\n")}
	hash, err := (&Repo{runner: fr}).Commit(context.Background(), "my message")
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"commit", "-F", "-"},
		{"rev-parse", "--short", "HEAD"},
	}
	if !reflect.DeepEqual(fr.calls, want) {
		t.Errorf("calls = %v, want %v", fr.calls, want)
	}
	if string(fr.gotIn) != "my message" {
		t.Errorf("stdin = %q, want %q", fr.gotIn, "my message")
	}
	if hash != "a1b2c3d" {
		t.Errorf("hash = %q, want a1b2c3d", hash)
	}
}

func TestRepo_CommitAmend_args(t *testing.T) {
	fr := &fakeRunner{stdout: []byte("deadbee\n")}
	hash, err := (&Repo{runner: fr}).CommitAmend(context.Background(), "amended")
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"commit", "--amend", "-F", "-"},
		{"rev-parse", "--short", "HEAD"},
	}
	if !reflect.DeepEqual(fr.calls, want) {
		t.Errorf("calls = %v, want %v", fr.calls, want)
	}
	if string(fr.gotIn) != "amended" {
		t.Errorf("stdin = %q, want %q", fr.gotIn, "amended")
	}
	if hash != "deadbee" {
		t.Errorf("hash = %q, want deadbee", hash)
	}
}

func TestRepo_HeadMessage_args(t *testing.T) {
	fr := &fakeRunner{stdout: []byte("feat: x\n\nbody line\n")}
	got, err := (&Repo{runner: fr}).HeadMessage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"log", "-1", "--pretty=%B"}
	if !reflect.DeepEqual(fr.gotArgs, wantArgs) {
		t.Errorf("args = %v, want %v", fr.gotArgs, wantArgs)
	}
	if got != "feat: x\n\nbody line" {
		t.Errorf("HeadMessage = %q, want trimmed full message", got)
	}
}

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

func TestRepo_CherryPick_buildsArgsInOrderAndReturnsOutput(t *testing.T) {
	fr := &fakeRunner{
		stdout: []byte("[main abc123] first\n"),
		stderr: []byte(""),
	}
	repo := &Repo{runner: fr}

	out, err := repo.CherryPick(context.Background(), []string{"newest123", "older456"})
	if err != nil {
		t.Fatal(err)
	}

	wantArgs := []string{"cherry-pick", "newest123", "older456"}
	if !reflect.DeepEqual(fr.gotArgs, wantArgs) {
		t.Errorf("args = %v, want %v", fr.gotArgs, wantArgs)
	}
	if out != "[main abc123] first" {
		t.Errorf("output = %q, want trimmed stdout", out)
	}
}

func TestRepo_CherryPick_rejectsEmptyHashList(t *testing.T) {
	fr := &fakeRunner{}
	repo := &Repo{runner: fr}

	out, err := repo.CherryPick(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for empty cherry-pick input")
	}
	if out != "" {
		t.Fatalf("output = %q, want empty", out)
	}
	if len(fr.calls) != 0 {
		t.Fatalf("runner calls = %v, want none", fr.calls)
	}
	if !strings.Contains(err.Error(), "no commits selected") {
		t.Fatalf("error = %v, want empty-selection context", err)
	}
}

func TestRepo_CherryPick_failureReturnsCombinedOutputAndCommandError(t *testing.T) {
	fr := &fakeRunner{
		stdout: []byte("Auto-merging app.go\n"),
		stderr: []byte("CONFLICT (content): Merge conflict in app.go\n"),
		err:    errors.New("exit status 1"),
	}
	repo := &Repo{runner: fr}

	out, err := repo.CherryPick(context.Background(), []string{"abc123"})
	if err == nil {
		t.Fatal("expected cherry-pick failure")
	}
	if !strings.Contains(out, "Auto-merging app.go") || !strings.Contains(out, "CONFLICT") {
		t.Fatalf("output = %q, want stdout and stderr", out)
	}
	if !strings.Contains(err.Error(), "git cherry-pick") {
		t.Fatalf("error = %v, want command context", err)
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
	fr := &fakeRunner{
		stdout: []byte("Auto-merging a.go\n"),
		stderr: []byte("CONFLICT (content): Merge conflict in a.go\n"),
		err:    errors.New("exit status 1"),
	}
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

func TestRepo_SearchCommits_buildsGitLogArgs(t *testing.T) {
	fr := &fakeRunner{stdout: []byte("abc123\x00fix auth\x00Kael\x002 hours ago\n")}
	repo := &Repo{runner: fr}

	got, err := repo.SearchCommits(context.Background(), CommitSearch{
		Query:  "fix auth",
		Ref:    "feature/search",
		Author: "Kael",
		Limit:  25,
	})
	if err != nil {
		t.Fatal(err)
	}

	wantArgs := []string{
		"log",
		"--format=%H%x00%s%x00%an%x00%ar",
		"-n", "25",
		"--author=Kael",
		"-i", "--fixed-strings",
		"--grep=fix auth",
		"feature/search",
	}
	if !reflect.DeepEqual(fr.gotArgs, wantArgs) {
		t.Errorf("args = %v, want %v", fr.gotArgs, wantArgs)
	}
	if len(got) != 1 || got[0].Hash != "abc123" || got[0].Subject != "fix auth" {
		t.Fatalf("commits = %#v", got)
	}
}

func TestRepo_SearchCommits_omitsEmptyQueryAuthorAndUsesDefaultLimit(t *testing.T) {
	fr := &fakeRunner{}
	repo := &Repo{runner: fr}

	_, err := repo.SearchCommits(context.Background(), CommitSearch{Ref: "main"})
	if err != nil {
		t.Fatal(err)
	}

	wantArgs := []string{
		"log",
		"--format=%H%x00%s%x00%an%x00%ar",
		"-n", "50",
		"main",
	}
	if !reflect.DeepEqual(fr.gotArgs, wantArgs) {
		t.Errorf("args = %v, want %v", fr.gotArgs, wantArgs)
	}
}

func TestRepo_CommitAuthors_buildsGitLogArgsAndParses(t *testing.T) {
	fr := &fakeRunner{stdout: []byte("Kael\nAlex\nKael\n")}
	repo := &Repo{runner: fr}

	got, err := repo.CommitAuthors(context.Background(), "main", 200)
	if err != nil {
		t.Fatal(err)
	}

	wantArgs := []string{"log", "--format=%an", "-n", "200", "main"}
	if !reflect.DeepEqual(fr.gotArgs, wantArgs) {
		t.Errorf("args = %v, want %v", fr.gotArgs, wantArgs)
	}
	want := []string{"Alex", "Kael"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("authors = %#v, want %#v", got, want)
	}
}
