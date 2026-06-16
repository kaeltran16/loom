package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/kael02/loom/internal/git"
)

func sampleDiff() git.Diff {
	return git.Diff{Files: []git.FileDiff{{
		Path: "x.go", Lang: "go", Adds: 1, Dels: 1,
		Hunks: []git.Hunk{{
			Header: "func View()",
			Lines: []git.DiffLine{
				{Kind: git.LineContext, OldNo: 10, NewNo: 10, Text: "context"},
				{Kind: git.LineDel, OldNo: 11, Text: "old line"},
				{Kind: git.LineAdd, NewNo: 11, Text: "new line"},
			},
		}},
	}}}
}

func TestRenderDiffProjectsModel(t *testing.T) {
	lines, hunkRows := renderDiff(sampleDiff(), 80)
	joined := ansi.Strip(strings.Join(lines, "\n"))
	for _, want := range []string{"x.go", "+1 -1", "func View()", "old line", "new line", "context"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("renderDiff projection missing %q:\n%s", want, joined)
		}
	}
	// one hunk → one band row index, and it points at a line containing the header
	if len(hunkRows) != 1 {
		t.Fatalf("hunkRows = %v, want 1 entry", hunkRows)
	}
	if !strings.Contains(ansi.Strip(lines[hunkRows[0]]), "func View()") {
		t.Fatalf("hunkRows[0] does not point at the band: %q", lines[hunkRows[0]])
	}
}

func TestHighlightLinePreservesPlainText(t *testing.T) {
	const code = "func main() { return }"
	if got := ansi.Strip(highlightLine(code, "go")); got != code {
		t.Errorf("highlighted projection = %q, want %q", got, code)
	}
}

func TestHighlightLineUnknownLangFallsBack(t *testing.T) {
	const code = "some text"
	if got := highlightLine(code, ""); got != code {
		t.Errorf("empty lang should be returned verbatim, got %q", got)
	}
	if got := highlightLine(code, "no-such-lang"); ansi.Strip(got) != code {
		t.Errorf("unknown lang projection = %q, want %q", ansi.Strip(got), code)
	}
}

func TestRenderDiffSyntaxKeepsProjection(t *testing.T) {
	const width = 60
	d := git.Diff{Files: []git.FileDiff{{
		Path: "x.go", Lang: "go", Adds: 1,
		Hunks: []git.Hunk{{Lines: []git.DiffLine{
			{Kind: git.LineAdd, NewNo: 1, Text: "func main() {}"},
		}}},
	}}}
	lines, _ := renderDiff(d, width)
	joined := ansi.Strip(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "func main() {}") {
		t.Errorf("syntax highlighting altered the projection:\n%s", joined)
	}
}

