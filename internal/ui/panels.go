package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kael02/loom/internal/git"
)

// accentColor is the single accent used for focus, the active tab, the caret,
// section headers, and diff hunks.
const accentColor = lipgloss.Color("14")

var (
	borderFocused = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accentColor)
	borderBlur = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238"))
	cursorStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("237")).
			Foreground(lipgloss.Color("15"))
	mutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	addStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	delStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	hunkStyle   = lipgloss.NewStyle().Foreground(accentColor)
	accentStyle = lipgloss.NewStyle().Foreground(accentColor)
	titleStyle  = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	strongStyle = lipgloss.NewStyle().Bold(true)
)

// file-status marker colors, keyed to the marker shown by fileLine.
const (
	colStaged    = lipgloss.Color("10")  // staged change
	colModified  = lipgloss.Color("11")  // worktree modification
	colUntracked = lipgloss.Color("245") // untracked
	colUnmerged  = lipgloss.Color("9")   // unmerged conflict
	colDeleted   = lipgloss.Color("9")   // worktree deletion
)

// markerColor maps a file's status to its marker color, mirroring fileLine's
// marker precedence (untracked, unmerged, staged, then worktree change).
func markerColor(f git.FileStatus) lipgloss.Color {
	switch {
	case f.Untracked:
		return colUntracked
	case f.Unmerged:
		return colUnmerged
	case f.IsStaged():
		return colStaged
	case f.Worktree == 'D':
		return colDeleted
	default:
		return colModified
	}
}

// conflictLabel maps a porcelain v2 unmerged code to a human description.
func conflictLabel(code string) string {
	switch code {
	case "UU":
		return "both modified"
	case "AA":
		return "both added"
	case "DD":
		return "both deleted"
	case "AU":
		return "added by us"
	case "UD":
		return "deleted by them"
	case "UA":
		return "added by them"
	case "DU":
		return "deleted by us"
	default:
		return "unmerged"
	}
}

// list-row gutter: the selected row is a full-width highlight bar; other rows
// are padded by the same gutter so columns stay aligned.
const (
	markerWidth = 2 // width of fileLine's status-marker field
	caretGutter = "  "
)

func panelTitle(name string, count int) string {
	return fmt.Sprintf("%s %d", name, count)
}

func emptyPanelLine(p Panel) string {
	switch p {
	case PanelFiles:
		return "No changes"
	case PanelBranches:
		return "No local branches"
	case PanelCommits:
		return "No commits"
	case PanelStashes:
		return "No stashes"
	default:
		return ""
	}
}

type panelRowKind int

const (
	panelRowItem panelRowKind = iota
	panelRowHeader
	panelRowEmpty
)

type panelRow struct {
	text      string
	kind      panelRowKind
	itemIndex int
}

type fileReviewGroup struct {
	label string
	match func(git.FileStatus) bool
}

var fileReviewGroups = []fileReviewGroup{
	{label: "Conflicts", match: func(f git.FileStatus) bool { return f.Unmerged }},
	{label: "Staged", match: func(f git.FileStatus) bool { return !f.Unmerged && f.IsStaged() }},
	{label: "Unstaged", match: func(f git.FileStatus) bool {
		return !f.Unmerged && !f.IsStaged() && !f.Untracked
	}},
	{label: "Untracked", match: func(f git.FileStatus) bool { return !f.Unmerged && f.Untracked }},
}

func panelName(p Panel) string {
	switch p {
	case PanelFiles:
		return "Files"
	case PanelBranches:
		return "Branches"
	case PanelCommits:
		return "Commits"
	case PanelStashes:
		return "Stashes"
	default:
		return ""
	}
}

func (m Model) filePanelRows() []panelRow {
	if len(m.files) == 0 {
		return []panelRow{{text: emptyPanelLine(PanelFiles), kind: panelRowEmpty, itemIndex: -1}}
	}

	rows := []panelRow{}
	for _, group := range fileReviewGroups {
		start := len(rows)
		rows = append(rows, panelRow{text: group.label, kind: panelRowHeader, itemIndex: -1})
		for i, f := range m.files {
			if group.match(f) {
				rows = append(rows, panelRow{text: fileLine(f), kind: panelRowItem, itemIndex: i})
			}
		}
		if len(rows) == start+1 {
			rows = rows[:start]
		}
	}
	return rows
}

