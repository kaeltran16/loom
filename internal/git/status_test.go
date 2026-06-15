package git

import (
	"os"
	"testing"
)

func TestParseStatus(t *testing.T) {
	data, err := os.ReadFile("testdata/status_v2.txt")
	if err != nil {
		t.Fatal(err)
	}
	files, br, err := parseStatus(data)
	if err != nil {
		t.Fatal(err)
	}

	if br.Name != "main" || br.Upstream != "origin/main" || br.Ahead != 2 || br.Behind != 1 {
		t.Fatalf("branch parsed wrong: %+v", br)
	}
	if len(files) != 5 {
		t.Fatalf("want 5 files, got %d: %+v", len(files), files)
	}

	// src/app.go: staged modification (X=M, Y=.)
	if files[0].Path != "src/app.go" || !files[0].IsStaged() || files[0].Worktree != '.' {
		t.Errorf("file[0] wrong: %+v", files[0])
	}
	// README.md: unstaged modification (X=., Y=M)
	if files[1].Path != "README.md" || files[1].IsStaged() || files[1].Worktree != 'M' {
		t.Errorf("file[1] wrong: %+v", files[1])
	}
	// rename: path is the new path
	if files[2].Path != "docs/new.md" {
		t.Errorf("file[2] path wrong: %+v", files[2])
	}
	// unmerged
	if files[3].Path != "conflict.txt" || !files[3].Unmerged {
		t.Errorf("file[3] wrong: %+v", files[3])
	}
	// untracked
	if files[4].Path != "notes.txt" || !files[4].Untracked {
		t.Errorf("file[4] wrong: %+v", files[4])
	}
}
