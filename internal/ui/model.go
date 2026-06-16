package ui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kael02/loom/internal/git"
)

// Panel identifies the focused context panel.
type Panel int

const (
	PanelFiles Panel = iota
	PanelBranches
	PanelCommits
	panelCount
)

// Mode is the interaction mode of the app.
type Mode int

const (
	ModeNormal Mode = iota
	ModeCommitting
	ModeConfirming
)

// commitField is which editor field has focus in ModeCommitting.
type commitField int

const (
	fieldSubject commitField = iota
	fieldBody
)

// cmdEntry is one git command we ran, with when it completed and any output
// it produced (remote ops carry git's combined stdout+stderr; fast mutations
// carry none).
type cmdEntry struct {
	at     time.Time
	text   string
	output string
}

// Model is the entire application state.
type Model struct {
	ctx         context.Context
	repo        *git.Repo
	files       []git.FileStatus
	branches    []git.Branch
	commits     []git.Commit
	branch      git.BranchInfo
	focus       Panel
	mainFocused bool // when true, j/k scroll the main pane instead of moving the list cursor
	cursor      map[Panel]int
	scroll      map[Panel]int
	viewport    viewport.Model
	mainDiff    *git.Diff // non-nil when the main pane shows a parsed diff
	mainText    string    // plain text for the branch-log path (mainDiff nil)
	hunkRows    []int     // viewport row index of each hunk band, for n/N
	subject     textinput.Model
	body        textarea.Model
	commitField commitField
	notice      string // transient success line; cleared on the next key
	amending    bool   // the current ModeCommitting session is a git commit --amend
	spinner     spinner.Model
	mode        Mode
	confirm     confirmReq
	busy        bool
	cmdLog      []cmdEntry
	showLog     bool
	showHelp    bool
	err         error
	w, h        int
	listHeight  int
	reqSeq      int                // monotonic token stamped on each main-pane load
	cancelOp    context.CancelFunc // cancels the in-flight remote op; nil when none
}

// confirmReq describes a pending destructive action awaiting [y/n].
type confirmReq struct {
	prompt string
	action tea.Cmd
}

// NewModel builds the initial model bound to repo.
func NewModel(ctx context.Context, repo *git.Repo) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	subj := textinput.New()
	subj.Placeholder = "Summary…"
	body := textarea.New()
	body.Placeholder = "Body (optional)…"
	return Model{
		ctx:      ctx,
		repo:     repo,
		focus:    PanelFiles,
		cursor:   map[Panel]int{},
		scroll:   map[Panel]int{},
		viewport: viewport.New(0, 0),
		subject:  subj,
		body:     body,
		spinner:  sp,
		mode:     ModeNormal,
	}
}

// Init kicks off the first data loads.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		loadStatus(m.ctx, m.repo),
		loadBranches(m.ctx, m.repo),
		loadCommits(m.ctx, m.repo),
		m.spinner.Tick,
	)
}

// mainViewportSize returns the inner width/height of the diff/main viewport for
// the current window. Single source of truth shared by layout() and View() so
// the render width always equals the displayed width. It mirrors View()'s pane
// layout (rail when wide, list = w/4, main takes the rest, less the border).
func (m Model) mainViewportSize() (w, h int) {
	railOuter := 0
	if m.w >= minRailWindowWide {
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
	w = mainOuter - borderBlur.GetHorizontalFrameSize()
	if w < 0 {
		w = 0
	}
	bodyH := m.h - topBarHeight - 1
	if bodyH < 0 {
		bodyH = 0
	}
	h = bodyH - borderBlur.GetVerticalFrameSize() - mainHeaderHeight
	if h < 0 {
		h = 0
	}
	return w, h
}

// layout sizes the panes for the current window.
func (m *Model) layout() {
	bodyH := m.h - topBarHeight - 1 // top bar row + footer row
	if bodyH < 0 {
		bodyH = 0
	}
	listH := bodyH - borderBlur.GetVerticalFrameSize() - 2 // border + title + blank
	if listH < 0 {
		listH = 0
	}
	m.listHeight = listH
	m.viewport.Width, m.viewport.Height = m.mainViewportSize()
}

// refreshViewport re-renders the main pane content at the current viewport
// width: a parsed diff via renderDiff, otherwise the plain branch-log text.
func (m *Model) refreshViewport() {
	if m.mainDiff != nil {
		lines, hunkRows := renderDiff(*m.mainDiff, m.viewport.Width)
		m.viewport.SetContent(strings.Join(lines, "\n"))
		m.hunkRows = hunkRows
		return
	}
	m.viewport.SetContent(m.mainText)
	m.hunkRows = nil
}
