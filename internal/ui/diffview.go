package ui

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/kael02/loom/internal/git"
)

const (
	diffNumWidth   = 4
	diffGutterCols = diffNumWidth + 1 + diffNumWidth + 1 + 1 + 1 // "NNNN NNNN │ " = 12 cells
	changeBarWidth = 5
	diffTabWidth   = 4
)

var (
	diffGutterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	addRowStyle     = lipgloss.NewStyle().Background(lipgloss.Color("22")) // dim green
	delRowStyle     = lipgloss.NewStyle().Background(lipgloss.Color("52")) // dim red
	fileHeaderStyle = lipgloss.NewStyle().Bold(true)
	hunkBandStyle   = lipgloss.NewStyle().Background(accentColor).Foreground(lipgloss.Color("0"))
	addWordStyle    = lipgloss.NewStyle().Background(lipgloss.Color("28")) // brighter green
	delWordStyle    = lipgloss.NewStyle().Background(lipgloss.Color("88")) // brighter red
)

// renderDiff walks the parsed diff and returns the viewport lines plus the row
// index of each hunk band (used by n/N navigation). Pure: all width math and
// styling live here; the model carries no styling.
func renderDiff(d git.Diff, width int) (lines []string, hunkRows []int) {
	for _, f := range d.Files {
		lines = append(lines, renderFileHeader(f, width))
		for _, h := range f.Hunks {
			hunkRows = append(hunkRows, len(lines))
			lines = append(lines, renderHunkBand(h, width))
			lines = append(lines, renderHunkLines(h.Lines, f.Lang, width)...)
		}
	}
	return lines, hunkRows
}

func renderFileHeader(f git.FileDiff, width int) string {
	head := fmt.Sprintf("%s  %s  +%d -%d", f.Path, changeBar(f.Adds, f.Dels), f.Adds, f.Dels)
	return fileHeaderStyle.Width(width).Render(ansi.Truncate(head, width, "…"))
}

// changeBar is a fixed-width bar whose green/red split is proportional to the
// add/del ratio. All cells are empty when there are no changes.
func changeBar(adds, dels int) string {
	total := adds + dels
	if total == 0 {
		return strings.Repeat("▱", changeBarWidth)
	}
	green := (adds*changeBarWidth + total/2) / total // rounded
	if green > changeBarWidth {
		green = changeBarWidth
	}
	return addStyle.Render(strings.Repeat("▰", green)) + delStyle.Render(strings.Repeat("▰", changeBarWidth-green))
}

func renderHunkBand(h git.Hunk, width int) string {
	s := "@@"
	if h.Header != "" {
		s = "@@ " + h.Header
	}
	return hunkBandStyle.Width(width).Render(ansi.Truncate(s, width, "…"))
}

func renderLine(ln git.DiffLine, lang string, width int) string {
	contentWidth := width - diffGutterCols
	if contentWidth < 0 {
		contentWidth = 0
	}
	row := gutter(ln) + ansi.Truncate(diffContent(ln.Text, lang), contentWidth, "…")
	switch ln.Kind {
	case git.LineAdd:
		return addRowStyle.Width(width).Render(row)
	case git.LineDel:
		return delRowStyle.Width(width).Render(row)
	default:
		return row // context rows: no background, no padding
	}
}

type seg struct {
	Text    string
	Changed bool
}

// wordDiff computes a rune-level LCS of a and b and returns each side split
// into unchanged/changed segments. Pure.
func wordDiff(a, b string) (aSegs, bSegs []seg) {
	ar, br := []rune(a), []rune(b)
	n, m := len(ar), len(br)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if ar[i] == br[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	aChanged := make([]bool, n)
	bChanged := make([]bool, m)
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case ar[i] == br[j]:
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			aChanged[i] = true
			i++
		default:
			bChanged[j] = true
			j++
		}
	}
	for ; i < n; i++ {
		aChanged[i] = true
	}
	for ; j < m; j++ {
		bChanged[j] = true
	}
	return groupSegs(ar, aChanged), groupSegs(br, bChanged)
}

func groupSegs(rs []rune, changed []bool) []seg {
	var out []seg
	for i := 0; i < len(rs); {
		j := i
		for j < len(rs) && changed[j] == changed[i] {
			j++
		}
		out = append(out, seg{Text: string(rs[i:j]), Changed: changed[i]})
		i = j
	}
	return out
}

// renderHunkLines renders a hunk's lines, detecting replace runs (consecutive
// deletions immediately followed by additions) so paired lines get word-level
// highlighting; everything else uses the plain row renderer.
func renderHunkLines(lines []git.DiffLine, lang string, width int) []string {
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); {
		if lines[i].Kind == git.LineDel {
			j := i
			for j < len(lines) && lines[j].Kind == git.LineDel {
				j++
			}
			k := j
			for k < len(lines) && lines[k].Kind == git.LineAdd {
				k++
			}
			if k > j { // a real replace run (dels followed by adds)
				out = append(out, renderReplaceRun(lines[i:j], lines[j:k], lang, width)...)
				i = k
				continue
			}
		}
		out = append(out, renderLine(lines[i], lang, width))
		i++
	}
	return out
}

