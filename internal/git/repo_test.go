package git

import (
	"context"
	"errors"
	"reflect"
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

func TestRepo_CommitAll_stagesEverythingThenCommits(t *testing.T) {
	fr := &fakeRunner{}
	if err := (&Repo{runner: fr}).CommitAll(context.Background(), "my message"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"add", "-A"},
		{"commit", "-F", "-"},
	}
	if !reflect.DeepEqual(fr.calls, want) {
		t.Errorf("calls = %v, want %v", fr.calls, want)
	}
	if string(fr.gotIn) != "my message" {
		t.Errorf("stdin = %q, want %q", fr.gotIn, "my message")
	}
}

func TestRepo_CommitAll_doesNotCommitWhenStageFails(t *testing.T) {
	fr := &fakeRunner{err: errors.New("boom")}
	err := (&Repo{runner: fr}).CommitAll(context.Background(), "msg")
	if err == nil {
		t.Fatal("expected error when staging fails")
	}
	if len(fr.calls) != 1 {
		t.Errorf("expected only the failed `add` call, got %v", fr.calls)
	}
}

func TestRepo_Commit_passesMessageViaStdin(t *testing.T) {
	fr := &fakeRunner{}
	if err := (&Repo{runner: fr}).Commit(context.Background(), "my message"); err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"commit", "-F", "-"}
	if !reflect.DeepEqual(fr.gotArgs, wantArgs) {
		t.Errorf("args = %v, want %v", fr.gotArgs, wantArgs)
	}
	if string(fr.gotIn) != "my message" {
		t.Errorf("stdin = %q, want %q", fr.gotIn, "my message")
	}
}
