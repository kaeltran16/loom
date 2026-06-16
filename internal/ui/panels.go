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
	default:
		return ""
	}
}

func panelName(p Panel) string {
	switch p {
	case PanelFiles:
		return "Files"
	case PanelBranches:
		return "Branches"
	case PanelCommits:
		return "Commits"
	default:
		return ""
	}
}

func (m Model) panelLines(p Panel) []string {
	switch p {
	case PanelFiles:
		if len(m.files) == 0 {
			return []string{emptyPanelLine(p)}
		}
		lines := make([]string, len(m.files))
		for i, f := range m.files {
			lines[i] = fileLine(f)
		}
		return lines
	case PanelBranches:
		if len(m.branches) == 0 {
			return []string{emptyPanelLine(p)}
		}
		lines := make([]string, len(m.branches))
		for i, b := range m.branches {
			lines[i] = branchLine(b)
		}
		return lines
	case PanelCommits:
		if len(m.commits) == 0 {
			return []string{emptyPanelLine(p)}
		}
		lines := make([]string, len(m.commits))
		for i, c := range m.commits {
			lines[i] = commitLine(c)
		}
		return lines
	default:
		return nil
	}
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
	out := make([]string, 0, len(lines))
	sel := m.cursor[p]
	empty := emptyPanelLine(p)
	for i, l := range lines {
		if len(lines) == 1 && l == empty {
			out = append(out, caretGutter+mutedStyle.Render(l))
			continue
		}
		if i == sel && m.focus == p {
			// the selected row is a full-width highlight bar (no caret glyph). Pad
			// to the panel width and clamp to one line so a long path can't wrap
			// the bar onto a second row; the gutter aligns it with other rows.
			bar := caretGutter + l
			if pad := width - lipgloss.Width(bar); pad > 0 {
				bar += strings.Repeat(" ", pad)
			}
			out = append(out, cursorStyle.Inline(true).MaxWidth(width).Render(bar))
			continue
		}
		// color the status marker on non-selected file rows (selection highlight
		// owns the selected row, so the two never nest)
		if p == PanelFiles && i < len(m.files) && len(l) >= markerWidth {
			marker := lipgloss.NewStyle().Foreground(markerColor(m.files[i])).Render(l[:markerWidth])
			l = marker + l[markerWidth:]
		}
		out = append(out, caretGutter+l)
	}
	return out
}

// listPaneFocused reports whether list panel p currently holds keyboard focus.
// The main pane steals focus from the list when mainFocused is set, so the list
// shows its blurred border/title even though m.focus still points at it (the
// selection bar stays, since it marks the file whose diff is being read).
func (m Model) listPaneFocused(p Panel) bool {
	return m.focus == p && !m.mainFocused
}

func (m Model) renderPanel(title string, p Panel, lines []string, w, h int) string {
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
	styled := m.styledPanelLines(p, lines, contentW-style.GetHorizontalPadding())
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