func TestExpandTabs(t *testing.T) {
	cases := map[string]string{
		"a\tb":  "a   b", // 'a' at col 0, tab fills to col 4
		"\tx":   "    x", // leading tab → 4 spaces
		"ab\tc": "ab  c", // 'ab' at col 2, tab fills to col 4
		"plain": "plain",
	}
	for in, want := range cases {
		if got := expandTabs(in, diffTabWidth); got != want {
			t.Errorf("expandTabs(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMarkTrailingWhitespaceInProjection(t *testing.T) {
	const width = 40
	line := renderLine(git.DiffLine{Kind: git.LineContext, OldNo: 1, NewNo: 1, Text: "code  "}, "", width)
	if !strings.Contains(ansi.Strip(line), "code··") {
		t.Errorf("trailing spaces not shown as middots: %q", ansi.Strip(line))
	}
}

func TestWordDiffSubstitution(t *testing.T) {
	aSegs, bSegs := wordDiff("foo bar", "foo baz")
	wantA := []seg{{Text: "foo ba", Changed: false}, {Text: "r", Changed: true}}
	wantB := []seg{{Text: "foo ba", Changed: false}, {Text: "z", Changed: true}}
	if !segsEqual(aSegs, wantA) {
		t.Errorf("aSegs = %+v, want %+v", aSegs, wantA)
	}
	if !segsEqual(bSegs, wantB) {
		t.Errorf("bSegs = %+v, want %+v", bSegs, wantB)
	}
}

func TestWordDiffInsertion(t *testing.T) {
	aSegs, bSegs := wordDiff("foo", "foo bar")
	if !segsEqual(aSegs, []seg{{Text: "foo", Changed: false}}) {
		t.Errorf("aSegs = %+v", aSegs)
	}
	if !segsEqual(bSegs, []seg{{Text: "foo", Changed: false}, {Text: " bar", Changed: true}}) {
		t.Errorf("bSegs = %+v", bSegs)
	}
}

func TestWordDiffOneSideEmpty(t *testing.T) {
	aSegs, bSegs := wordDiff("", "abc")
	if len(aSegs) != 0 {
		t.Errorf("aSegs = %+v, want empty", aSegs)
	}
	if !segsEqual(bSegs, []seg{{Text: "abc", Changed: true}}) {
		t.Errorf("bSegs = %+v", bSegs)
	}
}

func TestRenderDiffWordHighlightPreservesProjectionAndWidth(t *testing.T) {
	const width = 50
	d := git.Diff{Files: []git.FileDiff{{
		Path: "x.go", Lang: "go", Adds: 1, Dels: 1,
		Hunks: []git.Hunk{{Lines: []git.DiffLine{
			{Kind: git.LineDel, OldNo: 1, Text: "value := foo"},
			{Kind: git.LineAdd, NewNo: 1, Text: "value := bar"},
		}}},
	}}}
	lines, _ := renderDiff(d, width)
	for _, l := range lines {
		plain := ansi.Strip(l)
		if strings.Contains(plain, "foo") {
			if !strings.Contains(plain, "value := foo") {
				t.Errorf("del projection altered: %q", plain)
			}
			if w := lipgloss.Width(l); w != width {
				t.Errorf("del row width = %d, want %d", w, width)
			}
		}
		if strings.Contains(plain, "bar") {
			if !strings.Contains(plain, "value := bar") {
				t.Errorf("add projection altered: %q", plain)
			}
		}
	}
}

func segsEqual(a, b []seg) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestRenderHunkBandFullWidth(t *testing.T) {
	const width = 40
	band := renderHunkBand(git.Hunk{Header: "func View()"}, width)
	if w := lipgloss.Width(band); w != width {
		t.Errorf("hunk band width = %d, want %d", w, width)
	}
	if !strings.Contains(ansi.Strip(band), "func View()") {
		t.Errorf("hunk band missing header: %q", ansi.Strip(band))
	}
}

func TestChangeBarWidth(t *testing.T) {
	if w := lipgloss.Width(changeBar(12, 3)); w != changeBarWidth {
		t.Errorf("changeBar width = %d, want %d", w, changeBarWidth)
	}
	if w := lipgloss.Width(changeBar(0, 0)); w != changeBarWidth {
		t.Errorf("empty changeBar width = %d, want %d", w, changeBarWidth)
	}
}

func TestRenderFileHeaderBand(t *testing.T) {
	const width = 50
	h := renderFileHeader(git.FileDiff{Path: "x.go", Lang: "go", Adds: 12, Dels: 3}, width)
	if w := lipgloss.Width(h); w != width {
		t.Errorf("file header width = %d, want %d", w, width)
	}
	plain := ansi.Strip(h)
	for _, want := range []string{"x.go", "+12 -3"} {
		if !strings.Contains(plain, want) {
			t.Errorf("file header missing %q: %q", want, plain)
		}
	}
}

func TestRenderLineGutterAndFullWidth(t *testing.T) {
	const width = 40
	add := renderLine(git.DiffLine{Kind: git.LineAdd, NewNo: 11, Text: "added"}, "", width)
	del := renderLine(git.DiffLine{Kind: git.LineDel, OldNo: 7, Text: "removed"}, "", width)

	// add/del rows fill the full width so the background band spans the row
	if w := lipgloss.Width(add); w != width {
		t.Errorf("add row width = %d, want %d", w, width)
	}
	if w := lipgloss.Width(del); w != width {
		t.Errorf("del row width = %d, want %d", w, width)
	}
	// gutter shows the line number on the applicable side
	if !strings.Contains(ansi.Strip(add), "11") {
		t.Errorf("add row missing new line number: %q", ansi.Strip(add))
	}
	if !strings.Contains(ansi.Strip(del), "7") {
		t.Errorf("del row missing old line number: %q", ansi.Strip(del))
	}
	// content is still present
	if !strings.Contains(ansi.Strip(add), "added") {
		t.Errorf("add row missing content: %q", ansi.Strip(add))
	}
}

func TestRenderDiffTruncatesLongLines(t *testing.T) {
	d := git.Diff{Files: []git.FileDiff{{
		Path: "x.go", Lang: "go",
		Hunks: []git.Hunk{{Lines: []git.DiffLine{
			{Kind: git.LineContext, OldNo: 1, NewNo: 1, Text: strings.Repeat("abcdefghij", 20)},
		}}},
	}}}
	lines, _ := renderDiff(d, 30)
	for _, l := range lines {
		if w := lipgloss.Width(l); w > 30 {
			t.Fatalf("line exceeds width: %d > 30 (%q)", w, ansi.Strip(l))
		}
	}
	if !strings.Contains(ansi.Strip(strings.Join(lines, "\n")), "…") {
		t.Fatalf("expected an ellipsis on the truncated line")
	}
}
