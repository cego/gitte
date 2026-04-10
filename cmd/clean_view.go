package cmd

import (
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---- states ----------------------------------------------------------------

type cleanState int

const (
	cleanStatePending cleanState = iota
	cleanStateRunning
	cleanStateOK
	cleanStateFailed
)

// ---- data types ------------------------------------------------------------

type cleanEntry struct {
	name   string
	state  cleanState
	detail string
}

type cleanPhaseModel struct {
	title   string
	entries []*cleanEntry
	index   map[string]int
}

// cleanPhaseSpec is used by callers to declare a phase with its repos upfront.
type cleanPhaseSpec struct {
	Title string
	Repos []string
}

// ---- messages --------------------------------------------------------------

type cleanMsg struct {
	phase  string // matches cleanPhaseSpec.Title
	repo   string
	state  cleanState
	detail string
}

type cleanAdvanceMsg struct{} // advance to next phase
type cleanDoneMsg struct{}    // all messages consumed
type cleanTickMsg time.Time

// ---- styles ----------------------------------------------------------------

var (
	cleanPendingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cleanRunningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	cleanOKStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	cleanFailStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	cleanLabelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	cleanSectionStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	cleanDimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	cleanSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
)

// ---- BubbleTea model -------------------------------------------------------

type cleanModel struct {
	phases      []cleanPhaseModel
	phaseIndex  map[string]int // phase title → index in phases
	activePhase int
	msgCh       <-chan cleanMsg
	spinnerTick int
	drainedCh   chan struct{}
	drainOnce   sync.Once
	width       int
}

func newCleanModel(specs []cleanPhaseSpec, msgCh <-chan cleanMsg, drainedCh chan struct{}) *cleanModel {
	phases := make([]cleanPhaseModel, len(specs))
	idx := make(map[string]int, len(specs))
	for i, spec := range specs {
		entries := make([]*cleanEntry, len(spec.Repos))
		entryIdx := make(map[string]int, len(spec.Repos))
		for j, r := range spec.Repos {
			entries[j] = &cleanEntry{name: r, state: cleanStatePending}
			entryIdx[r] = j
		}
		phases[i] = cleanPhaseModel{title: spec.Title, entries: entries, index: entryIdx}
		idx[spec.Title] = i
	}
	return &cleanModel{
		phases:     phases,
		phaseIndex: idx,
		msgCh:      msgCh,
		drainedCh:  drainedCh,
	}
}

func (m *cleanModel) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg { return cleanTickMsg(t) }),
		m.listen(),
	)
}

func (m *cleanModel) listen() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.msgCh
		if !ok {
			m.drainOnce.Do(func() { close(m.drainedCh) })
			return cleanDoneMsg{}
		}
		return msg
	}
}

func (m *cleanModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width

	case cleanTickMsg:
		m.spinnerTick++
		return m, tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg { return cleanTickMsg(t) })

	case cleanAdvanceMsg:
		if m.activePhase < len(m.phases)-1 {
			m.activePhase++
		}

	case cleanMsg:
		if pi, ok := m.phaseIndex[msg.phase]; ok {
			phase := &m.phases[pi]
			if ei, ok := phase.index[msg.repo]; ok {
				e := phase.entries[ei]
				switch msg.state {
				case cleanStateRunning:
					if e.state == cleanStatePending {
						e.state = cleanStateRunning
					}
					if msg.detail != "" {
						e.detail = msg.detail
					}
				case cleanStateOK, cleanStateFailed:
					e.state = msg.state
					if msg.detail != "" {
						e.detail = msg.detail
					}
				}
			}
		}
		return m, m.listen()

	case cleanDoneMsg:
		return m, tea.Quit
	}
	return m, nil
}

func (m *cleanModel) View() string {
	return ""
}

// colsForWidth returns the number of columns based on terminal width.
// ≥180 cols → 3, ≥120 cols → 2, otherwise → 1.
func colsForWidth(width int) int {
	if width >= 180 {
		return 3
	}
	if width >= 120 {
		return 2
	}
	return 1
}
