package ui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
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

// cmdEntry is one git command we ran, with when it completed.
type cmdEntry struct {
	at   time.Time
	text string
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
	input       textarea.Model
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
	ta := textarea.New()
	ta.Placeholder = "Commit message…"
	return Model{
		ctx:      ctx,
		repo:     repo,
		focus:    PanelFiles,
		cursor:   map[Panel]int{},
		scroll:   map[Panel]int{},
		viewport: viewport.New(0, 0),
		input:    ta,
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

// layout sizes the panes for the current window.
func (m *Model) layout() {
	sideW := m.w / 3
	mainW := m.w - sideW - 2
	if mainW < 0 {
		mainW = 0
	}
	bodyH := m.h - topBarHeight - 1 // top bar row + footer row
	if bodyH < 0 {
		bodyH = 0
	}
	listH := bodyH - borderBlur.GetVerticalFrameSize() - 2 // border + title + blank
	if listH < 0 {
		listH = 0
	}
	m.listHeight = listH
	mainH := bodyH - borderBlur.GetVerticalFrameSize() - mainHeaderHeight
	if mainH < 0 {
		mainH = 0
	}
	m.viewport.Width = mainW
	m.viewport.Height = mainH
}