func renderReplaceRun(dels, adds []git.DiffLine, lang string, width int) []string {
	pairs := min(len(dels), len(adds))
	out := make([]string, 0, len(dels)+len(adds))
	for x, d := range dels {
		if x < pairs {
			dSegs, _ := wordDiff(d.Text, adds[x].Text)
			out = append(out, renderSegLine(d, dSegs, delRowStyle, delWordStyle, lang, width))
		} else {
			out = append(out, renderLine(d, lang, width))
		}
	}
	for x, a := range adds {
		if x < pairs {
			_, aSegs := wordDiff(dels[x].Text, a.Text)
			out = append(out, renderSegLine(a, aSegs, addRowStyle, addWordStyle, lang, width))
		} else {
			out = append(out, renderLine(a, lang, width))
		}
	}
	return out
}

func renderSegLine(ln git.DiffLine, segs []seg, rowStyle, wordStyle lipgloss.Style, lang string, width int) string {
	contentWidth := width - diffGutterCols
	if contentWidth < 0 {
		contentWidth = 0
	}
	var b strings.Builder
	for _, s := range segs {
		txt := highlightLine(expandTabs(s.Text, diffTabWidth), lang)
		if s.Changed {
			b.WriteString(wordStyle.Render(txt))
		} else {
			b.WriteString(txt)
		}
	}
	return rowStyle.Width(width).Render(gutter(ln) + ansi.Truncate(b.String(), contentWidth, "…"))
}

// diffContent prepares a line's content for display: tabs expanded to a fixed
// stop, syntax colors applied by language, and trailing whitespace shown as
// muted middots.
func diffContent(text, lang string) string {
	exp := expandTabs(text, diffTabWidth)
	body, trail := splitTrailingWS(exp)
	return highlightLine(body, lang) + mutedDots(trail)
}

// highlightLine applies syntax foreground colors to a single line of code.
// Foreground (syntax) and background (row/word) are independent ANSI
// attributes, so they compose. Any error or unknown language returns the input
// unchanged, keeping the plain-text projection intact.
func highlightLine(text, lang string) string {
	if lang == "" {
		return text
	}
	lexer := lexers.Get(lang)
	if lexer == nil {
		return text
	}
	it, err := lexer.Tokenise(nil, text)
	if err != nil {
		return text
	}
	var b strings.Builder
	for _, tok := range it.Tokens() {
		if c, ok := syntaxColor(tok.Type); ok {
			b.WriteString(lipgloss.NewStyle().Foreground(c).Render(tok.Value))
		} else {
			b.WriteString(tok.Value)
		}
	}
	out := b.String()
	if !strings.HasSuffix(text, "\n") {
		out = strings.TrimSuffix(out, "\n") // lexers may append a trailing newline
	}
	return out
}

// syntaxColor maps a chroma token category to a fixed foreground color.
func syntaxColor(t chroma.TokenType) (lipgloss.Color, bool) {
	switch {
	case t.InCategory(chroma.Comment):
		return lipgloss.Color("245"), true
	case t.InCategory(chroma.Keyword):
		return lipgloss.Color("13"), true
	case t.InCategory(chroma.LiteralString):
		return lipgloss.Color("10"), true
	case t.InCategory(chroma.LiteralNumber):
		return lipgloss.Color("11"), true
	case t.InCategory(chroma.Name):
		return lipgloss.Color("14"), true
	}
	return "", false
}

func expandTabs(s string, tabWidth int) string {
	if !strings.ContainsRune(s, '\t') {
		return s
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			n := tabWidth - col%tabWidth
			b.WriteString(strings.Repeat(" ", n))
			col += n
			continue
		}
		b.WriteRune(r)
		col++
	}
	return b.String()
}

func splitTrailingWS(s string) (body, trail string) {
	body = strings.TrimRight(s, " ")
	return body, s[len(body):]
}

func mutedDots(trail string) string {
	if trail == "" {
		return ""
	}
	return mutedStyle.Render(strings.Repeat("·", len(trail)))
}

func gutter(ln git.DiffLine) string {
	return diffGutterStyle.Render(fmt.Sprintf("%s %s │ ", numCell(ln.OldNo), numCell(ln.NewNo)))
}

func numCell(n int) string {
	if n == 0 {
		return strings.Repeat(" ", diffNumWidth)
	}
	return fmt.Sprintf("%*d", diffNumWidth, n)
}
