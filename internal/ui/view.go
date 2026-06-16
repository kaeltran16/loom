package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const mainHeaderHeight = 3

// commitHeaderHeight is the lines mainContent draws above the input in commit
// mode: heading, scope hint, key hint, and a blank separator.
const commitHeaderHeight = 4

const (
	topBarHeight      = 1
	statusRailWidth   = 30
	listMinWidth      = 26
	minRailWindowWide = 110
	recentCommandMax  = 3
)

func (m Model) View() string {
	if m.w == 0 {
		return "Loading repository..."
	}

	if m.showHelp {
		return m.helpOverlay()
	}
	if m.showLog {
		return m.commandLogOverlay()
	}

	railVisible := m.w >= minRailWindowWide

	bodyH := m.h - topBarHeight - 1
	if bodyH < 0 {
		bodyH = 0
	}

	railOuter := 0
	if railVisible {
		railOuter = statusRailWidth
	}
	listOuter := m.w / 4
	if listOuter < listMinWidth {
		listOuter = listMinWidth
	}
	mainOuter := m.w - listOuter - railOuter
	if mainOuter < 0 {
		mainOuter = 0
	}

	list := m.renderPanel(panelTitle(panelName(m.focus), m.focusLen()), m.focus, m.panelLines(m.focus), listOuter, bodyH)

	mainStyle := borderBlur
	if m.mainFocused {
		mainStyle = borderFocused
	}
	mainInnerW := mainOuter - mainStyle.GetHorizontalFrameSize()
	if mainInnerW < 0 {
		mainInnerW = 0
	}
	mainInnerH := bodyH - mainStyle.GetVerticalFrameSize()
	if mainInnerH < 0 {
		mainInnerH = 0
	}
	vm := m
	if vm.mode == ModeCommitting {
		// fill the pane: header above the input is 4 lines (heading, scope
		// hint, key hint, blank), the rest is the editor.
		vm.input.SetWidth(mainInnerW)
		inputH := mainInnerH - commitHeaderHeight
		if inputH < 1 {
			inputH = 1
		}
		vm.input.SetHeight(inputH)
	} else {
		vm.viewport.Width = mainInnerW
		vm.viewport.Height = mainInnerH - mainHeaderHeight
		if vm.viewport.Height < 0 {
			vm.viewport.Height = 0
		}
	}
	main := mainStyle.Width(mainInnerW).Height(mainInnerH).MaxHeight(bodyH).Render(vm.mainContent())

	cols := []string{list, main}
	if railVisible {
		cols = append(cols, m.renderStatusRail(railOuter, bodyH))
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, cols...)

	return m.topBar() + "\n" + body + "\n" + m.footer(railVisible)
}

func (m Model) helpOverlay() string {
	help := strings.Join([]string{
		"loom — keys",
		"",
		"1/2/3 or Tab   focus Files / Branches / Commits",
		"j/k or ↑/↓      move cursor (scroll the diff when in it)",
		"g / G           diff: jump to top / bottom",
		"l / h           enter diff pane / back to list",
		"space           stage / unstage file",
		"d               discard (confirm y)",
		"enter           switch branch",
		"c               commit (Ctrl-D send, Esc cancel)",
		"f / p / P       fetch / pull / push",
		"esc             cancel a running fetch / pull / push",
		"x               command log",
		"? / q           close help / quit",
	}, "\n")
	return borderFocused.Width(m.w - 2).Height(m.h - 2).Render(help)
}

func (m Model) commandLogOverlay() string {
	lines := []string{"loom — command log", ""}
	if len(m.cmdLog) == 0 {
		lines = append(lines, mutedStyle.Render("no commands run yet"))
	} else {
		for i := len(m.cmdLog) - 1; i >= 0; i-- {
			e := m.cmdLog[i]
			lines = append(lines, formatCmdEntry(e))
			if e.output != "" {
				for _, ol := range strings.Split(strings.TrimRight(e.output, "\n"), "\n") {
					lines = append(lines, mutedStyle.Render("    "+ol))
				}
			}
		}
	}
	return borderFocused.Width(m.w - 2).Height(m.h - 2).Render(strings.Join(lines, "\n"))
}

func (m Model) topBar() string {
	stateText, stateStyle := m.commandState()
	parts := []string{
		m.branchSummary(),
		m.workflowTabs(),
		stateStyle.Render(stateText),
	}
	return strings.Join(parts, mutedStyle.Render(" | "))
}

func (m Model) branchSummary() string {
	branch := m.branch.Name
	if branch == "" {
		branch = "(no branch)"
	}
	return strongStyle.Render(fmt.Sprintf("%s ↑%d ↓%d", branch, m.branch.Ahead, m.branch.Behind))
}

func (m Model) workflowTabs() string {
	tabs := []string{
		m.workflowTab(PanelFiles, "1", "Files", len(m.files)),
		m.workflowTab(PanelBranches, "2", "Branches", len(m.branches)),
		m.workflowTab(PanelCommits, "3", "Commits", len(m.commits)),
	}
	return strings.Join(tabs, " ")
}

