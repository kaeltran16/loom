package git

import "testing"

// Context lines begin with one space; add/del with +/-. Preserve exact whitespace.
const diffSample = `diff --git a/internal/ui/view.go b/internal/ui/view.go
index 1111111..2222222 100644
--- a/internal/ui/view.go
+++ b/internal/ui/view.go
@@ -10,4 +10,4 @@ func (m Model) View() string {
 context one
-old line
+new line
 context two
`

const diffNewFile = `diff --git a/notes.txt b/notes.txt
new file mode 100644
index 0000000..3333333
--- /dev/null
+++ b/notes.txt
@@ -0,0 +1,2 @@
+first
+second
`

const diffRename = `diff --git a/old/name.go b/new/name.go
similarity index 100%
rename from old/name.go
rename to new/name.go
`

const showSample = `commit abc123def4567890
Author: Kael <k@example.com>
Date:   Wed Jun 11 10:00:00 2026

    fix two files

diff --git a/a.go b/a.go
index aaa1111..bbb2222 100644
--- a/a.go
+++ b/a.go
@@ -1,2 +1,2 @@ package main
-old a
+new a
 keep a
diff --git a/b.md b/b.md
index ccc3333..ddd4444 100644
--- a/b.md
+++ b/b.md
@@ -5,1 +5,2 @@ title
 keep b
+added b
`

func TestParseDiffModification(t *testing.T) {
	d := ParseDiff(diffSample)
	if len(d.Files) != 1 {
		t.Fatalf("want 1 file, got %d", len(d.Files))
	}
	f := d.Files[0]
	if f.Path != "internal/ui/view.go" || f.Lang != "go" {
		t.Fatalf("path/lang = %q/%q", f.Path, f.Lang)
	}
	if f.Adds != 1 || f.Dels != 1 {
		t.Fatalf("adds/dels = %d/%d, want 1/1", f.Adds, f.Dels)
	}
	if len(f.Hunks) != 1 || f.Hunks[0].Header != "func (m Model) View() string {" {
		t.Fatalf("hunk header = %q", f.Hunks[0].Header)
	}
	want := []DiffLine{
		{Kind: LineContext, OldNo: 10, NewNo: 10, Text: "context one"},
		{Kind: LineDel, OldNo: 11, NewNo: 0, Text: "old line"},
		{Kind: LineAdd, OldNo: 0, NewNo: 11, Text: "new line"},
		{Kind: LineContext, OldNo: 12, NewNo: 12, Text: "context two"},
	}
	got := f.Hunks[0].Lines
	if len(got) != len(want) {
		t.Fatalf("want %d lines, got %d: %+v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseDiffNewFile(t *testing.T) {
	d := ParseDiff(diffNewFile)
	f := d.Files[0]
	if f.Path != "notes.txt" || f.Adds != 2 || f.Dels != 0 {
		t.Fatalf("new file = %+v", f)
	}
	if f.Hunks[0].Lines[0] != (DiffLine{Kind: LineAdd, NewNo: 1, Text: "first"}) {
		t.Errorf("line[0] = %+v", f.Hunks[0].Lines[0])
	}
	if f.Hunks[0].Lines[1].NewNo != 2 {
		t.Errorf("line[1] NewNo = %d, want 2", f.Hunks[0].Lines[1].NewNo)
	}
}

func TestParseDiffRename(t *testing.T) {
	d := ParseDiff(diffRename)
	if len(d.Files) != 1 {
		t.Fatalf("want 1 file, got %d", len(d.Files))
	}
	f := d.Files[0]
	if f.Path != "new/name.go" || f.Adds != 0 || f.Dels != 0 || len(f.Hunks) != 0 {
		t.Fatalf("rename = %+v", f)
	}
}

func TestSynthesizeUntrackedTextFile(t *testing.T) {
	d := SynthesizeUntracked("notes.txt", []byte("alpha\nbeta\n"))
	f := d.Files[0]
	if f.Path != "notes.txt" || f.Adds != 2 || f.Dels != 0 {
		t.Fatalf("synthesized = %+v", f)
	}
	lines := f.Hunks[0].Lines
	if lines[0] != (DiffLine{Kind: LineAdd, NewNo: 1, Text: "alpha"}) {
		t.Errorf("line[0] = %+v", lines[0])
	}
	if lines[1] != (DiffLine{Kind: LineAdd, NewNo: 2, Text: "beta"}) {
		t.Errorf("line[1] = %+v", lines[1])
	}
}

func TestSynthesizeUntrackedBinary(t *testing.T) {
	d := SynthesizeUntracked("img.png", []byte{0x89, 0x50, 0x00, 0x4e})
	line := d.Files[0].Hunks[0].Lines[0]
	if line.Kind != LineContext || line.Text != "Binary file" {
		t.Fatalf("binary model = %+v", line)
	}
}

func TestParseShowMultiFile(t *testing.T) {
	d := ParseDiff(showSample)
	if len(d.Files) != 2 {
		t.Fatalf("want 2 files, got %d", len(d.Files))
	}
	if d.Files[0].Path != "a.go" || d.Files[0].Adds != 1 || d.Files[0].Dels != 1 {
		t.Errorf("file[0] = %+v", d.Files[0])
	}
	if d.Files[0].Hunks[0].Header != "package main" {
		t.Errorf("file[0] header = %q", d.Files[0].Hunks[0].Header)
	}
	if d.Files[1].Path != "b.md" || d.Files[1].Adds != 1 || d.Files[1].Dels != 0 {
		t.Errorf("file[1] = %+v", d.Files[1])
	}
	// b.md: " keep b" is context at 5/5; "+added b" is an add at new line 6.
	addLine := d.Files[1].Hunks[0].Lines[1]
	if addLine.Kind != LineAdd || addLine.NewNo != 6 {
		t.Errorf("b.md add line = %+v, want LineAdd NewNo 6", addLine)
	}
}
