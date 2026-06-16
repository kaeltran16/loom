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