func (m Model) workflowTab(p Panel, key, name string, count int) string {
	label := fmt.Sprintf("%s %s %d", key, name, count)
	if m.focus == p {
		return titleStyle.Render("[" + label + "]")
	}
	return mutedStyle.Render(label)
}

// commandState returns the command-state label and the style that colors it.
// commandStateText keeps returning plain text so its exact-match test holds.
func (m Model) commandState() (string, lipgloss.Style) {
	switch {
	case m.err != nil:
		return "Error", delStyle
	case m.busy:
		return "Working...", warnStyle
	default:
		return "Ready", addStyle
	}
}

func (m Model) commandStateText() string {
	text, _ := m.commandState()
	return text
}

func formatCmdEntry(e cmdEntry) string {
	return e.at.Format("15:04") + " " + e.text
}

func (m Model) recentCommandLines(max int) []string {
	if max <= 0 || len(m.cmdLog) == 0 {
		return nil
	}
	lines := make([]string, 0, max)
	for i := len(m.cmdLog) - 1; i >= 0 && len(lines) < max; i-- {
		lines = append(lines, formatCmdEntry(m.cmdLog[i]))
	}
	return lines
}

func (m Model) selectedContextLines() []string {
	switch m.focus {
	case PanelFiles:
		i := m.cursor[PanelFiles]
		if i >= len(m.files) {
			return []string{"No file selected", "working tree clean"}
		}
		f := m.files[i]
		state := "unstaged file"
		actions := "actions: stage, discard"
		if f.IsStaged() {
			state = "staged file"
			actions = "actions: unstage, commit"
		}
		if f.Untracked {
			state = "untracked file"
			actions = "actions: stage, discard"
		}
		if f.Unmerged {
			state = "unmerged file"
			actions = "actions: inspect, resolve outside loom"
		}
		return []string{f.Path, state, actions}
	case PanelBranches:
		i := m.cursor[PanelBranches]
		if i >= len(m.branches) {
			return []string{"No branch selected"}
		}
		b := m.branches[i]
		state := "branch"
		actions := "actions: switch, commit, fetch, pull, push"
		if b.Current {
			state = "current branch"
			actions = "actions: commit, fetch, pull, push"
		}
		return []string{b.Name, state, actions}
	case PanelCommits:
		i := m.cursor[PanelCommits]
		if i >= len(m.commits) {
			return []string{"No commit selected"}
		}
		c := m.commits[i]
		hash := c.Hash
		if len(hash) > 7 {
			hash = hash[:7]
		}
		lines := []string{hash, c.Subject}
		if meta := commitMeta(c.Author, c.RelTime); meta != "" {
			lines = append(lines, meta)
		}
		return append(lines, "actions: commit, fetch, pull, push")
	default:
		return nil
	}
}

// commitMeta joins a commit's author and relative time, omitting either when empty.
func commitMeta(author, relTime string) string {
	switch {
	case author != "" && relTime != "":
		return author + " · " + relTime
	case author != "":
		return author
	default:
		return relTime
	}
}

func (m Model) statusRailContent() string {
	stateText, stateStyle := m.commandState()
	sections := []string{
		accentStyle.Render("Status Rail"),
		"",
		accentStyle.Render("Workflow"),
		fmt.Sprintf("Files: %d changed", len(m.files)),
		fmt.Sprintf("Branches: %d local", len(m.branches)),
		fmt.Sprintf("Commits: %d loaded", len(m.commits)),
		"",
		accentStyle.Render("Command"),
		stateStyle.Render(stateText),
	}

	if m.err != nil {
		sections = append(sections, m.err.Error())
	} else if len(m.cmdLog) > 0 {
		sections = append(sections, "Last: "+formatCmdEntry(m.cmdLog[len(m.cmdLog)-1]))
	}

	recent := m.recentCommandLines(recentCommandMax)
	if len(recent) > 0 {
		sections = append(sections, "", accentStyle.Render("Recent"))
		sections = append(sections, recent...)
	}

	selected := m.selectedContextLines()
	if len(selected) > 0 {
		sections = append(sections, "", accentStyle.Render("Selected"))
		sections = append(sections, selected...)
	}

	return strings.Join(sections, "\n")
}

func (m Model) renderStatusRail(w, h int) string {
	style := borderBlur.Padding(0, 1)
	contentW := w - style.GetHorizontalFrameSize()
	if contentW < 0 {
		contentW = 0
	}
	contentH := h - style.GetVerticalFrameSize()
	if contentH < 0 {
		contentH = 0
	}
	return style.Width(contentW).Height(contentH).MaxHeight(h).Render(m.statusRailContent())
}