func (m Model) panelRows(p Panel) []panelRow {
	switch p {
	case PanelFiles:
		return m.filePanelRows()
	case PanelBranches:
		if len(m.branches) == 0 {
			return []panelRow{{text: emptyPanelLine(p), kind: panelRowEmpty, itemIndex: -1}}
		}
		rows := make([]panelRow, len(m.branches))
		for i, b := range m.branches {
			rows[i] = panelRow{text: branchLine(b), kind: panelRowItem, itemIndex: i}
		}
		return rows
	case PanelCommits:
		if len(m.commits) == 0 {
			return []panelRow{{text: emptyPanelLine(p), kind: panelRowEmpty, itemIndex: -1}}
		}
		rows := make([]panelRow, len(m.commits))
		for i, c := range m.commits {
			rows[i] = panelRow{text: commitLine(c), kind: panelRowItem, itemIndex: i}
		}
		return rows
	case PanelStashes:
		if len(m.stashes) == 0 {
			return []panelRow{{text: emptyPanelLine(p), kind: panelRowEmpty, itemIndex: -1}}
		}
		rows := make([]panelRow, len(m.stashes))
		for i, s := range m.stashes {
			rows[i] = panelRow{text: stashLine(s), kind: panelRowItem, itemIndex: i}
		}
		return rows
	default:
		return nil
	}
}

func panelRowTexts(rows []panelRow) []string {
	lines := make([]string, len(rows))
	for i, row := range rows {
		lines[i] = row.text
	}
	return lines
}

func (m Model) selectedPanelRow(p Panel) int {
	rows := m.panelRows(p)
	selected := m.cursor[p]
	for i, row := range rows {
		if row.kind == panelRowItem && row.itemIndex == selected {
			return i
		}
	}
	if len(rows) == 0 {
		return 0
	}
	return len(rows) - 1
}

func (m Model) panelLines(p Panel) []string {
	return panelRowTexts(m.panelRows(p))
}

// windowLines returns the slice of lines visible in a window of the given
// height starting at offset, plus how many lines remain hidden below it.
func windowLines(lines []string, offset, height int) (visible []string, hiddenBelow int) {
	if height <= 0 || len(lines) == 0 {
		return nil, 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(lines)-1 {
		offset = len(lines) - 1
	}
	end := offset + height
	if end > len(lines) {
		end = len(lines)
	}
	return lines[offset:end], len(lines) - end
}

func (m Model) styledPanelLines(p Panel, lines []string, width int) []string {
	if p == PanelFiles {
		return m.styledPanelRows(p, m.panelRows(p), width)
	}
	empty := emptyPanelLine(p)
	rows := make([]panelRow, len(lines))
	for i, line := range lines {
		kind := panelRowItem
		itemIndex := i
		if len(lines) == 1 && line == empty {
			kind = panelRowEmpty
			itemIndex = -1
		}
		rows[i] = panelRow{text: line, kind: kind, itemIndex: itemIndex}
	}
	return m.styledPanelRows(p, rows, width)
}

func (m Model) styledPanelRows(p Panel, rows []panelRow, width int) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.kind == panelRowEmpty {
			out = append(out, caretGutter+mutedStyle.Render(row.text))
			continue
		}
		if row.kind == panelRowHeader {
			out = append(out, caretGutter+mutedStyle.Render(row.text))
			continue
		}

		if row.itemIndex == m.cursor[p] && m.focus == p {
			// the selected row is a full-width highlight bar (no caret glyph). Pad
			// to the panel width and clamp to one line so a long path can't wrap
			// the bar onto a second row; the gutter aligns it with other rows.
			bar := normalizePanelRowText(caretGutter + row.text)
			if p == PanelCommits {
				out = append(out, cursorStyle.Width(width).Render(bar))
				continue
			}
			bar = oneLinePanelRow(bar, width)
			if pad := width - lipgloss.Width(bar); pad > 0 {
				bar += strings.Repeat(" ", pad)
			}
			out = append(out, cursorStyle.Inline(true).MaxWidth(width).Render(bar))
			continue
		}

		line := row.text
		// color the status marker on non-selected file rows (selection highlight
		// owns the selected row, so the two never nest)
		if p == PanelFiles && row.itemIndex >= 0 && row.itemIndex < len(m.files) && len(line) >= markerWidth {
			marker := lipgloss.NewStyle().Foreground(markerColor(m.files[row.itemIndex])).Render(line[:markerWidth])
			line = marker + line[markerWidth:]
		}
		if p == PanelCommits {
			out = append(out, lipgloss.NewStyle().Width(width).Render(normalizePanelRowText(caretGutter+line)))
			continue
		}
		out = append(out, oneLinePanelRow(caretGutter+line, width))
	}
	return out
}

