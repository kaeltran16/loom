package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kael02/loom/internal/git"
)

const mainHeaderHeight = 3

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

	list := m.renderPanel(panelTitle(panelName(m.focus), m.focusLen()), m.focus, m.panelRows(m.focus), listOuter, bodyH)

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
		// fill the pane: the body textarea takes whatever the editor header
		// (commitHeaderHeight) leaves of the pane's content height.
		vm.viewport.Width = mainInnerW
		vm.subject.Width = mainInnerW - 2
		editorBodyH := mainInnerH - vm.commitHeaderHeight()
		if editorBodyH < 1 {
			editorBodyH = 1
		}
		vm.body.SetWidth(mainInnerW)
		vm.body.SetHeight(editorBodyH)
	} else if vm.mode == ModeStashing {
		vm.viewport.Width = mainInnerW
		vm.stashMessage.Width = mainInnerW - 2
		if vm.stashMessage.Width < 1 {
			vm.stashMessage.Width = 1
		}
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
		"1/2/3/4 or Tab focus Files / Branches / Commits / Stashes",
		"j/k or ↑/↓      move cursor (scroll the diff when in it)",
		"g / G           diff: jump to top / bottom",
		"l / h           enter diff pane / back to list",
		"space           stage / unstage file",
		"d               discard (confirm y)",
		"s               save stash from Stashes",
		"a / o / d       apply / pop / drop selected stash",
		"enter           switch branch",
		"c               commit (Ctrl-D send, Esc cancel)",
		"C               amend last commit (edit message, Ctrl-D send, Esc cancel)",
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
	if banner := m.mergeBanner(); banner != "" {
		parts = append(parts, warnStyle.Render(banner))
	}
	return strings.Join(parts, mutedStyle.Render(" | "))
}

// mergeBanner is the top-bar merge cue: the remaining-conflict count while any
// remain, then "ready to commit" once all are resolved but the merge commit has
// not landed. Empty when not merging.
func (m Model) mergeBanner() string {
	if !m.merging {
		return ""
	}
	switch n := m.unmergedCount(); n {
	case 0:
		return "MERGING — ready to commit"
	case 1:
		return "MERGING — 1 conflict"
	default:
		return fmt.Sprintf("MERGING — %d conflicts", n)
	}
}

func (m Model) unmergedCount() int {
	n := 0
	for _, f := range m.files {
		if f.Unmerged {
			n++
		}
	}
	return n
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
		m.workflowTab(PanelStashes, "4", "Stashes", len(m.stashes)),
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
	case m.notice != "":
		return m.notice, addStyle
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
		f, ok := m.selectedFile()
		if !ok {
			return []string{"No file selected", "working tree clean"}
		}
		return []string{f.Path, fileReviewState(f), m.selectedFileActionLine(f)}
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
	case PanelStashes:
		s, ok := m.selectedStash()
		if !ok {
			return []string{"No stash selected"}
		}
		lines := []string{s.Ref, s.Message}
		if meta := stashMeta(s); meta != "" {
			lines = append(lines, meta)
		}
		return append(lines, "actions: s save, a apply, o pop, d drop")
	default:
		return nil
	}
}

func (m Model) selectedFile() (git.FileStatus, bool) {
	i := m.cursor[PanelFiles]
	if i < 0 || i >= len(m.files) {
		return git.FileStatus{}, false
	}
	return m.files[i], true
}

func (m Model) selectedStash() (git.Stash, bool) {
	i := m.cursor[PanelStashes]
	if i < 0 || i >= len(m.stashes) {
		return git.Stash{}, false
	}
	return m.stashes[i], true
}

func fileReviewState(f git.FileStatus) string {
	switch {
	case f.Unmerged:
		return "conflict: " + conflictLabel(f.Conflict)
	case f.Untracked:
		return "untracked file"
	case f.IsStaged():
		return "staged file"
	default:
		return "unstaged file"
	}
}

func (m Model) commitHint() string {
	if m.anyStaged() {
		return "commit staged"
	}
	return "commit all"
}