func (m Model) mainContent() string {
	if m.mode == ModeCommitting {
		return strings.Join([]string{
			"Commit message",
			mutedStyle.Render(m.commitScopeHint()),
			mutedStyle.Render("Ctrl-D to commit, Esc to cancel"),
			"",
			m.input.View(),
		}, "\n")
	}

	title := m.mainTitle()
	if status := m.scrollStatus(); status != "" {
		pad := m.viewport.Width - lipgloss.Width(title) - lipgloss.Width(status)
		if pad < 1 {
			pad = 1
		}
		title += strings.Repeat(" ", pad) + mutedStyle.Render(status)
	}
	body := m.viewport.View()
	if strings.TrimSpace(body) == "" {
		body = m.emptyMainBody()
	}
	return title + "\n" + mutedStyle.Render(strings.Repeat("─", lipgloss.Width(title))) + "\n\n" + colorizeDiff(body)
}

// scrollStatus is a compact main-pane position cue: arrows for hidden content
// above/below plus a scroll percentage. Empty when everything already fits, so
// it adds no clutter to short diffs.
func (m Model) scrollStatus() string {
	if m.viewport.TotalLineCount() <= m.viewport.Height {
		return ""
	}
	up, down := " ", " "
	if !m.viewport.AtTop() {
		up = "↑"
	}
	if !m.viewport.AtBottom() {
		down = "↓"
	}
	return fmt.Sprintf("%s%s %.0f%%", up, down, m.viewport.ScrollPercent()*100)
}

// commitScopeHint tells the user what `c` will commit: the staged files when any
// are staged, otherwise every change (loom stages all first).
func (m Model) commitScopeHint() string {
	switch n := m.stagedCount(); n {
	case 0:
		return "Nothing staged — committing all changes"
	case 1:
		return "Committing 1 staged file"
	default:
		return fmt.Sprintf("Committing %d staged files", n)
	}
}

func (m Model) mainTitle() string {
	switch m.focus {
	case PanelFiles:
		i := m.cursor[PanelFiles]
		if i >= len(m.files) {
			return "Working tree clean"
		}
		f := m.files[i]
		state := "unstaged"
		if f.IsStaged() {
			state = "staged"
		}
		return fmt.Sprintf("Diff: %s (%s)", f.Path, state)
	case PanelBranches:
		i := m.cursor[PanelBranches]
		if i >= len(m.branches) {
			return "No local branches"
		}
		return "Branch log: " + m.branches[i].Name
	case PanelCommits:
		i := m.cursor[PanelCommits]
		if i >= len(m.commits) {
			return "No commits"
		}
		hash := m.commits[i].Hash
		if len(hash) > 7 {
			hash = hash[:7]
		}
		return "Commit: " + hash
	default:
		return ""
	}
}

func (m Model) emptyMainBody() string {
	switch m.focus {
	case PanelFiles:
		return "No file diff to show."
	case PanelBranches:
		return "No branch log to show."
	case PanelCommits:
		return "No commit detail to show."
	default:
		return ""
	}
}

type keyHint struct {
	key  string
	desc string
}

func (m Model) footerHints() (label string, hints []keyHint) {
	switch m.mode {
	case ModeConfirming:
		return "Confirm", []keyHint{{"y", "yes"}, {"n", "no"}, {"esc", "cancel"}}
	case ModeCommitting:
		return "Commit", []keyHint{{"ctrl+d", "submit"}, {"esc", "cancel"}}
	}
	if m.mainFocused {
		return "Diff", []keyHint{{"j/k", "scroll"}, {"g/G", "top/bot"}, {"h", "back"}, {"?", "help"}, {"q", "quit"}}
	}
	switch m.focus {
	case PanelFiles:
		return "Files", []keyHint{{"space", "stage"}, {"d", "discard"}, {"c", "commit"}, {"?", "help"}, {"q", "quit"}}
	case PanelBranches:
		return "Branches", []keyHint{{"enter", "switch"}, {"c", "commit"}, {"f", "fetch"}, {"p", "pull"}, {"P", "push"}, {"?", "help"}, {"q", "quit"}}
	case PanelCommits:
		return "Commits", []keyHint{{"c", "commit"}, {"f", "fetch"}, {"p", "pull"}, {"P", "push"}, {"?", "help"}, {"q", "quit"}}
	default:
		return "", []keyHint{{"?", "help"}, {"q", "quit"}}
	}
}

func (m Model) footerActions() string {
	label, hints := m.footerHints()
	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = h.key + " " + h.desc
	}
	body := strings.Join(parts, " · ")
	if label == "" {
		return body
	}
	return label + ": " + body
}

func (m Model) styledFooterActions() string {
	label, hints := m.footerHints()
	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = accentStyle.Render(h.key) + " " + mutedStyle.Render(h.desc)
	}
	body := strings.Join(parts, mutedStyle.Render(" · "))
	if label == "" {
		return body
	}
	return mutedStyle.Render(label+": ") + body
}

func (m Model) footer(railVisible bool) string {
	actions := m.styledFooterActions()
	switch {
	case m.busy:
		hint := ""
		if m.cancelOp != nil {
			hint = accentStyle.Render(" esc") + mutedStyle.Render(" cancel")
		}
		return m.spinner.View() + " working…" + hint + "   " + actions
	case !railVisible && m.err != nil:
		return delStyle.Render("error: "+m.err.Error()) + "   " + actions
	default:
		return actions
	}
}
