package ui

import (
	"context"
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
	PanelStashes
	panelCount
)

// Mode is the interaction mode of the app.
type Mode int

const (
	ModeNormal Mode = iota
	ModeCommitting
	ModeConfirming
	ModeStashing
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
	ctx          context.Context
	repo         *git.Repo
	files        []git.FileStatus
	branches     []git.Branch
	commits      []git.Commit
	stashes      []git.Stash
	branch       git.BranchInfo
	focus        Panel
	mainFocused  bool // when true, j/k scroll the main pane instead of moving the list cursor
	cursor       map[Panel]int
	scroll       map[Panel]int
	viewport     viewport.Model
	mainLoading  bool // true while the main pane is waiting for the latest selection load
	subject      textinput.Model
	body         textarea.Model
	stashMessage textinput.Model
	commitField  commitField
	notice       string // transient success line; cleared on the next key
	amending     bool   // the current ModeCommitting session is a git commit --amend
	spinner      spinner.Model
	mode         Mode
	confirm      confirmReq
	busy         bool
	merging      bool // a merge is in progress (MERGE_HEAD exists)
	cmdLog       []cmdEntry
	showLog      bool
	showHelp     bool
	err          error
	w, h         int
	listHeight   int
	reqSeq       int                // monotonic token stamped on each main-pane load
	cancelOp     context.CancelFunc // cancels the in-flight remote op; nil when none
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
	stashMsg := textinput.New()
	stashMsg.Placeholder = "Stash message..."
	return Model{
		ctx:          ctx,
		repo:         repo,
		focus:        PanelFiles,
		cursor:       map[Panel]int{},
		scroll:       map[Panel]int{},
		viewport:     viewport.New(0, 0),
		subject:      subj,
		body:         body,
		stashMessage: stashMsg,
		spinner:      sp,
		mode:         ModeNormal,
	}
}

// Init kicks off the first data loads.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		loadStatus(m.ctx, m.repo),
		loadBranches(m.ctx, m.repo),
		loadCommits(m.ctx, m.repo),
		loadStashes(m.ctx, m.repo),
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