func (m Model) selectedFileActionLine(f git.FileStatus) string {
	switch {
	case f.Unmerged:
		return "actions: e edit, space resolve, A abort, c commit"
	case f.Untracked:
		return "actions: space stage, d discard, s stash"
	case f.IsStaged():
		return "actions: space unstage, s stash, c commit staged"
	default:
		return "actions: space stage, d discard, s stash, c " + m.commitHint()
	}
}

func (m Model) selectedFileFooterHints(f git.FileStatus) []keyHint {
	switch {
	case f.Unmerged:
		return []keyHint{{"e", "edit"}, {"space", "resolve"}, {"A", "abort"}, {"c", "commit"}, {"?", "help"}, {"q", "quit"}}
	case f.Untracked:
		return []keyHint{{"space", "stage"}, {"d", "discard"}, {"s", "stash"}, {"?", "help"}, {"q", "quit"}}
	case f.IsStaged():
		return []keyHint{{"space", "unstage"}, {"s", "stash"}, {"c", "commit staged"}, {"?", "help"}, {"q", "quit"}}
	default:
		return []keyHint{{"space", "stage"}, {"d", "discard"}, {"s", "stash"}, {"c", m.commitHint()}, {"?", "help"}, {"q", "quit"}}
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

func stashMeta(s git.Stash) string {
	switch {
	case s.Branch != "" && s.Age != "":
		return s.Branch + " · " + s.Age
	case s.Branch != "":
		return s.Branch
	default:
		return s.Age
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
		fmt.Sprintf("Stashes: %d saved", len(m.stashes)),
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
		return m.commitEditorView()
	}
	if m.mode == ModeStashing {
		return m.stashEditorView()
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
	if m.mainLoading && m.focus == PanelFiles {
		body = "Loading diff..."
	} else if m.mainLoading && m.focus == PanelStashes {
		body = "Loading stash preview..."
	} else if strings.TrimSpace(body) == "" {
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

// commitHeaderHeight is the number of non-body lines the commit editor draws,
// so the body textarea can claim the rest of the pane.
func (m Model) commitHeaderHeight() int {
	h := 9
	if m.amending && m.amendPushed() {
		h++ // the force-push warning row
	}
	return h
}

func (m Model) commitEditorView() string {
	heading := "Commit message"
	scope := m.commitScopeHint()
	verb := "commit"
	if m.amending {
		heading = "Amend commit"
		scope = m.amendScopeHint()
		verb = "amend"
	}

	n := len([]rune(strings.TrimSpace(m.subject.Value())))
	counter := counterStyle(subjectLevel(n)).Render(fmt.Sprintf("%d/%d", n, subjectIdeal))

	subjLabel := mutedStyle.Render("Subject")
	bodyLabel := mutedStyle.Render("Body")
	if m.commitField == fieldSubject {
		subjLabel = accentStyle.Render("Subject")
	} else {
		bodyLabel = accentStyle.Render("Body")
	}
	hint := mutedStyle.Render("type(scope): subject · imperative, ≤50")
	left := subjLabel + "  " + hint
	pad := m.viewport.Width - lipgloss.Width(left) - lipgloss.Width(counter)
	if pad < 1 {
		pad = 1
	}
	subjectRow := left + strings.Repeat(" ", pad) + counter

	sepW := m.viewport.Width
	if sepW < 1 {
		sepW = 1
	}
	sep := mutedStyle.Render(strings.Repeat("─", sepW))

	lines := []string{strongStyle.Render(heading), mutedStyle.Render(scope)}
	if m.amending && m.amendPushed() {
		lines = append(lines, delStyle.Render("⚠ already pushed — amend needs a force-push"))
	}
	lines = append(lines,
		"",
		subjectRow,
		m.subject.View(),
		sep,
		bodyLabel,
		m.body.View(),
		mutedStyle.Render("Ctrl-D "+verb+" · Tab switch · Esc cancel"),
	)
	return strings.Join(lines, "\n")
}

func (m Model) stashEditorView() string {
	label := accentStyle.Render("Message")
	width := m.viewport.Width
	if width < 1 {
		width = m.w
	}
	vm := m
	vm.stashMessage.Width = width - 2
	if vm.stashMessage.Width < 1 {
		vm.stashMessage.Width = 1
	}
	return strings.Join([]string{
		strongStyle.Render("Save stash"),
		mutedStyle.Render("Save current tracked and untracked work"),
		"",
		label,
		vm.stashMessage.View(),
		"",
		mutedStyle.Render("Ctrl-D save · Esc cancel"),
	}, "\n")
}

// amendScopeHint tells the user what `C` will fold into HEAD: staged changes
// plus the edited message, or just the message when nothing is staged.
func (m Model) amendScopeHint() string {
	switch n := m.stagedCount(); n {
	case 0:
		return "Amending HEAD (message only)"
	case 1:
		return "Amending HEAD + 1 staged file"
	default:
		return fmt.Sprintf("Amending HEAD + %d staged files", n)
	}
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
		f, ok := m.selectedFile()
		if !ok {
			return "Working tree clean"
		}
		pos, total := m.selectedFilePosition()
		return fmt.Sprintf("%s | %s | %d of %d", f.Path, fileTitleState(f), pos, total)
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
	case PanelStashes:
		s, ok := m.selectedStash()
		if !ok {
			return "No stashes"
		}
		pos, total := m.selectedStashPosition()
		return fmt.Sprintf("%s | %s | %d of %d", s.Ref, s.Message, pos, total)
	default:
		return ""
	}
}

func fileTitleState(f git.FileStatus) string {
	switch {
	case f.Unmerged:
		return "conflict"
	case f.Untracked:
		return "untracked"
	case f.IsStaged():
		return "staged"
	default:
		return "unstaged"
	}
}

func (m Model) selectedFilePosition() (int, int) {
	if len(m.files) == 0 {
		return 0, 0
	}
	i := m.cursor[PanelFiles]
	if i < 0 {
		i = 0
	}
	if i >= len(m.files) {
		i = len(m.files) - 1
	}
	return i + 1, len(m.files)
}

func (m Model) selectedStashPosition() (int, int) {
	if len(m.stashes) == 0 {
		return 0, 0
	}
	i := m.cursor[PanelStashes]
	if i < 0 {
		i = 0
	}
	if i >= len(m.stashes) {
		i = len(m.stashes) - 1
	}
	return i + 1, len(m.stashes)
}

func (m Model) emptyMainBody() string {
	switch m.focus {
	case PanelFiles:
		return "No diff for this file"
	case PanelBranches:
		return "No branch log to show."
	case PanelCommits:
		return "No commit detail to show."
	case PanelStashes:
		return "No stash preview to show."
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
		label := "Commit"
		if m.amending {
			label = "Amend"
		}
		return label, []keyHint{{"ctrl+d", "submit"}, {"tab", "switch"}, {"esc", "cancel"}}
	case ModeStashing:
		return "Stash", []keyHint{{"ctrl+d", "save"}, {"esc", "cancel"}}
	}
	if m.mainFocused {
		return "Diff", []keyHint{{"j/k", "scroll"}, {"g/G", "top/bot"}, {"h", "back"}, {"?", "help"}, {"q", "quit"}}
	}
	switch m.focus {
	case PanelFiles:
		if f, ok := m.selectedFile(); ok {
			return "Files", m.selectedFileFooterHints(f)
		}
		return "Files", []keyHint{{"?", "help"}, {"q", "quit"}}
	case PanelBranches:
		return "Branches", []keyHint{{"enter", "switch"}, {"c", "commit"}, {"f", "fetch"}, {"p", "pull"}, {"P", "push"}, {"?", "help"}, {"q", "quit"}}
	case PanelCommits:
		return "Commits", []keyHint{{"c", "commit"}, {"f", "fetch"}, {"p", "pull"}, {"P", "push"}, {"?", "help"}, {"q", "quit"}}
	case PanelStashes:
		return "Stashes", []keyHint{{"s", "save"}, {"a", "apply"}, {"o", "pop"}, {"d", "drop"}, {"?", "help"}, {"q", "quit"}}
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
