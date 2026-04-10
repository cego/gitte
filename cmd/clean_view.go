package cmd

import (
	"fmt"
	"strings"
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

// cleanMsg carries a state change for one repo within a phase.
// state must be cleanStateRunning, cleanStateOK, or cleanStateFailed;
// cleanStatePending is the initial state set by newCleanModel, never sent.
type cleanMsg struct {
	phase  string // matches cleanPhaseSpec.Title
	repo   string
	state  cleanState
	detail string
}

// cleanAdvanceMsg is sent via program.Send() (not through msgCh) to increment
// activePhase. Because it bypasses the channel, it never interferes with the
// listen() goroutine that is already waiting for the next cleanMsg.
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
	var b strings.Builder
	width := m.width
	if width <= 0 {
		width = 80
	}

	for i, phase := range m.phases {
		isUpcoming := i > m.activePhase
		header := renderPhaseHeader(phase.title, width)
		if isUpcoming {
			b.WriteString(cleanDimStyle.Render(header) + "\n")
			continue
		}
		b.WriteString(cleanSectionStyle.Render(header) + "\n")

		colCount := colsForWidth(width)
		colW := width / colCount

		lines := make([]string, len(phase.entries))
		for j, e := range phase.entries {
			lines[j] = m.renderEntry(e, colW)
		}

		nRows := (len(lines) + colCount - 1) / colCount
		for row := 0; row < nRows; row++ {
			for col := 0; col < colCount; col++ {
				idx := col*nRows + row
				if idx >= len(lines) {
					break
				}
				b.WriteString(lines[idx])
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

func renderPhaseHeader(title string, width int) string {
	prefix := "── " + title + " "
	fill := width - len([]rune(prefix))
	if fill < 0 {
		fill = 0
	}
	return prefix + strings.Repeat("─", fill)
}

func (m *cleanModel) renderEntry(e *cleanEntry, colW int) string {
	var icon, nameStr, extra string
	switch e.state {
	case cleanStatePending:
		icon = cleanPendingStyle.Render("○")
		nameStr = cleanDimStyle.Render(e.name)
	case cleanStateRunning:
		frame := cleanSpinnerFrames[m.spinnerTick%len(cleanSpinnerFrames)]
		icon = cleanRunningStyle.Render(frame)
		nameStr = cleanLabelStyle.Render(e.name)
		if e.detail != "" {
			extra = cleanDimStyle.Render("  " + e.detail)
		}
	case cleanStateOK:
		icon = cleanOKStyle.Render("✓")
		nameStr = cleanOKStyle.Render(e.name)
		if e.detail != "" {
			extra = cleanDimStyle.Render("  " + e.detail)
		}
	case cleanStateFailed:
		icon = cleanFailStyle.Render("✗")
		nameStr = cleanFailStyle.Render(e.name)
		if e.detail != "" {
			extra = cleanFailStyle.Render("  " + e.detail)
		}
	}
	line := fmt.Sprintf(" %s %s%s", icon, nameStr, extra)
	return cleanFitToWidth(line, colW)
}

// cleanFitToWidth pads or truncates s to exactly width visible characters.
func cleanFitToWidth(s string, width int) string {
	vis := cleanVisibleWidth(s)
	if vis < width {
		return s + strings.Repeat(" ", width-vis)
	}
	return cleanTruncateToVisualWidth(s, width)
}

// cleanVisibleWidth returns the display width of s, ignoring ANSI escape codes.
func cleanVisibleWidth(s string) int {
	width := 0
	inEscape := false
	for _, r := range s {
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			continue
		}
		width++
	}
	return width
}

// cleanTruncateToVisualWidth truncates s to at most maxWidth visible characters,
// preserving ANSI escape codes that appear before the truncation point.
func cleanTruncateToVisualWidth(s string, maxWidth int) string {
	var b strings.Builder
	width := 0
	inEscape := false
	for _, r := range s {
		if inEscape {
			b.WriteRune(r)
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			b.WriteRune(r)
			continue
		}
		if width >= maxWidth {
			break
		}
		b.WriteRune(r)
		width++
	}
	return b.String()
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
