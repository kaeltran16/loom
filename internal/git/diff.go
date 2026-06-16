package git

import (
	"bufio"
	"bytes"
	"path/filepath"
	"strconv"
	"strings"
)

// LineKind classifies a rendered diff line. LineHunk and LineMeta exist for
// tolerance/documentation; ParseDiff carries hunk identity in the Hunk struct
// and suppresses commit/show preamble, so it does not emit those two kinds.
type LineKind int

const (
	LineContext LineKind = iota
	LineAdd
	LineDel
	LineHunk
	LineMeta
)

// DiffLine is one content line of a hunk, with the leading +/-/space stripped.
// OldNo/NewNo are 0 when the line does not exist on that side.
type DiffLine struct {
	Kind  LineKind
	OldNo int
	NewNo int
	Text  string
}

// Hunk is one @@ block. Header is the function context git emits after the
// closing @@ (empty when absent).
type Hunk struct {
	Header string
	Lines  []DiffLine
}

// FileDiff is one file's worth of changes.
type FileDiff struct {
	Path  string
	Lang  string // file extension without the dot, for the syntax lexer
	Adds  int
	Dels  int
	Hunks []Hunk
}

// Diff is the parsed model — the single source of truth for rendering.
type Diff struct {
	Files []FileDiff
}

// ParseDiff parses unified `git diff` or `git show` output. It is pure and
// tolerant: unknown structural lines are skipped, and any show commit preamble
// (before the first "diff --git") is suppressed in v1.
func ParseDiff(raw string) Diff {
	var d Diff
	var cur *FileDiff
	var hunk *Hunk
	var oldNo, newNo int

	flushHunk := func() {
		if cur != nil && hunk != nil {
			cur.Hunks = append(cur.Hunks, *hunk)
			hunk = nil
		}
	}
	flushFile := func() {
		flushHunk()
		if cur != nil {
			d.Files = append(d.Files, *cur)
			cur = nil
		}
	}

	sc := bufio.NewScanner(strings.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flushFile()
			path := parseDiffGitPath(line)
			cur = &FileDiff{Path: path, Lang: langFromPath(path)}
		case cur == nil:
			// commit/show preamble — suppressed in v1
			continue
		case strings.HasPrefix(line, "@@"):
			flushHunk()
			oldNo, newNo = parseHunkHeader(line)
			hunk = &Hunk{Header: hunkContext(line)}
		case hunk == nil:
			// structural lines before the first hunk: index, mode, ---, +++,
			// rename, similarity. Ignored (path/counts come from elsewhere).
			continue
		case strings.HasPrefix(line, "\\"):
			// "\ No newline at end of file" — not a content line
			continue
		case strings.HasPrefix(line, "+"):
			hunk.Lines = append(hunk.Lines, DiffLine{Kind: LineAdd, NewNo: newNo, Text: line[1:]})
			cur.Adds++
			newNo++
		case strings.HasPrefix(line, "-"):
			hunk.Lines = append(hunk.Lines, DiffLine{Kind: LineDel, OldNo: oldNo, Text: line[1:]})
			cur.Dels++
			oldNo++
		default:
			text := line
			if strings.HasPrefix(line, " ") {
				text = line[1:]
			}
			hunk.Lines = append(hunk.Lines, DiffLine{Kind: LineContext, OldNo: oldNo, NewNo: newNo, Text: text})
			oldNo++
			newNo++
		}
	}
	flushFile()
	return d
}

// MessageDiff builds a one-line informational Diff (binary, unreadable, etc.).
func MessageDiff(path, message string) Diff {
	return Diff{Files: []FileDiff{{
		Path: path, Lang: langFromPath(path),
		Hunks: []Hunk{{Lines: []DiffLine{{Kind: LineContext, Text: message}}}},
	}}}
}

// SynthesizeUntracked builds an all-additions Diff for an untracked file's
// working-tree content. NUL in the scanned prefix yields a "Binary file" model.
func SynthesizeUntracked(path string, content []byte) Diff {
	if isBinary(content) {
		return MessageDiff(path, "Binary file")
	}
	text := strings.TrimSuffix(string(content), "\n")
	var lines []DiffLine
	if text != "" {
		for i, l := range strings.Split(text, "\n") {
			lines = append(lines, DiffLine{Kind: LineAdd, NewNo: i + 1, Text: l})
		}
	}
	return Diff{Files: []FileDiff{{
		Path: path, Lang: langFromPath(path),
		Adds:  len(lines),
		Hunks: []Hunk{{Lines: lines}},
	}}}
}

func isBinary(content []byte) bool {
	n := len(content)
	if n > 8000 {
		n = 8000
	}
	return bytes.IndexByte(content[:n], 0) >= 0
}

// parseDiffGitPath returns the new-side path from "diff --git a/<old> b/<new>".
// Assumes paths without spaces (v1).
func parseDiffGitPath(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return ""
	}
	return strings.TrimPrefix(fields[len(fields)-1], "b/")
}

// parseHunkHeader reads the old/new start lines from "@@ -a,b +c,d @@".
func parseHunkHeader(line string) (oldStart, newStart int) {
	parts := strings.Fields(line)
	if len(parts) >= 3 {
		oldStart = atoiBefore(strings.TrimPrefix(parts[1], "-"), ',')
		newStart = atoiBefore(strings.TrimPrefix(parts[2], "+"), ',')
	}
	return
}

func atoiBefore(s string, sep byte) int {
	if i := strings.IndexByte(s, sep); i >= 0 {
		s = s[:i]
	}
	n, _ := strconv.Atoi(s)
	return n
}

// hunkContext returns the function context after the closing "@@".
func hunkContext(line string) string {
	if i := strings.Index(line, "@@"); i >= 0 {
		rest := line[i+2:]
		if j := strings.Index(rest, "@@"); j >= 0 {
			return strings.TrimSpace(rest[j+2:])
		}
	}
	return ""
}

// langFromPath derives the syntax language token (extension without the dot).
func langFromPath(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return ""
	}
	return strings.ToLower(ext[1:])
}
