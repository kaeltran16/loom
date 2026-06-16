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