func oneLinePanelRow(s string, width int) string {
	s = normalizePanelRowText(s)
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	return lipgloss.NewStyle().Inline(true).MaxWidth(width).Render(s)
}

func normalizePanelRowText(s string) string {
	return strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s)
}

// listPaneFocused reports whether list panel p currently holds keyboard focus.
// The main pane steals focus from the list when mainFocused is set, so the list
// shows its blurred border/title even though m.focus still points at it (the
// selection bar stays, since it marks the file whose diff is being read).
func (m Model) listPaneFocused(p Panel) bool {
	return m.focus == p && !m.mainFocused
}

func (m Model) renderPanel(title string, p Panel, rows []panelRow, w, h int) string {
	style := borderBlur
	if m.listPaneFocused(p) {
		style = borderFocused
	}
	style = style.Padding(0, 1)

	contentW := w - style.GetHorizontalFrameSize()
	if contentW < 0 {
		contentW = 0
	}
	contentH := h - style.GetVerticalFrameSize()
	if contentH < 0 {
		contentH = 0
	}

	listRows := contentH - 2 // title + blank line
	if listRows < 0 {
		listRows = 0
	}

	// pad the selection bar to the panel's text area (content width minus the
	// horizontal padding lipgloss keeps inside Width) so it fills the row without
	// wrapping onto a second line.
	styled := m.styledPanelRows(p, rows, contentW-style.GetHorizontalPadding())
	visible, hiddenBelow := windowLines(styled, m.scroll[p], listRows)
	if hiddenBelow > 0 && listRows > 0 {
		visible, hiddenBelow = windowLines(styled, m.scroll[p], listRows-1)
		visible = append(visible, mutedStyle.Render(fmt.Sprintf("… +%d more", hiddenBelow)))
	}

	shownTitle := title
	if m.listPaneFocused(p) {
		shownTitle = titleStyle.Render(title)
	}
	content := shownTitle + "\n\n" + strings.Join(visible, "\n")
	return style.Width(contentW).Height(contentH).MaxHeight(h).Render(content)
}

func fileLine(f git.FileStatus) string {
	mark := " "
	switch {
	case f.Untracked:
		mark = "?"
	case f.Unmerged:
		mark = "!"
	case f.IsStaged():
		mark = "+"
	case f.Worktree != 0 && f.Worktree != '.':
		mark = string(f.Worktree)
	}
	return fmt.Sprintf("%-*s %s", markerWidth, mark, f.Path)
}

func branchLine(b git.Branch) string {
	prefix := "  "
	if b.Current {
		prefix = "* "
	}
	return prefix + b.Name
}

func commitLine(c git.Commit) string {
	h := c.Hash
	if len(h) > 7 {
		h = h[:7]
	}
	return fmt.Sprintf("%s %s", h, c.Subject)
}

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

// diffKind classifies one line of diff/show output for styling.
type diffKind int

const (
	kindContext diffKind = iota
	kindAdd
	kindDel
	kindHunk
	kindMeta
)

// diffMetaPrefixes mark structural/metadata lines in diff and `git show` output.
var diffMetaPrefixes = []string{
	"diff --git", "index ", "--- ", "+++ ",
	"old mode", "new mode", "new file mode", "deleted file mode",
	"rename ", "similarity ", "copy ",
	"commit ", "Author:", "AuthorDate:", "Commit:", "CommitDate:", "Date:", "Merge:",
}

// classifyDiffLine maps a diff/show line to its kind. Order matters: hunk and
// metadata prefixes are checked before the bare +/- so +++/--- are not mistaken
// for added/removed content.
func classifyDiffLine(line string) diffKind {
	if strings.HasPrefix(line, "@@") {
		return kindHunk
	}
	for _, p := range diffMetaPrefixes {
		if strings.HasPrefix(line, p) {
			return kindMeta
		}
	}
	switch {
	case strings.HasPrefix(line, "+"):
		return kindAdd
	case strings.HasPrefix(line, "-"):
		return kindDel
	}
	return kindContext
}

// colorizeDiff applies green/red to +/- lines.
func colorizeDiff(text string) string {
	var b strings.Builder
	for _, line := range strings.Split(text, "\n") {
		switch classifyDiffLine(line) {
		case kindAdd:
			b.WriteString(addStyle.Render(line))
		case kindDel:
			b.WriteString(delStyle.Render(line))
		case kindHunk:
			b.WriteString(hunkStyle.Render(line))
		case kindMeta:
			b.WriteString(mutedStyle.Render(line))
		default:
			b.WriteString(line)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
